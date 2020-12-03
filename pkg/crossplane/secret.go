package crossplane

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (cp *Crossplane) getSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	secretRef := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	s := &corev1.Secret{}
	if err := cp.Client.Get(ctx, secretRef, s); err != nil {
		return nil, fmt.Errorf("unable to get secret: %w", err)
	}
	if s.Data == nil {
		return nil, errors.New("nil secret data")
	}
	return s, nil
}
