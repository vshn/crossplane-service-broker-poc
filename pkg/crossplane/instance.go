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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// ErrInstanceNotFound is an instance doesn't exist
var ErrInstanceNotFound = errors.New("instance not found")

// CreateInstance creates a service instance
func (cp *Crossplane) CreateInstance(ctx context.Context, instanceID string, parameters json.RawMessage, plan *v1beta1.Composition) error {
	labels := map[string]string{
		InstanceIDLabel: instanceID,
	}
	// Copy relevant labels from plan
	for _, l := range []string{ServiceIDLabel, ServiceNameLabel, PlanNameLabel, ClusterLabel} {
		labels[l] = plan.Labels[l]
	}

	groupVersion, err := schema.ParseGroupVersion(plan.Spec.CompositeTypeRef.APIVersion)
	if err != nil {
		return err
	}
	gvk := groupVersion.WithKind(plan.Spec.CompositeTypeRef.Kind)

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
	groupVersion, err := schema.ParseGroupVersion(plan.Spec.CompositeTypeRef.APIVersion)
	if err != nil {
		return err
	}
	gvk := groupVersion.WithKind(plan.Spec.CompositeTypeRef.Kind)

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
	groupVersion, err := schema.ParseGroupVersion(plan.Spec.CompositeTypeRef.APIVersion)
	if err != nil {
		return nil, err
	}
	gvk := groupVersion.WithKind(plan.Spec.CompositeTypeRef.Kind)

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
		cp.logger.Debug("instance-not-found-labels-dont-match", lager.Data{"instance-id": instanceID, "plan-labels": plan.Labels})
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
