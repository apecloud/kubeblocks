package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

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
	DeletingPhase    Phase = "Deleting"
	CreatingPhase    Phase = "Creating"
	RunningPhase     Phase = "Running"
	FailedPhase      Phase = "Failed"
	UpdatingPhase    Phase = "Updating"
)

type Status string

// CR.Status.ClusterDefSyncStatus
const (
	OutOfSyncStatus Status = "OutOfSync"
	InSyncStatus    Status = "InSync"
)

var webhookMgr *webhookManager

type webhookManager struct {
	client client.Client
}

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}
