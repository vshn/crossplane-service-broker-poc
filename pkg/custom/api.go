package custom

import (
	"broker/pkg/crossplane"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	"github.com/pivotal-cf/brokerapi/v7/domain/apiresponses"
)

type API struct {
	handler CustomAPI
	logger  lager.Logger
}

type APIError struct {
	code int
	err  apiresponses.ErrorResponse
}

func (ae APIError) Error() string {
	return fmt.Sprintf("%s (http code %d)", ae.err.Error, ae.code)
}

func NewAPI(router *mux.Router, handler *APIHandler, logger lager.Logger) *API {
	api := API{
		handler: handler,
		logger:  logger,
	}

	router.HandleFunc("/custom/service_instances/{service_instance_id}/endpoint", api.Endpoints).Methods("GET")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/usage", api.ServiceUsage).Methods("GET")
	router.HandleFunc("/custom/admin/service-definition", api.CreateUpdateServiceDefinition).Methods("POST")
	router.HandleFunc("/custom/admin/service-definition/{id}", api.DeleteServiceDefinition).Methods("DELETE")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups", api.CreateBackup).Methods("POST")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups/{backup_id}", api.DeleteBackup).Methods("DELETE")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups/{backup_id}", api.Backup).Methods("GET")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups", api.ListBackups).Methods("GET")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups/{backup_id}/restores", api.RestoreBackup).Methods("POST")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/backups/{backup_id}/restores/{restore_id}", api.Endpoints).Methods("GET")
	router.HandleFunc("/custom/service_instances/{service_instance_id}/api-docs", api.APIDocs).Methods("GET")

	return &api
}

func (a API) respond(w http.ResponseWriter, status int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if response == nil {
		return
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logger.Error("encoding response", err, lager.Data{"status": status, "response": response})
	}
}

func (a API) handleAPIError(ctx context.Context, w http.ResponseWriter, err error) {
	var ae APIError
	if errors.As(err, &ae) {
		a.respond(w, ae.code, ae.err)
		return
	}
	err = crossplane.ConvertError(ctx, err)
	a.respond(w, http.StatusInternalServerError, apiresponses.ErrorResponse{Error: err.Error()})
}

func (a API) Endpoints(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]

	r, err := a.handler.Endpoints(req.Context(), instanceID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) ServiceUsage(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]

	r, err := a.handler.ServiceUsage(req.Context(), instanceID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) CreateUpdateServiceDefinition(w http.ResponseWriter, req *http.Request) {
	var sd ServiceDefinitionRequest
	err := json.NewDecoder(req.Body).Decode(&sd)
	if err != nil {
		a.handleAPIError(req.Context(), w, APIError{
			code: http.StatusBadRequest,
			err: apiresponses.ErrorResponse{
				Error: err.Error(),
			},
		})
		return
	}
	defer req.Body.Close()

	err = a.handler.CreateUpdateServiceDefinition(req.Context(), &sd)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
	}
	a.respond(w, http.StatusNoContent, nil)
}

func (a API) DeleteServiceDefinition(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]

	err := a.handler.DeleteServiceDefinition(req.Context(), id)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusNoContent, nil)
}

func (a API) CreateBackup(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]

	var br BackupRequest
	err := json.NewDecoder(req.Body).Decode(&br)
	if err != nil {
		a.handleAPIError(req.Context(), w, APIError{
			code: http.StatusBadRequest,
			err: apiresponses.ErrorResponse{
				Error: err.Error(),
			},
		})
		return
	}

	b, err := a.handler.CreateBackup(req.Context(), instanceID, &br)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
	}
	a.respond(w, http.StatusCreated, b)
}

func (a API) DeleteBackup(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]
	backupID := vars["backup_id"]

	r, err := a.handler.DeleteBackup(req.Context(), instanceID, backupID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) Backup(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]
	backupID := vars["backup_id"]

	r, err := a.handler.Backup(req.Context(), instanceID, backupID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) ListBackups(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]

	r, err := a.handler.ListBackups(req.Context(), instanceID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) RestoreBackup(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]
	backupID := vars["backup_id"]

	var restore RestoreRequest
	err := json.NewDecoder(req.Body).Decode(&restore)
	if err != nil {
		a.handleAPIError(req.Context(), w, APIError{
			code: http.StatusBadRequest,
			err: apiresponses.ErrorResponse{
				Error: err.Error(),
			},
		})
		return
	}

	r, err := a.handler.RestoreBackup(req.Context(), instanceID, backupID, &restore)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) RestoreStatus(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]
	backupID := vars["backup_id"]
	restoreID := vars["restore_id"]

	r, err := a.handler.RestoreStatus(req.Context(), instanceID, backupID, restoreID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}

func (a API) APIDocs(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	instanceID := vars["service_instance_id"]

	r, err := a.handler.APIDocs(req.Context(), instanceID)
	if err != nil {
		a.handleAPIError(req.Context(), w, err)
		return
	}
	a.respond(w, http.StatusOK, r)
}
