package crossplane

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// InstanceSpecParamsPath is the path to an instance's parameters
	InstanceSpecParamsPath = "spec.parameters"

	// instanceParamsParentReferenceName is the name of an instance's parent reference parameter
	instanceParamsParentReferenceName = "parent_reference"
	// instanceSpecParamsParentReferencePath is the path to an instance's parent reference parameter
	instanceSpecParamsParentReferencePath = InstanceSpecParamsPath + "." + instanceParamsParentReferenceName
)

var (
	// ErrInstanceNotFound is an instance doesn't exist
	ErrInstanceNotFound = errors.New("instance not found")
	// ErrServiceUpdateNotPermitted when updating an instance
	ErrServiceUpdateNotPermitted = errors.New("service update not permitted")
	// ErrClusterChangeNotPermitted when updating an instance
	ErrClusterChangeNotPermitted = errors.New("cluster change not permitted")
	// ErrSLAChangeNotPermitted when updating an instance's SLA plan (only premium<->standard is permitted)
	ErrSLAChangeNotPermitted = errors.New("SLA change not permitted")
)

// CreateInstance creates a service instance
func (cp *Crossplane) CreateInstance(ctx context.Context, instanceID string, parameters json.RawMessage, plan *v1beta1.Composition) error {
	labels := map[string]string{
		InstanceIDLabel: instanceID,
	}
	// Copy relevant labels from plan
	for _, l := range []string{
		ServiceIDLabel,
		ServiceNameLabel,
		PlanNameLabel,
		ClusterLabel,
		SLALabel,
	} {
		labels[l] = plan.Labels[l]
	}

	gvk, err := gvkFromPlan(plan)
	if err != nil {
		return err
	}

	cmp := composite.New(composite.WithGroupVersionKind(gvk))
	cmp.SetName(instanceID)
	cmp.SetCompositionReference(&corev1.ObjectReference{
		Name: plan.Name,
	})
	parametersMap := map[string]interface{}{}
	if parameters != nil {
		if err := json.Unmarshal(parameters, &parametersMap); err != nil {
			return err
		}
		if parentReference, err := fieldpath.
			Pave(parametersMap).
			GetString(instanceParamsParentReferenceName); err == nil {
			// Set parent reference in a label so we can search for it later.
			labels[ParentIDLabel] = parentReference
		}
	}
	if err := fieldpath.Pave(cmp.Object).SetValue(InstanceSpecParamsPath, parametersMap); err != nil {
		return err
	}
	cmp.SetLabels(labels)
	cp.logger.Debug("create-instance", lager.Data{"instance": cmp})
	return cp.Client.Create(ctx, cmp)
}

// DeleteInstance deletes a service instance
func (cp *Crossplane) DeleteInstance(ctx context.Context, instanceName string, plan *v1beta1.Composition) error {
	gvk, err := gvkFromPlan(plan)
	if err != nil {
		return err
	}

	cmp := composite.New(composite.WithGroupVersionKind(gvk))
	cmp.SetName(instanceName)

	return cp.Client.Delete(ctx, cmp)
}

// InstanceExists checks if a service instance exists with the given ID for the given plan
func (cp *Crossplane) InstanceExists(ctx context.Context, instanceID string, plan *v1beta1.Composition) (*composite.Unstructured, bool, error) {
	instance, err := cp.GetInstanceWithPlan(ctx, instanceID, plan)
	if err != nil {
		if errors.Is(err, ErrInstanceNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return instance, true, nil
}

// GetInstanceWithPlan returns the instance with a given ID for a given plan
func (cp *Crossplane) GetInstanceWithPlan(ctx context.Context, instanceID string, plan *v1beta1.Composition) (*composite.Unstructured, error) {
	gvk, err := gvkFromPlan(plan)
	if err != nil {
		return nil, err
	}

	cmp := composite.New(composite.WithGroupVersionKind(gvk))
	cmp.SetName(instanceID)

	err = cp.Client.Get(ctx, types.NamespacedName{
		Name: instanceID,
	}, cmp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if cmp.GetLabels()[PlanNameLabel] != plan.Labels[PlanNameLabel] {
		return nil, ErrInstanceNotFound
	}

	return cmp, nil
}

// GetInstance returns the instance with a given ID.
// It will search all available plans for the instance, therefore use `GetInstanceWithPlan` whenever possible.
func (cp *Crossplane) GetInstance(ctx context.Context, instanceID string) (*composite.Unstructured, error) {
	plans, err := cp.getPlansForService(ctx, cp.ServiceIDs)
	if err != nil {
		return nil, fmt.Errorf("could not get plans %w", err)
	}
	for _, plan := range plans {
		instance, err := cp.GetInstanceWithPlan(ctx, instanceID, &plan)
		if err != nil {
			if errors.Is(err, ErrInstanceNotFound) {
				continue
			}
			return nil, err
		}
		return instance, nil

	}
	return nil, ErrInstanceNotFound
}

// UpdateInstanceSLA updates the SLA of an instance specified by the supplied planID.
// Only SLA changes are allowed, any other change is not permitted and yields an error.
func (cp *Crossplane) UpdateInstanceSLA(ctx context.Context, instanceID, serviceID, planID string) error {
	instance, err := cp.GetInstance(ctx, instanceID)
	if err != nil {
		return err
	}

	instanceLabels := instance.GetLabels()
	if serviceID != instanceLabels[ServiceIDLabel] {
		return ErrServiceUpdateNotPermitted
	}

	newPlan, err := cp.GetPlan(ctx, planID)
	if err != nil {
		return err
	}

	slaChangePermitted := func() bool {
		instanceSLA := instanceLabels[SLALabel]
		newPlanSLA := newPlan.Labels[SLALabel]
		instancePlanLevel := getPlanLevel(instanceLabels[PlanNameLabel])
		newPlanLevel := getPlanLevel(newPlan.Labels[PlanNameLabel])
		instanceService := instanceLabels[ServiceIDLabel]
		newPlanService := newPlan.Labels[ServiceIDLabel]

		// switch from redis to mariadb not permitted
		if instanceService != newPlanService {
			return false
		}
		// xsmall -> large not permitted, only xsmall <-> xsmall-premium
		if instancePlanLevel != newPlanLevel {
			return false
		}
		if instanceSLA == SLAPremium && newPlanSLA == SLAStandard {
			return true
		}
		if instanceSLA == SLAStandard && newPlanSLA == SLAPremium {
			return true
		}
		return false
	}

	if !slaChangePermitted() {
		return ErrSLAChangeNotPermitted
	}

	instance.SetCompositionReference(&corev1.ObjectReference{
		Name: newPlan.Name,
	})
	for _, l := range []string{
		PlanNameLabel,
		SLALabel,
	} {
		instanceLabels[l] = newPlan.Labels[l]
	}
	instance.SetLabels(instanceLabels)

	gvk, err := gvkFromPlan(newPlan)
	if err != nil {
		return err
	}
	instance.SetGroupVersionKind(gvk)

	return cp.Client.Update(ctx, instance.GetUnstructured())
}
