package crossplane

import (
	"context"
	"sort"

	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (cp *Crossplane) getPlansForService(ctx context.Context, serviceIDs []string) ([]v1beta1.Composition, error) {
	req, err := labels.NewRequirement(ServiceIDLabel, selection.In, serviceIDs)
	if err != nil {
		return nil, err
	}

	compositions := &v1beta1.CompositionList{}
	err = cp.Client.List(ctx, compositions, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*req),
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(compositions.Items, func(i, j int) bool {
		return compositions.Items[i].Labels[PlanNameLabel] < compositions.Items[j].Labels[PlanNameLabel]
	})

	return compositions.Items, nil
}

// GetPlan searchs a plan by ID
func (cp *Crossplane) GetPlan(ctx context.Context, planID string) (*v1beta1.Composition, error) {
	composition := &v1beta1.Composition{}
	err := cp.Client.Get(ctx, types.NamespacedName{Name: planID}, composition)
	if err != nil {
		return nil, err
	}

	return composition, nil
}
