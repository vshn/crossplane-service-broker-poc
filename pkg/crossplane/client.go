package crossplane

import (
	"context"
	"errors"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	helm "github.com/crossplane-contrib/provider-helm/apis"
	helmv1alpha1 "github.com/crossplane-contrib/provider-helm/apis/release/v1alpha1"
	"github.com/crossplane-contrib/provider-helm/apis/v1alpha1"
	helmclient "github.com/crossplane-contrib/provider-helm/pkg/clients"
	crossplane "github.com/crossplane/crossplane/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Crossplane client to access crossplane resources.
type Crossplane struct {
	Client            k8sclient.Client
	logger            lager.Logger
	DownstreamClients map[string]k8sclient.Client
	ServiceIDs        []string
}

// SetupScheme configures the given runtime.Scheme with all requried resources
func SetupScheme(scheme *runtime.Scheme) error {
	sBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		metav1.AddToGroupVersion(s, schema.GroupVersion{Group: "syn.tools", Version: "v1alpha1"})
		return nil
	})
	if err := sBuilder.AddToScheme(scheme); err != nil {
		return err
	}
	if err := helm.AddToScheme(scheme); err != nil {
		return err
	}
	if err := crossplane.AddToScheme(scheme); err != nil {
		return err
	}

	return nil
}

// New instantiates a crossplane client.
func New(serviceIDs []string, logger lager.Logger) (*Crossplane, error) {
	if err := SetupScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	k, err := k8sclient.New(config, k8sclient.Options{})
	if err != nil {
		return nil, err
	}

	cp := Crossplane{
		Client:            k,
		logger:            logger,
		DownstreamClients: make(map[string]k8sclient.Client, 0),
		ServiceIDs:        serviceIDs,
	}

	return &cp, nil
}

// GetDownstreamClientForHelmRelease retrieves the provider config of a helm release, fetches the secret containing a kubeconfig from
// the specified secretRef and instantiates necessary k8s clients.
func (cp *Crossplane) GetDownstreamClientForHelmRelease(ctx context.Context, release *helmv1alpha1.Release) (k8sclient.Client, error) {
	if kubeconfig := os.Getenv(clientcmd.RecommendedConfigPathEnvVar); len(kubeconfig) > 0 {
		// Reuse local cluster for dev/debugging instead of remote downstream cluster,
		// which won't be accessible.
		config := ctrl.GetConfigOrDie()
		return k8sclient.New(config, k8sclient.Options{})
	}

	name := release.Spec.ResourceSpec.ProviderConfigReference.Name

	if _, ok := cp.DownstreamClients[name]; ok {
		return cp.DownstreamClients[name], nil
	}

	providerconfig := &v1alpha1.ProviderConfig{}
	ns := types.NamespacedName{Name: name}
	if err := cp.Client.Get(ctx, ns, providerconfig); err != nil {
		return nil, err
	}

	secretRef := types.NamespacedName{
		Namespace: providerconfig.Spec.Credentials.SecretRef.Namespace,
		Name:      providerconfig.Spec.Credentials.SecretRef.Name,
	}
	s := &corev1.Secret{}
	if err := cp.Client.Get(ctx, secretRef, s); err != nil {
		return nil, fmt.Errorf("unable to get secret: %w", err)
	}
	if s.Data == nil {
		return nil, errors.New("nil secret data")
	}

	config, err := helmclient.NewRestConfig(s.Data)
	if err != nil {
		return nil, err
	}

	k, err := helmclient.NewKubeClient(config)
	if err != nil {
		return nil, err
	}

	cp.DownstreamClients[name] = k

	return k, nil
}
