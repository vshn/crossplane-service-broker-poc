package crossplane

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/lager"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/mitchellh/mapstructure"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	serviceMariadbDatabase = "mariadb-k8s-database"
)

// MariadbDatabaseServiceBinder defines a specific Mariadb service with enough data to retrieve connection credentials.
type MariadbDatabaseServiceBinder struct {
	instance  *composite.Unstructured
	resources []corev1.ObjectReference
	cp        *Crossplane
	logger    lager.Logger
}

// NewMariadbDatabaseServiceBinder instantiates a Mariadb service instance based on the given CompositeMariadbInstance.
func NewMariadbDatabaseServiceBinder(c *Crossplane, instance *composite.Unstructured, logger lager.Logger) *MariadbDatabaseServiceBinder {
	return &MariadbDatabaseServiceBinder{
		instance:  instance,
		resources: instance.GetResourceReferences(),
		cp:        c,
		logger:    logger,
	}
}

// FinishProvision is not implemented.
func (msb MariadbDatabaseServiceBinder) FinishProvision(ctx context.Context) error {
	return nil
}

type compositeMariaDBDatabaseInstanceSpecParameters struct {
	ParentReference string `mapstructure:"parent_reference" json:"parent_reference,omitempty"`
}
type compositeMariaDBDatabaseInstanceSpec struct {
	Parameters compositeMariaDBDatabaseInstanceSpecParameters `json:"parameters"`
}
type compositeMariaDBDatabaseInstance struct {
	Spec compositeMariaDBDatabaseInstanceSpec `json:"spec"`
}

// Bind creates a MariaDB binding composite.
func (msb MariadbDatabaseServiceBinder) Bind(ctx context.Context, bindingID string) (Credentials, error) {
	inst, err := msb.parseDBInstance()
	if err != nil {
		return nil, err
	}

	password, err := msb.cp.createBinding(
		ctx,
		bindingID,
		msb.instance.GetLabels()[InstanceIDLabel],
		inst.Spec.Parameters.ParentReference,
	)
	if err != nil {
		return nil, err
	}

	// In order to directly return the credentials we need to get the IP/port for this instance.
	secret, err := msb.cp.getSecret(ctx, spksNamespace, inst.Spec.Parameters.ParentReference)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = ErrInstanceNotReady
		}
		return nil, err
	}

	endpoint, err := mapMariadbEndpoint(secret.Data)
	if err != nil {
		return nil, err
	}

	creds := createCredentials(endpoint, bindingID, password, msb.instance.GetName())

	return creds, nil
}

// GetBinding returns credentials for MariaDB
func (msb MariadbDatabaseServiceBinder) GetBinding(ctx context.Context, bindingID string) (Credentials, error) {
	us, err := msb.cp.getSecret(ctx, spksNamespace, bindingID)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = apiresponses.ErrBindingNotFound
		}
		return nil, err
	}

	endpoint, err := mapMariadbEndpoint(us.Data)
	if err != nil {
		return nil, err
	}

	password := string(us.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey])

	creds := createCredentials(endpoint, bindingID, password, msb.instance.GetName())

	return creds, nil
}

// Unbind deletes the created User and Grant.
func (msb MariadbDatabaseServiceBinder) Unbind(ctx context.Context, bindingID string) error {
	return msb.cp.deleteBinding(ctx, bindingID)
}

// Endpoints returns the accessible endpoints for the db instance.
func (msb MariadbDatabaseServiceBinder) Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error) {
	inst, err := msb.parseDBInstance()
	if err != nil {
		return nil, err
	}

	secret, err := msb.cp.getSecret(ctx, spksNamespace, inst.Spec.Parameters.ParentReference)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			err = ErrInstanceNotReady
		}
		return nil, err
	}

	endpoint, err := mapMariadbEndpoint(secret.Data)
	if err != nil {
		return nil, err
	}
	return []Endpoint{
		*endpoint,
	}, nil
}

// Deprovision does nothing for MariaDB DB instances.
func (msb MariadbDatabaseServiceBinder) Deprovision(ctx context.Context) error {
	return nil
}

func (msb MariadbDatabaseServiceBinder) parseDBInstance() (*compositeMariaDBDatabaseInstance, error) {
	inst := &compositeMariaDBDatabaseInstance{}

	if err := mapstructure.Decode(msb.instance.Object, &inst); err != nil {
		return nil, fmt.Errorf("illegal instance %w", err)
	}
	return inst, nil
}

func mapMariadbEndpoint(data map[string][]byte) (*Endpoint, error) {
	hostBytes, ok := data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey]
	if !ok {
		return nil, apiresponses.ErrBindingNotFound
	}
	host := string(hostBytes)
	port, err := strconv.Atoi(string(data[runtimev1alpha1.ResourceCredentialsSecretPortKey]))
	if err != nil {
		return nil, err
	}
	return &Endpoint{
		Host:     host,
		Port:     int32(port),
		Protocol: "tcp",
	}, nil
}

func createCredentials(endpoint *Endpoint, username, password, database string) Credentials {
	uri := fmt.Sprintf("mysql://%s:%s@%s:%d/%s?reconnect=true", username, password, endpoint.Host, endpoint.Port, database)

	creds := Credentials{
		"host":     endpoint.Host,
		"hostname": endpoint.Host,
		runtimev1alpha1.ResourceCredentialsSecretPortKey: endpoint.Port,
		"name":     username,
		"database": database,
		runtimev1alpha1.ResourceCredentialsSecretUserKey:     username,
		runtimev1alpha1.ResourceCredentialsSecretPasswordKey: password,
		"database_uri": uri,
		"uri":          uri,
		"jdbcUrl":      fmt.Sprintf("jdbc:mysql://%s:%d/%s?user=%s&password=%s", endpoint.Host, endpoint.Port, database, username, password),
	}

	return creds
}
