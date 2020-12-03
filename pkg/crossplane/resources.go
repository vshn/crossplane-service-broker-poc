package crossplane

import (
	"context"
	"strconv"

	helmv1alpha1 "github.com/crossplane-contrib/provider-helm/apis/release/v1alpha1"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CredentialExtractor retrieve binding credentials.
type CredentialExtractor interface {
	GetCredentials(ctx context.Context) (interface{}, error)
}

// Port describes available ports for HaProxyCredentials based on a service.
type Port struct {
	Name string
	Port int32
}

// HaProxyCredentials contains data to connect via haproxy service to a backend.
type HaProxyCredentials struct {
	Host  string
	Ports []Port
}

// HaProxyResource is CredentialExtractor.
type HaProxyResource struct {
	release *helmv1alpha1.Release
	cp      *Crossplane
}

// NewHaProxyResource instantiates a HaProxyResource to be used as a CredentialExtractor.
func NewHaProxyResource(release *helmv1alpha1.Release, cp *Crossplane) *HaProxyResource {
	return &HaProxyResource{
		release: release,
		cp:      cp,
	}
}

// GetCredentials retrieves service details from the respective downstream cluster.
func (hpr *HaProxyResource) GetCredentials(ctx context.Context) (interface{}, error) {
	klient, err := hpr.cp.GetDownstreamClientForHelmRelease(ctx, hpr.release)
	if err != nil {
		return nil, err
	}

	svc := &corev1.Service{}
	if err := klient.Get(ctx, types.NamespacedName{
		Name:      helmHaProxyRelease,
		Namespace: hpr.release.Spec.ForProvider.Namespace,
	}, svc); err != nil {
		return nil, err
	}

	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return nil, ErrInstanceNotReady
	}

	creds := HaProxyCredentials{
		Host:  svc.Status.LoadBalancer.Ingress[0].IP,
		Ports: make([]Port, len(svc.Spec.Ports)),
	}

	for i, p := range svc.Spec.Ports {
		creds.Ports[i] = Port{
			Name: p.Name,
			Port: p.Port,
		}
	}

	return &creds, nil
}

// SecretCredentials encapsulates a password retrieved from a k8s secret.
type SecretCredentials struct {
	Endpoint string
	Port     int32
	Password string
}

// SecretResource is a credential extractor.
type SecretResource struct {
	namespace   string
	resourceRef corev1.ObjectReference
	cp          *Crossplane
}

// NewSecretResource instantiates a SecretResource to be used as a CredentialExtractor.
func NewSecretResource(namespace string, resourceRef corev1.ObjectReference, cp *Crossplane) *SecretResource {
	return &SecretResource{
		namespace:   namespace,
		resourceRef: resourceRef,
		cp:          cp,
	}
}

// GetCredentials retrieves the secret specified by the resourceRef and returns the password within that secret.
func (sr *SecretResource) GetCredentials(ctx context.Context) (interface{}, error) {
	s, err := sr.cp.getSecret(ctx, sr.namespace, sr.resourceRef.Name)
	if err != nil {
		return nil, err
	}

	sport := string(s.Data[runtimev1alpha1.ResourceCredentialsSecretPortKey][:])
	port, err := strconv.Atoi(sport)
	if err != nil {
		return nil, err
	}

	creds := SecretCredentials{
		Endpoint: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey][:]),
		Port:     int32(port),
		Password: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey][:]),
	}
	return &creds, nil
}
