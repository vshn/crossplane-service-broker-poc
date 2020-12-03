package custom

import (
	"broker/pkg/crossplane"
	"code.cloudfoundry.org/lager"
	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
	"net/http"
)

type APIHandler struct {
	c      *crossplane.Crossplane
	logger lager.Logger
}

func NewAPIHandler(c *crossplane.Crossplane, logger lager.Logger) *APIHandler {
	return &APIHandler{c, logger}
}

func notFoundError(description string, err error) error {
	return APIError{
		code: http.StatusNotFound,
		err: apiresponses.ErrorResponse{
			Error:       err.Error(),
			Description: description,
		},
	}
}
