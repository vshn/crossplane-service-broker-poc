package crossplane

import (
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/lager"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	corev1 "k8s.io/api/core/v1"
)

const (
	serviceRedis = "redis-k8s"
)

// RedisServiceBinder defines a specific redis service with enough data to retrieve connection credentials.
type RedisServiceBinder struct {
	instanceID string
	resources  []corev1.ObjectReference
	cp         *Crossplane
	logger     lager.Logger
}

// NewRedisServiceBinder instantiates a redis service instance based on the given CompositeRedisInstance.
func NewRedisServiceBinder(c *Crossplane, instance *composite.Unstructured, logger lager.Logger) *RedisServiceBinder {
	return &RedisServiceBinder{
		instanceID: instance.GetName(),
		resources:  instance.GetResourceReferences(),
		cp:         c,
		logger:     logger,
	}
}

// FinishProvision does extra provision work specific to Redis
func (rsb RedisServiceBinder) FinishProvision(ctx context.Context) error {
	return nil
}

// Bind retrieves the necessary external IP, password and ports.
func (rsb RedisServiceBinder) Bind(ctx context.Context, bindingID string) (Credentials, error) {
	return rsb.GetBinding(ctx, bindingID)
}

// GetBinding always returns the same credentials for Redis
func (rsb RedisServiceBinder) GetBinding(ctx context.Context, _ string) (Credentials, error) {
	creds := make(Credentials)

	secrets := findResourceRefs(rsb.resources, "Secret")
	if len(secrets) != 1 {
		return nil, errors.New("resourceRef contains more than one secret")
	}
	sr := NewSecretResource(spksNamespace, secrets[0], rsb.cp)
	sc, err := sr.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}
	creds[runtimev1alpha1.ResourceCredentialsSecretPasswordKey] = sc.(*SecretCredentials).Password

	releases := findResourceRefs(rsb.resources, "Release")
	haproxy, err := findRelease(ctx, rsb.cp, releases, helmHaProxyRelease)
	if err != nil {
		return nil, err
	}
	hpr := NewHaProxyResource(haproxy, rsb.cp)
	hcreds, err := hpr.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	c, err := mapRedisCredentials(rsb.instanceID, hcreds.(*HaProxyCredentials))
	if err != nil {
		return nil, err
	}
	for k, v := range c {
		creds[k] = v
	}

	return creds, nil
}

// Unbind does nothing for redis bindings.
func (rsb RedisServiceBinder) Unbind(ctx context.Context, bindingID string) error {
	return nil
}

// Endpoints retrieves host/port/protocol for the redis instance.
func (rsb RedisServiceBinder) Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error) {
	releases := findResourceRefs(rsb.resources, "Release")
	haproxy, err := findRelease(ctx, rsb.cp, releases, helmHaProxyRelease)
	if err != nil {
		return nil, err
	}
	hpr := NewHaProxyResource(haproxy, rsb.cp)
	hc, err := hpr.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	ep, err := mapRedisEndpoints(hc.(*HaProxyCredentials))
	if err != nil {
		return nil, err
	}

	d := make([]Endpoint, 0)
	for _, v := range ep {
		d = append(d, v)
	}
	return d, nil
}

// Deprovision removes the downstream namespace.
func (rsb RedisServiceBinder) Deprovision(ctx context.Context) error {
	return markNamespaceDeleted(ctx, rsb.cp, rsb.instanceID, rsb.resources)
}

func mapRedisEndpoints(hcreds *HaProxyCredentials) (map[string]Endpoint, error) {
	port, err := findPort(hcreds.Ports, "redis")
	if err != nil {
		port, err = findPort(hcreds.Ports, "frontend")
		if err != nil {
			return nil, err
		}
	}

	sentinelPort, err := findPort(hcreds.Ports, "sentinel")
	if err != nil {
		return nil, err
	}

	return map[string]Endpoint{
		"master": {
			Host:     hcreds.Host,
			Port:     port,
			Protocol: "tcp",
		},
		"sentinel": {
			Host:     hcreds.Host,
			Port:     sentinelPort,
			Protocol: "tcp",
		},
	}, nil
}

func mapRedisCredentials(instanceID string, hcreds *HaProxyCredentials) (Credentials, error) {
	endpoints, err := mapRedisEndpoints(hcreds)
	if err != nil {
		return nil, err
	}

	host := endpoints["master"].Host
	port := endpoints["master"].Port
	sentinelPort := endpoints["sentinel"].Port

	creds := Credentials{
		"host":   host,
		"master": fmt.Sprintf("redis://%s", instanceID),
	}

	creds[runtimev1alpha1.ResourceCredentialsSecretPortKey] = port
	creds["sentinels"] = []Credentials{
		{
			"host": host,
			"port": sentinelPort,
		},
	}
	creds["servers"] = []Credentials{
		{
			"host": hcreds.Host,
			"port": port,
		},
	}

	return creds, nil
}
