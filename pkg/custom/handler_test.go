package custom

import (
	"broker/pkg/crossplane"
	"context"
	"testing"

	"code.cloudfoundry.org/lager"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	planName    = "fake"
	serviceName = "redis-k8s"
)

func createAPIHandler(objs []runtime.Object) *APIHandler {
	logger := lager.NewLogger("apihandler")
	s := scheme.Scheme
	if err := crossplane.SetupScheme(s); err != nil {
		panic(err)
	}

	plan := &v1beta1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: planName,
			Labels: map[string]string{
				crossplane.ServiceIDLabel:   serviceName,
				crossplane.PlanNameLabel:    planName,
				crossplane.ServiceNameLabel: serviceName,
			},
		},
		Spec: v1beta1.CompositionSpec{
			CompositeTypeRef: v1beta1.TypeReference{
				APIVersion: "syn.tools/v1alpha1",
				Kind:       "CompositeRedisInstance",
			},
		},
	}

	objs = append(objs, plan)
	cp := &crossplane.Crossplane{
		Client:     fake.NewFakeClientWithScheme(s, objs...),
		ServiceIDs: []string{serviceName},
	}
	return NewAPIHandler(cp, logger)
}

// func TestAPIHandler_Endpoints(t *testing.T) {
// 	instance := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
// 		Group:   "syn.tools",
// 		Kind:    "CompositeRedisInstance",
// 		Version: "v1alpha1",
// 	}))
// 	instance.SetName("test")
// 	instance.SetCompositionReference(&corev1.ObjectReference{
// 		Name: planName,
// 	})
// 	instance.SetLabels(map[string]string{
// 		crossplane.PlanNameLabel:    planName,
// 		crossplane.ServiceNameLabel: serviceName,
// 	})
// 	apiHandler := createAPIHandler([]runtime.Object{instance})
// 	l, err := apiHandler.Endpoints(context.Background(), "test")
// 	assert.NoError(t, err)
// 	assert.Len(t, l, 0)
// }

func TestAPIHandler_CreateUpdateServiceDefinition(t *testing.T) {
	err := createAPIHandler([]runtime.Object{}).CreateUpdateServiceDefinition(context.Background(), &ServiceDefinitionRequest{})
	assert.Error(t, err)
	assert.EqualError(t, err, "API not implemented (http code 404)")
}
