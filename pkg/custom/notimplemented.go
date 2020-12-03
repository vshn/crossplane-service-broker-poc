package custom

import (
	"context"
	"net/http"

	"github.com/pivotal-cf/brokerapi/domain/apiresponses"
)

var notImplemented = APIError{
	code: http.StatusNotImplemented,
	err: apiresponses.ErrorResponse{
		Error:       "API not implemented",
		Description: "API not implemented",
	},
}

func (h APIHandler) ServiceUsage(ctx context.Context, instanceID string) (*ServiceUsage, error) {
	return nil, notImplemented
}
func (h APIHandler) CreateUpdateServiceDefinition(ctx context.Context, sd *ServiceDefinitionRequest) error {
	return notImplemented
}
func (h APIHandler) DeleteServiceDefinition(ctx context.Context, id string) error {
	return notImplemented
}
func (h APIHandler) CreateBackup(ctx context.Context, instanceID string, b *BackupRequest) (*Backup, error) {
	return nil, notImplemented
}
func (h APIHandler) DeleteBackup(ctx context.Context, instanceID, backupID string) (string, error) {
	return "", notImplemented
}
func (h APIHandler) Backup(ctx context.Context, instanceID, backupID string) (*Backup, error) {
	return nil, notImplemented
}
func (h APIHandler) ListBackups(ctx context.Context, instanceID string) ([]Backup, error) {
	return nil, notImplemented
}
func (h APIHandler) RestoreBackup(ctx context.Context, instanceID, backupID string, r *RestoreRequest) (*Restore, error) {
	return nil, notImplemented
}
func (h APIHandler) RestoreStatus(ctx context.Context, instanceID, backupID, restoreID string) (*Restore, error) {
	return nil, notImplemented
}
func (h APIHandler) APIDocs(ctx context.Context, instanceID string) (string, error) {
	return "", notImplemented
}
