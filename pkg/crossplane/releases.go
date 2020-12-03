package crossplane

import (
	"context"
	"fmt"

	helmv1alpha1 "github.com/crossplane-contrib/provider-helm/apis/release/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func (cp *Crossplane) getRelease(ctx context.Context, releaseName string) (*helmv1alpha1.Release, error) {
	name := types.NamespacedName{Name: releaseName}
	release := &helmv1alpha1.Release{}
	if err := cp.Client.Get(ctx, name, release); err != nil {
		return nil, fmt.Errorf("Dynamic.Get(%s): %w", releaseName, err)
	}
	return release, nil
}
