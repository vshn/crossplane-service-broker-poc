package custom

import (
	"context"
	"time"
)

// CustomAPI describes the service broker endpoints not defined by the open service broker API spec.
type CustomAPI interface {
	// Endpoints lists service endpoints
	// GET /custom/service_instances/{service_instance_id}/endpoint
	Endpoints(ctx context.Context, instanceID string) ([]Endpoint, error)
	// Usage returns service usage
	// GET /custom/service_instances/{service_instance_id}/usage
	ServiceUsage(ctx context.Context, instanceID string) (*ServiceUsage, error)
	// CreateUpdateServiceDefinition
	// POST /custom/admin/service-definition
	CreateUpdateServiceDefinition(ctx context.Context, sd *ServiceDefinitionRequest) error
	// DeleteServiceDefinition
	// DELETE /custom/admin/service-definition/{id}
	DeleteServiceDefinition(ctx context.Context, id string) error
	// CreateBackup
	// POST /custom/service_instances/{service_instance_id}/backups
	CreateBackup(ctx context.Context, instanceID string, b *BackupRequest) (*Backup, error)
	// DeleteBackup
	// DELETE /custom/service_instances/{service_instance_id}/backups/{backup_id}
	DeleteBackup(ctx context.Context, instanceID, backupID string) (string, error)
	// Backup
	// GET /custom/service_instances/{service_instance_id}/backups/{backup_id}
	Backup(ctx context.Context, instanceID, backupID string) (*Backup, error)
	// ListBackups
	// GET /custom/service_instances/{service_instance_id}/backups
	ListBackups(ctx context.Context, instanceID string) ([]Backup, error)
	// RestoreBackup
	// POST /custom/service_instances/{service_instance_id}/backups/{backup_id}/restores
	RestoreBackup(ctx context.Context, instanceID, backupID string, r *RestoreRequest) (*Restore, error)
	// RestoreStatus
	// GET /custom/service_instances/{service_instance_id}/backups/{backup_id}/restores/{restore_id}
	RestoreStatus(ctx context.Context, instanceID, backupID, restoreID string) (*Restore, error)
	// APIDocs
	// GET /custom/service_instances/{service_instance_id}/api-docs
	APIDocs(ctx context.Context, instanceID string) (string, error)
}

// Endpoint describes available service endpoints.
type Endpoint struct {
	Destination string `json:"destination"`
	Ports       string `json:"ports"`
	Protocol    string `json:"protocol"`
}

type UsageUnit string
type UsageType string

const (
	GigabyteSecond UsageUnit = "GB-s"
	MegabyteSecond UsageUnit = "MB-s"

	Transactions UsageType = "transactions"
	Watermark    UsageType = "watermark"
)

type ServiceUsage struct {
	Value   string    `json:"value"`
	Unit    UsageUnit `json:"unit"`
	Type    UsageType `json:"type"`
	EndDate time.Time `json:"end_date"`
}

type BackupStatus string
type RestoreStatus string

const (
	CreateInProgress BackupStatus = "CREATE_IN_PROGRESS"
	CreateSucceeded  BackupStatus = "CREATE_SUCCEEDED"
	CreateFailed     BackupStatus = "CREATE_FAILED"
	DeleteInProgress BackupStatus = "DELETE_IN_PROGRESS"
	DeleteSucceeded  BackupStatus = "DELETE_SUCCEEDED"
	DeleteFailed     BackupStatus = "DELETE_FAILED"

	RestoreInProgress RestoreStatus = "IN_PROGRESS"
	RestoreSucceeded  RestoreStatus = "SUCCEEDED"
	RestoreFailed     RestoreStatus = "FAILED"
)

type Backup struct {
	ID                string       `json:"id"`
	ServiceInstanceID string       `json:"service_instance_id"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	Status            BackupStatus `json:"status"`
	Restores          []Restore    `json:"restores"`
}
type Restore struct {
	ID        string        `json:"id"`
	BackupID  string        `json:"backup_id"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Status    RestoreStatus `json:"status"`
}

type ServiceDefinitionRequest struct{}
type BackupRequest struct{}
type RestoreRequest struct{}
