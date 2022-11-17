/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	APIVersion            = "dbaas.kubeblocks.io/v1alpha1"
	AppVersionKind        = "AppVersion"
	ClusterDefinitionKind = "ClusterDefinition"
	ClusterKind           = "Cluster"
	OpsRequestKind        = "OpsRequestKind"
)

// Phase defines the CR .Status.Phase
// +enum
type Phase string

const (
	AvailablePhase   Phase = "Available"
	UnavailablePhase Phase = "Unavailable"
	DeletingPhase    Phase = "Deleting"
	CreatingPhase    Phase = "Creating"
	PendingPhase     Phase = "Pending"
	RunningPhase     Phase = "Running"
	FailedPhase      Phase = "Failed"
	UpdatingPhase    Phase = "Updating"
	SucceedPhase     Phase = "Succeed"
)

// Status define CR .Status.ClusterDefSyncStatus
// +enum
type Status string

const (
	OutOfSyncStatus Status = "OutOfSync"
	InSyncStatus    Status = "InSync"
)

// OpsType defines operation types.
// +enum
type OpsType string

const (
	VerticalScalingType   OpsType = "VerticalScaling"
	HorizontalScalingType OpsType = "HorizontalScaling"
	VolumeExpansionType   OpsType = "VolumeExpansion"
	UpgradeType           OpsType = "Upgrade"
	RestartType           OpsType = "Restart"
)

var DefaultLeader = ConsensusMember{
	Name:       "leader",
	AccessMode: ReadWrite,
}

var webhookMgr *webhookManager

type webhookManager struct {
	client client.Client
}

type ScopeType string

const (
	ScopeBothType   ScopeType = "ScopeBoth"
	ScopeFileType   ScopeType = "ScopeFile"
	ScopeMemoryType ScopeType = "ScopeMemory"
)

type ConfigurationFormatter string

const (
	INI    ConfigurationFormatter = "ini"
	YAML   ConfigurationFormatter = "yaml"
	JSON   ConfigurationFormatter = "json"
	XML    ConfigurationFormatter = "xml"
	HCL    ConfigurationFormatter = "hcl"
	DOTENV ConfigurationFormatter = "dotenv"
)

type UpgradePolicy string

const (
	NormalPolicy  UpgradePolicy = "simple"
	RestartPolicy UpgradePolicy = "parallel"
	RollingPolicy UpgradePolicy = "rolling"
	AutoReload    UpgradePolicy = "autoReload"
)

type CfgReloadType string

const (
	UnixSignalType CfgReloadType = "signal"
	SqlType        CfgReloadType = "sql"
	ShellType      CfgReloadType = "exec"
	HttpType       CfgReloadType = "http"
)

func RegisterWebhookManager(mgr manager.Manager) {
	webhookMgr = &webhookManager{mgr.GetClient()}
}
