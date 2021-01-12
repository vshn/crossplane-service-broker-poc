package crossplane

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
	"github.com/pivotal-cf/brokerapi/v7/middlewares"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	// SynToolsBase is the base domain
	SynToolsBase = "service.syn.tools"

	// DescriptionAnnotation of the instance
	DescriptionAnnotation = SynToolsBase + "/description"
	// MetadataAnnotation of the instance
	MetadataAnnotation = SynToolsBase + "/metadata"
	// DeletionTimestampAnnotation marks when an object got deleted
	DeletionTimestampAnnotation = SynToolsBase + "/deletionTimestamp"
	// TagsAnnotation of the instance
	TagsAnnotation = SynToolsBase + "/tags"
)

const (
	// ServiceNameLabel of the instance
	ServiceNameLabel = SynToolsBase + "/name"
	// ServiceIDLabel of the instance
	ServiceIDLabel = SynToolsBase + "/id"
	// PlanNameLabel of the instance
	PlanNameLabel = SynToolsBase + "/plan"
	// InstanceIDLabel of the instance
	InstanceIDLabel = SynToolsBase + "/instance"
	// ParentIDLabel of the instance
	ParentIDLabel = SynToolsBase + "/parent"
	// BindableLabel of the instance
	BindableLabel = SynToolsBase + "/bindable"
	// DeletedLabel marks an object as deleted to clean up
	DeletedLabel = SynToolsBase + "/deleted"
	// ClusterLabel name of the cluster this instance is deployed to
	ClusterLabel = SynToolsBase + "/cluster"
	// SLALabel SLA level for this instance
	SLALabel = SynToolsBase + "/sla"
)

const (
	// SLAPremium represents the string for the premium SLA
	SLAPremium = "premium"
	// SLAStandard represents the string for the standard SLA
	SLAStandard = "standard"
)

// ConvertError converts an error to a proper API error
func ConvertError(ctx context.Context, err error) error {
	var kErr *k8serrors.StatusError
	if errors.As(err, &kErr) {
		err = apiresponses.NewFailureResponseBuilder(
			kErr,
			int(kErr.ErrStatus.Code),
			"invalid",
		).WithErrorKey(string(kErr.ErrStatus.Reason)).Build()
	}
	id, ok := ctx.Value(middlewares.CorrelationIDKey).(string)
	if !ok {
		id = "unknown"
	}
	var apiErr *apiresponses.FailureResponse
	if errors.As(err, &apiErr) {
		return apiErr.AppendErrorMessage(fmt.Sprintf("(correlation-id: %q)", id))
	}
	return apiresponses.NewFailureResponseBuilder(
		fmt.Errorf("%w (correlation-id: %q)", err, id),
		http.StatusInternalServerError,
		"internal-server-error",
	).Build()
}

func getPlanLevel(name string) string {
	tmp := strings.Split(name, "-")
	return tmp[0]
}
