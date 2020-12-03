package crossplane

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	helmv1alpha1 "github.com/crossplane-contrib/provider-helm/apis/release/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	helmHaProxyRelease = "haproxy"
	spksNamespace      = "spks-crossplane"
)

var (
	// ErrNotImplemented is the error returned for not implmemented functions
	ErrNotImplemented = apiresponses.
		NewFailureResponseBuilder(
			errors.New("not implemented"),
			http.StatusNotImplemented,
			"not-implemented").
		WithErrorKey("NotImplemented").
		Build()
)

// ErrInstanceNotReady is returned if credentials are fetched for an instance which is still provisioning.
var ErrInstanceNotReady = errors.New("instance not ready")

func (cp *Crossplane) getServices(ctx context.Context) ([]v1beta1.CompositeResourceDefinition, error) {
	xrds := &v1beta1.CompositeResourceDefinitionList{}

	req, err := labels.NewRequirement(ServiceIDLabel, selection.In, cp.ServiceIDs)
	if err != nil {
		return nil, err
	}

	err = cp.Client.List(ctx, xrds, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*req),
	})
	if err != nil {
		return nil, err
	}
	return xrds.Items, nil
}

// Credentials contain connection information for accessing a service.
type Credentials map[string]interface{}

// Endpoint describes available service endpoints.
type Endpoint struct {
	Host     string
	Port     int32
	Protocol string
}

// ServiceBinder is an interface for service specific implementation for binding, retrieving credentials, etc.
type ServiceBinder interface {
	FinishProvision(ctx context.Context) error
	Bind(ctx context.Context, bindingID string) (Credentials, error)
	GetBinding(ctx context.Context, bindingID string) (Credentials, error)
	Unbind(ctx context.Context, bindingID string) error
	Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error)
	Deprovision(ctx context.Context) error
}

// ServiceBinderFactory reads the composite's labels service name and instantiates an appropriate ServiceBinder.
func ServiceBinderFactory(c *Crossplane, instance *composite.Unstructured, logger lager.Logger) (ServiceBinder, error) {
	serviceName := instance.GetLabels()[ServiceNameLabel]
	switch serviceName {
	case serviceRedis:
		return NewRedisServiceBinder(c, instance, logger), nil
	case serviceMariadb:
		return NewMariadbServiceBinder(c, instance, logger), nil
	case serviceMariadbDatabase:
		return NewMariadbDatabaseServiceBinder(c, instance, logger), nil
	}
	return nil, fmt.Errorf("service binder %q not implemented", serviceName)
}

func findRelease(ctx context.Context, cp *Crossplane, refs []corev1.ObjectReference, name string) (*helmv1alpha1.Release, error) {
	for _, ref := range refs {
		release, err := cp.getRelease(ctx, ref.Name)
		if err != nil {
			return nil, err
		}

		chartName := release.Spec.ForProvider.Chart.Name
		if chartName == name {
			return release, nil
		}
	}
	return nil, fmt.Errorf("release %q not found", name)
}

func findResourceRefs(refs []corev1.ObjectReference, kind string) []corev1.ObjectReference {
	s := make([]corev1.ObjectReference, 0)
	for _, ref := range refs {
		if ref.Kind == kind {
			s = append(s, ref)
		}
	}
	return s
}

func findPort(ports []Port, name string) (int32, error) {
	for _, p := range ports {
		if p.Name == name {
			return p.Port, nil
		}
	}
	return 0, errors.New("port not found")
}

func markNamespaceDeleted(ctx context.Context, c *Crossplane, instanceID string, refs []corev1.ObjectReference) error {
	c.logger.Debug("mark namespace deleted", lager.Data{"instance-id": instanceID})

	releases := findResourceRefs(refs, "Release")
	if len(releases) <= 0 {
		return fmt.Errorf("no releases found for instance %q", instanceID)
	}
	name := types.NamespacedName{
		Namespace: releases[0].Namespace,
		Name:      releases[0].Name,
	}
	release := &helmv1alpha1.Release{}
	err := c.Client.Get(ctx, name, release)
	if err != nil {
		return fmt.Errorf("get release(%q - %q): %w", name.Namespace, name.Name, err)
	}

	klient, err := c.GetDownstreamClientForHelmRelease(ctx, release)
	if err != nil {
		return err
	}
	ns := corev1.Namespace{}
	if err := klient.Get(ctx, types.NamespacedName{Name: instanceID}, &ns); err != nil {
		return fmt.Errorf("get namespace(%q): %w", instanceID, err)
	}

	ns.Labels[DeletedLabel] = "true"
	ns.Annotations[DeletionTimestampAnnotation] = metav1.NowMicro().UTC().Format(metav1.RFC3339Micro)

	if err := klient.Patch(ctx, &ns, client.Merge); err != nil {
		return fmt.Errorf("patch namespace(%q): %w", instanceID, err)
	}

	c.logger.Debug("success marking namespace deleted", lager.Data{"instance-id": instanceID})

	return nil
}
