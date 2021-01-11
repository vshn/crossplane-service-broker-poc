package crossplanebroker

import (
	"context"
	"errors"
	"net/http"

	"broker/pkg/crossplane"

	"code.cloudfoundry.org/lager"
	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/pivotal-cf/brokerapi/v7/domain"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/brokerapi/v7/middlewares"
	corev1 "k8s.io/api/core/v1"
)

// CrossplaneBroker implements the Crossplane service broker
type CrossplaneBroker struct {
	c      *crossplane.Crossplane
	logger lager.Logger
}

// New is the constructor for Crossplane
func New(c *crossplane.Crossplane, logger lager.Logger) (*CrossplaneBroker, error) {
	return &CrossplaneBroker{
		c:      c,
		logger: logger,
	}, nil
}

// Services returns the catalog
func (b *CrossplaneBroker) Services(ctx context.Context) ([]domain.Service, error) {
	logger := requestScopedLogger(ctx, b.logger)
	logger.Info("get-catalog")

	return b.c.GetCatalog(ctx)
}

// Provision a service instance
func (b *CrossplaneBroker) Provision(ctx context.Context, instanceID string, details domain.ProvisionDetails, asyncAllowed bool) (spec domain.ProvisionedServiceSpec, err error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID})
	logger.Info("provision-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})

	if !asyncAllowed {
		return spec, apiresponses.ErrAsyncRequired
	}

	plan, err := b.c.GetPlan(ctx, details.PlanID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	if instance, exists, err := b.c.InstanceExists(ctx, instanceID, plan); err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	} else if exists {
		if instance.GetLabels()[crossplane.PlanNameLabel] == plan.Labels[crossplane.PlanNameLabel] {
			// To avoid having to compare parameters,
			// only instances without any parameters are considered to be equal to another (i.e. existing)
			if details.RawParameters == nil {
				return domain.ProvisionedServiceSpec{
					AlreadyExists: true,
				}, nil
			}
		}
		return spec, apiresponses.ErrInstanceAlreadyExists
	}

	err = b.c.CreateInstance(ctx, instanceID, details.RawParameters, plan)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	return domain.ProvisionedServiceSpec{
		IsAsync: true,
	}, nil
}

// Deprovision deletes a service instance
func (b *CrossplaneBroker) Deprovision(ctx context.Context, instanceID string, details domain.DeprovisionDetails, asyncAllowed bool) (domain.DeprovisionServiceSpec, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID})
	logger.Info("deprovision-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})

	spec := domain.DeprovisionServiceSpec{}

	plan, err := b.c.GetPlan(ctx, details.PlanID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	instance, exists, err := b.c.InstanceExists(ctx, instanceID, plan)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}
	if !exists {
		return spec, apiresponses.ErrInstanceDoesNotExist
	}

	sb, err := crossplane.ServiceBinderFactory(b.c, instance, logger)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}
	if err := sb.Deprovision(ctx); err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	if err := b.c.DeleteInstance(ctx, instance.GetName(), plan); err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	return domain.DeprovisionServiceSpec{
		IsAsync: false,
	}, nil
}

// Bind creates a binding
func (b *CrossplaneBroker) Bind(ctx context.Context, instanceID, bindingID string, details domain.BindDetails, asyncAllowed bool) (domain.Binding, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID, "binding-id": bindingID})
	logger.Info("bind-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})

	spec := domain.Binding{
		IsAsync: false,
	}

	plan, err := b.c.GetPlan(ctx, details.PlanID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	instance, exists, err := b.c.InstanceExists(ctx, instanceID, plan)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	} else if !exists {
		return spec, apiresponses.ErrInstanceDoesNotExist
	}

	if instance.GetCondition(v1alpha1.TypeReady).Status != corev1.ConditionTrue {
		return spec, apiresponses.ErrConcurrentInstanceAccess
	}

	sb, err := crossplane.ServiceBinderFactory(b.c, instance, logger)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	if err := sb.FinishProvision(ctx); err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	creds, err := sb.Bind(ctx, bindingID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	spec.Credentials = creds

	return spec, nil
}

// Unbind deletes a binding
func (b *CrossplaneBroker) Unbind(ctx context.Context, instanceID, bindingID string, details domain.UnbindDetails, asyncAllowed bool) (domain.UnbindSpec, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID, "binding-id": bindingID})
	logger.Info("unbind-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})

	spec := domain.UnbindSpec{
		IsAsync: false,
	}

	plan, err := b.c.GetPlan(ctx, details.PlanID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	instance, exists, err := b.c.InstanceExists(ctx, instanceID, plan)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	} else if !exists {
		return spec, apiresponses.ErrInstanceDoesNotExist
	}

	sb, err := crossplane.ServiceBinderFactory(b.c, instance, logger)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	return spec, sb.Unbind(ctx, bindingID)
}

