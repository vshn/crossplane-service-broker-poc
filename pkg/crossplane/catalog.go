package crossplane

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"k8s.io/utils/pointer"
)

// GetCatalog returns the full catalog
func (cp *Crossplane) GetCatalog(ctx context.Context) ([]domain.Service, error) {
	services, err := cp.getServicesForBroker(ctx)
	if err != nil {
		return nil, err
	}

	return services, nil

}

func (cp *Crossplane) getServicesForBroker(ctx context.Context) ([]domain.Service, error) {
	services := make([]domain.Service, 0)

	xrds, err := cp.getServices(ctx)
	if err != nil {
		return nil, err
	}

	for _, xrd := range xrds {
		serviceID, ok := xrd.Labels[ServiceIDLabel]
		if !ok {
			return nil, fmt.Errorf("Could not find service id of XRD %s", xrd.Name)
		}
		serviceName, ok := xrd.Labels[ServiceNameLabel]
		if !ok {
			return nil, fmt.Errorf("Could not find service name of XRD %s", xrd.Name)
		}

		plans, err := cp.getPlansForBroker(ctx, []string{serviceID})

		if err != nil {
			cp.logger.Error(fmt.Sprint("Could not get plans for service"), err, lager.Data{"serviceId": serviceID})
		}

		meta := &domain.ServiceMetadata{}
		if err := json.Unmarshal([]byte(xrd.Annotations[MetadataAnnotation]), meta); err != nil {
			cp.logger.Error("parse-metadata", err)
			meta.DisplayName = serviceName
		}
		bindable := true
		if b, ok := xrd.Labels[BindableLabel]; ok {
			bindable, err = strconv.ParseBool(b)
			if err != nil {
				cp.logger.Error("parse-bindable", err)
			}
		}

		services = append(services, domain.Service{
			ID:                   serviceID,
			Name:                 serviceName,
			Description:          xrd.Annotations[DescriptionAnnotation],
			Bindable:             bindable,
			InstancesRetrievable: true,
			BindingsRetrievable:  bindable,
			PlanUpdatable:        false,
			Plans:                plans,
			Metadata:             meta,
		})
	}

	return services, nil
}

func (cp *Crossplane) getPlansForBroker(ctx context.Context, serviceIDs []string) ([]domain.ServicePlan, error) {
	plans := make([]domain.ServicePlan, 0)

	compositions, err := cp.getPlansForService(ctx, serviceIDs)
	if err != nil {
		return nil, err
	}

	for _, composition := range compositions {
		planName := composition.Labels[PlanNameLabel]
		meta := &domain.ServicePlanMetadata{}
		if err := json.Unmarshal([]byte(composition.Annotations[MetadataAnnotation]), meta); err != nil {
			cp.logger.Error("parse-metadata", err)
			meta.DisplayName = planName
		}
		bindable := true
		if b, ok := composition.Labels[BindableLabel]; ok {
			bindable, err = strconv.ParseBool(b)
			if err != nil {
				cp.logger.Error("parse-bindable", err)
			}
		}
		plans = append(plans, domain.ServicePlan{
			ID:          composition.Name,
			Name:        planName,
			Description: composition.Annotations[DescriptionAnnotation],
			Free:        pointer.BoolPtr(false),
			Bindable:    &bindable,
			Metadata:    meta,
		})
	}

	return plans, nil
}
