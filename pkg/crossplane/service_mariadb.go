package crossplane

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/lager"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serviceMariadb = "mariadb-k8s"
)

// MariadbServiceBinder defines a specific Mariadb service with enough data to retrieve connection credentials.
type MariadbServiceBinder struct {
	instanceID     string
	instanceLabels map[string]string
	resources      []corev1.ObjectReference
	cp             *Crossplane
	logger         lager.Logger
}

// NewMariadbServiceBinder instantiates a Mariadb service instance based on the given CompositeMariadbInstance.
func NewMariadbServiceBinder(c *Crossplane, instance *composite.Unstructured, logger lager.Logger) *MariadbServiceBinder {
	return &MariadbServiceBinder{
		instanceID:     instance.GetName(),
		instanceLabels: instance.GetLabels(),
		resources:      instance.GetResourceReferences(),
		cp:             c,
		logger:         logger,
	}
}

// FinishProvision does extra provision work specific to MariaDB
func (msb MariadbServiceBinder) FinishProvision(ctx context.Context) error {
	// FIXME(mw): check if we can merge code with RedisServiceBinder
	releases := findResourceRefs(msb.resources, "Release")
	haproxy, err := findRelease(ctx, msb.cp, releases, helmHaProxyRelease)
	if err != nil {
		return err
	}
	hpr := NewHaProxyResource(haproxy, msb.cp)
	hc, err := hpr.GetCredentials(ctx)
	if err != nil {
		return err
	}
	hcreds := hc.(*HaProxyCredentials)

	s, err := msb.cp.getSecret(ctx, spksNamespace, msb.instanceID)
	if err != nil {
		return err
	}

	if s.Data == nil {
		s.Data = make(map[string][]byte)
	}

	if string(s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey]) != hcreds.Host {
		msb.logger.Info("update-secret", lager.Data{"endpoint": hcreds.Host, "namespace": spksNamespace})
		s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(hcreds.Host)
		if err := msb.cp.Client.Update(ctx, s); err != nil {
			return err
		}
	}

	return nil
}

// Bind is not implemented.
func (msb MariadbServiceBinder) Bind(_ context.Context, _ string) (Credentials, error) {
	return nil, apiresponses.NewFailureResponseBuilder(
		fmt.Errorf("Service MariaDB Galera Cluster is not bindable. "+
			"You can create a bindable database on this cluster using "+
			"cf create-service mariadb-k8s-database default my-mariadb-db -c '{\"parent_reference\": %q}'", msb.instanceID),
		http.StatusUnprocessableEntity,
		"binding-not-supported",
	).WithErrorKey("BindingNotSupported").Build()
}

// GetBinding is not implemented.
func (msb MariadbServiceBinder) GetBinding(_ context.Context, _ string) (Credentials, error) {
	return nil, ErrNotImplemented
}

// Unbind is not implemented.
func (msb MariadbServiceBinder) Unbind(_ context.Context, _ string) error {
	return ErrNotImplemented
}

// Endpoints is not implemented.
func (msb MariadbServiceBinder) Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error) {
	return []Endpoint{}, nil
}

// Deprovision removes the downstream namespace and checks if no DBs exist for this instance anymore.
func (msb MariadbServiceBinder) Deprovision(ctx context.Context) error {
	instanceList := &unstructured.UnstructuredList{}
	instanceList.SetGroupVersionKind(groupVersionKind)
	instanceList.SetKind("CompositeMariaDBDatabaseInstanceList")
	if err := msb.cp.Client.List(ctx, instanceList, client.MatchingLabels{
		ParentIDLabel: msb.instanceID,
	}); err != nil {
		return err
	}
	if len(instanceList.Items) > 0 {
		var instances []string
		for _, instance := range instanceList.Items {
			instances = append(instances, instance.GetName())
		}
		return apiresponses.NewFailureResponseBuilder(
			fmt.Errorf("instance is still in use by %q", strings.Join(instances, ", ")),
			http.StatusUnprocessableEntity,
			"deprovision-instance-in-use",
		).WithErrorKey("InUseError").Build()
	}
	return markNamespaceDeleted(ctx, msb.cp, msb.instanceID, msb.resources)
}
