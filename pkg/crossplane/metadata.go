package crossplane

const (
	// SynToolsBase is the base domain
	SynToolsBase = "service.syn.tools"

	// DescriptionAnnotation of the instance
	DescriptionAnnotation = SynToolsBase + "/description"
	// MetadataAnnotation of the instance
	MetadataAnnotation = SynToolsBase + "/metadata"
	// DeletionTimestampAnnotation marks when an object got deleted
	DeletionTimestampAnnotation = SynToolsBase + "/deletionTimestamp"
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
)
