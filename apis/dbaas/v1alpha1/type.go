package v1alpha1

const (
	APIVersion            = "dbaas.infracreate.com/v1alpha1"
	AppVersionKind        = "AppVersion"
	ClusterDefinitionKind = "ClusterDefinition"
	ClusterKind           = "Cluster"
)

type Phase string

// CR.Status.Phase
const (
	AvailablePhase   Phase = "Available"
	UnAvailablePhase Phase = "UnAvailable"
)

type Status string

// CR.Status.ClusterDefSyncStatus
const (
	OutOfSyncStatus Status = "OutOfSync"
	InSyncStatus    Status = "InSync"
)

// label keys
const (
	AppVersionLabelKey = "appversion.infracreate.com/name"
	ClusterDefLabelKey = "clusterdefinition.infracreate.com/name"
)