// LastOperation returns the status of the last async operation
func (b *CrossplaneBroker) LastOperation(ctx context.Context, instanceID string, details domain.PollDetails) (domain.LastOperation, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID})
	logger.Info("last-operation", lager.Data{"operation-data": details.OperationData, "plan-id": details.PlanID, "service-id": details.ServiceID})

	instance, err := b.c.GetInstance(ctx, instanceID)
	if err != nil {
		if errors.Is(err, crossplane.ErrInstanceNotFound) {
			err = apiresponses.ErrInstanceDoesNotExist
		}
		return domain.LastOperation{}, crossplane.ConvertError(ctx, err)
	}

	condition := instance.GetCondition(v1alpha1.TypeReady)
	op := domain.LastOperation{
		Description: string(condition.Reason),
	}
	switch condition.Reason {
	case v1alpha1.ReasonAvailable:
		op.State = domain.Succeeded
		sb, err := crossplane.ServiceBinderFactory(b.c, instance, logger)
		if err != nil {
			return domain.LastOperation{}, crossplane.ConvertError(ctx, err)
		}
		logger.Info("finish-provision")
		if err := sb.FinishProvision(ctx); err != nil {
			return domain.LastOperation{}, crossplane.ConvertError(ctx, err)
		}
	case v1alpha1.ReasonCreating:
		op.State = domain.InProgress
	default:
		op.State = domain.Failed
	}
	return op, nil
}

// Update implements updates
func (b *CrossplaneBroker) Update(ctx context.Context, instanceID string, details domain.UpdateDetails, asyncAllowed bool) (domain.UpdateServiceSpec, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID})
	logger.Info("update-service-instance", lager.Data{"plan-id": details.PlanID, "service-id": details.ServiceID})

	spec := domain.UpdateServiceSpec{}

	if err := b.c.UpdateInstanceSLA(ctx, instanceID, details.ServiceID, details.PlanID); err != nil {
		switch err {
		case crossplane.ErrSLAChangeNotPermitted, crossplane.ErrClusterChangeNotPermitted, crossplane.ErrServiceUpdateNotPermitted:
			err = apiresponses.NewFailureResponse(err, http.StatusUnprocessableEntity, "update-instance-failed")
		case crossplane.ErrInstanceNotFound:
			err = apiresponses.ErrInstanceDoesNotExist
		}
		return spec, crossplane.ConvertError(ctx, err)
	}

	return spec, nil
}

// GetBinding returns a previously created binding
func (b *CrossplaneBroker) GetBinding(ctx context.Context, instanceID, bindingID string) (domain.GetBindingSpec, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID, "binding-id": bindingID})
	logger.Info("get-binding", lager.Data{"binding-id": bindingID})

	spec := domain.GetBindingSpec{}

	instance, err := b.c.GetInstance(ctx, instanceID)
	if err != nil {
		if errors.Is(err, crossplane.ErrInstanceNotFound) {
			err = apiresponses.ErrInstanceDoesNotExist
		}
		return domain.GetBindingSpec{}, crossplane.ConvertError(ctx, err)
	}

	if instance.GetCondition(v1alpha1.TypeReady).Status != corev1.ConditionTrue {
		return spec, apiresponses.ErrConcurrentInstanceAccess
	}

	sb, err := crossplane.ServiceBinderFactory(b.c, instance, logger)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	creds, err := sb.GetBinding(ctx, bindingID)
	if err != nil {
		return spec, crossplane.ConvertError(ctx, err)
	}

	spec.Credentials = creds

	return spec, nil
}

// GetInstance returns a service instance
func (b *CrossplaneBroker) GetInstance(ctx context.Context, instanceID string) (domain.GetInstanceDetailsSpec, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID})
	logger.Info("get-instance")

	instance, err := b.c.GetInstance(ctx, instanceID)
	if err != nil {
		if errors.Is(err, crossplane.ErrInstanceNotFound) {
			err = apiresponses.ErrInstanceDoesNotExist
		}
		return domain.GetInstanceDetailsSpec{}, crossplane.ConvertError(ctx, err)
	}

	params, err := fieldpath.Pave(instance.Object).GetValue(crossplane.InstanceSpecParamsPath)
	if err != nil {
		return domain.GetInstanceDetailsSpec{}, err
	}

	spec := domain.GetInstanceDetailsSpec{
		PlanID:     instance.GetCompositionReference().Name,
		ServiceID:  instance.GetLabels()[crossplane.ServiceIDLabel],
		Parameters: params,
	}
	return spec, nil
}

// LastBindingOperation is not implemented since async bindings are not supported
func (b *CrossplaneBroker) LastBindingOperation(ctx context.Context, instanceID, bindingID string, details domain.PollDetails) (domain.LastOperation, error) {
	logger := requestScopedLogger(ctx, b.logger).WithData(lager.Data{"instance-id": instanceID, "binding-id": bindingID})
	logger.Info("last-binding-operation", lager.Data{"operation-data": details.OperationData, "plan-id": details.PlanID, "service-id": details.ServiceID})

	return domain.LastOperation{}, crossplane.ErrNotImplemented
}

func requestScopedLogger(ctx context.Context, logger lager.Logger) lager.Logger {
	id, ok := ctx.Value(middlewares.CorrelationIDKey).(string)
	if !ok {
		id = "unknown"
	}

	return logger.WithData(lager.Data{"correlation-id": id})
}
