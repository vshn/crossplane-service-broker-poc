package crossplane

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/password"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   "syn.tools",
		Version: "v1alpha1",
		Kind:    "CompositeMariaDBUserInstance",
	}
	planName   = "mariadb-user"
	secretName = "%s-password"
)

func (cp *Crossplane) createBinding(ctx context.Context, bindingID, instanceID, parentReference string) (string, error) {
	pw, err := password.Generate()
	if err != nil {
		return "", err
	}

	labels := map[string]string{
		InstanceIDLabel: instanceID,
		ParentIDLabel:   parentReference,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(secretName, bindingID),
			Namespace: spksNamespace,
			Labels:    labels,
		},
		Data: map[string][]byte{
			runtimev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(pw),
		},
	}
	err = cp.Client.Create(ctx, secret)
	if errors.IsAlreadyExists(err) {
		err = cp.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)
	}
	if err != nil {
		return "", err
	}

	cmp := composite.New(composite.WithGroupVersionKind(groupVersionKind))
	cmp.SetName(bindingID)
	cmp.SetLabels(labels)
	cmp.SetCompositionReference(&corev1.ObjectReference{
		Name: planName,
	})
	if err := fieldpath.Pave(cmp.Object).SetValue(instanceSpecParamsParentReferencePath, parentReference); err != nil {
		return "", err
	}

	cp.logger.Debug("create-binding", lager.Data{"instance": cmp})
	err = cp.Client.Create(ctx, cmp)
	if err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}
	return string(secret.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey]), nil
}

func (cp *Crossplane) deleteBinding(ctx context.Context, bindingID string) error {
	cmp := composite.New(composite.WithGroupVersionKind(groupVersionKind))
	cmp.SetName(bindingID)
	if err := cp.Client.Delete(ctx, cmp); err != nil {
		return err
	}

	// TODO: figure out a better way to delete the password secret
	// If we delete the secret to quickly, the provider-sql can't deprovision the user
	time.Sleep(5 * time.Second)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(secretName, bindingID),
			Namespace: spksNamespace,
		},
	}
	return cp.Client.Delete(ctx, secret)
}
