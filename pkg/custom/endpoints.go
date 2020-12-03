package custom

import (
	"broker/pkg/crossplane"
	"context"
	"errors"
	"strconv"
)

func (h APIHandler) Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error) {
	instance, err := h.c.GetInstance(ctx, instanceID)
	if err != nil {
		if errors.Is(err, crossplane.ErrInstanceNotFound) {
			return nil, notFoundError("instance not found", err)
		}
		return nil, err
	}

	sb, err := crossplane.ServiceBinderFactory(h.c, instance, h.logger)
	if err != nil {
		return nil, err
	}

	srvEndpoints, err := sb.Endpoints(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	endpoints := make([]Endpoint, 0)
	for _, v := range srvEndpoints {
		endpoints = append(endpoints, Endpoint{
			Destination: v.Host,
			Ports:       strconv.Itoa(int(v.Port)),
			Protocol:    v.Protocol,
		})
	}
	return endpoints, nil
}
