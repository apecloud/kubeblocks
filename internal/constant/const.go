/*
Copyright ApeCloud, Inc.

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

package constant

const (
	// config keys used in viper
	CfgKeyServerInfo = "_KUBE_SERVER_INFO"
)

const (
	ConnCredentialPlaceHolder = "$(CONN_CREDENTIAL_SECRET_NAME)"
)

const (
	KBPrefix = "KB"
)

const KBImage = "KUBEBLOCKS_IMAGE"

const (
	KBImagePullPolicy = "KUBEBLOCKS_IMAGE_PULL_POLICY"
)

const (
	APIGroup = "kubeblocks.io"

	AppName = "kubeblocks"

	// K8s recommonded and well-known label and annotation keys
	AppInstanceLabelKey  = "app.kubernetes.io/instance"
	AppNameLabelKey      = "app.kubernetes.io/name"
	AppManagedByLabelKey = "app.kubernetes.io/managed-by"
	RegionLabelKey       = "topology.kubernetes.io/region"
	ZoneLabelKey         = "topology.kubernetes.io/zone"

	// kubeblocks.io labels
	ClusterDefLabelKey              = "clusterdefinition.kubeblocks.io/name"
	KBAppComponentLabelKey          = "apps.kubeblocks.io/component-name"
	ConsensusSetAccessModeLabelKey  = "cs.apps.kubeblocks.io/access-mode"
	AppConfigTypeLabelKey           = "apps.kubeblocks.io/config-type"
	WorkloadTypeLabelKey            = "apps.kubeblocks.io/workload-type"
	VolumeClaimTemplateNameLabelKey = "vct.kubeblocks.io/name"
	RoleLabelKey                    = "kubeblocks.io/role"              // RoleLabelKey consensusSet and replicationSet role label key
	BackupProtectionLabelKey        = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	AddonNameLabelKey               = "extensions.kubeblocks.io/addon-name"
	ClusterAccountLabelKey          = "account.kubeblocks.io/name"
	VolumeTypeLabelKey              = "kubeblocks.io/volume-type"

	// kubeblocks.io annotations
	OpsRequestAnnotationKey          = "kubeblocks.io/ops-request" // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	OpsRequestReconcileAnnotationKey = "kubeblocks.io/reconcile"   // OpsRequestReconcileAnnotationKey Notify OpsRequest to reconcile
	RestartAnnotationKey             = "kubeblocks.io/restart"     // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	SnapShotForStartAnnotationKey    = "kubeblocks.io/snapshot-for-start"
	RestoreFromBackUpAnnotationKey   = "kubeblocks.io/restore-from-backup" // RestoreFromBackUpAnnotationKey specifies the component to recover from the backup.
	ClusterSnapshotAnnotationKey     = "kubeblocks.io/cluster-snapshot"    // ClusterSnapshotAnnotationKey saves the snapshot of cluster.

	// ConfigurationTplLabelPrefixKey clusterVersion or clusterdefinition using tpl
	ConfigurationTplLabelPrefixKey         = "configuration.kubeblocks.io/cfg-tpl"
	ConfigurationConstraintsLabelPrefixKey = "configuration.kubeblocks.io/cfg-constraints"

	LastAppliedOpsCRAnnotation                  = "configuration.kubeblocks.io/last-applied-ops-name"
	LastAppliedConfigAnnotation                 = "configuration.kubeblocks.io/last-applied-configuration"
	DisableUpgradeInsConfigurationAnnotationKey = "configuration.kubeblocks.io/disable-reconfigure"
	UpgradePolicyAnnotationKey                  = "configuration.kubeblocks.io/reconfigure-policy"
	UpgradeRestartAnnotationKey                 = "configuration.kubeblocks.io/restart"

	// CMConfigurationTypeLabelKey configmap is config template type, e.g: "tpl", "instance"
	CMConfigurationTypeLabelKey            = "configuration.kubeblocks.io/configuration-type"
	CMConfigurationTplNameLabelKey         = "configuration.kubeblocks.io/configuration-tpl-name"
	CMConfigurationConstraintsNameLabelKey = "configuration.kubeblocks.io/configuration-constraints-name"
	CMInsConfigurationHashLabelKey         = "configuration.kubeblocks.io/configuration-hash"
	CMConfigurationProviderTplLabelKey     = "configuration.kubeblocks.io/configtemplate-name"

	// CMConfigurationCMKeysLabelKey Specify keys
	CMConfigurationCMKeysLabelKey = "configuration.kubeblocks.io/configmap-keys"

	// CMInsConfigurationLabelKey configmap is configuration file for component
	// CMInsConfigurationLabelKey = "configuration.kubeblocks.io/ins-configure"

	CMInsLastReconfigureMethodLabelKey = "configuration.kubeblocks.io/last-applied-reconfigure-policy"

	// configuration finalizer
	ConfigurationTemplateFinalizerName = "configuration.kubeblocks.io/finalizer"
)

const (
	// ReasonNotFoundCR referenced custom resource not found
	ReasonNotFoundCR = "NotFound"
	// ReasonRefCRUnavailable  referenced custom resource is unavailable
	ReasonRefCRUnavailable = "Unavailable"
	// ReasonDeletedCR deleted custom resource
	ReasonDeletedCR = "DeletedCR"
	// ReasonDeletingCR deleting custom resource
	ReasonDeletingCR = "DeletingCR"
	// ReasonCreatedCR created custom resource
	ReasonCreatedCR = "CreatedCR"
	// ReasonRunTaskFailed run task failed
	ReasonRunTaskFailed = "RunTaskFailed"
	// ReasonDeleteFailed delete failed
	ReasonDeleteFailed = "DeleteFailed"
)

const (
	DeploymentKind            = "Deployment"
	StatefulSetKind           = "StatefulSet"
	PodKind                   = "Pod"
	PersistentVolumeClaimKind = "PersistentVolumeClaim"
	CronJob                   = "CronJob"
	ReplicaSet                = "ReplicaSet"
	VolumeSnapshotKind        = "VolumeSnapshot"
	ServiceKind               = "Service"
)

const (
	// BackupRetain always retained, unless manually deleted by the user
	BackupRetain = "Retain"

	// BackupRetainUntilExpired retains backup till it expires
	BackupRetainUntilExpired = "RetainUntilExpired"

	// BackupDelete (default) deletes backup immediately when cluster's terminationPolicy is WipeOut
	BackupDelete = "Delete"
)

const (
	// Container port name
	ProbeHTTPPortName = "probe-http-port"
	ProbeGRPCPortName = "probe-grpc-port"

	// KubeBlocksDataNodeLabelKey is the node label key of the built-in data node label
	KubeBlocksDataNodeLabelKey = "kb-data"
	// KubeBlocksDataNodeLabelValue is the node label value of the built-in data node label
	KubeBlocksDataNodeLabelValue = "true"
	// KubeBlocksDataNodeTolerationKey is the taint label key of the built-in data node taint
	KubeBlocksDataNodeTolerationKey = "kb-data"
	// KubeBlocksDataNodeTolerationValue is the taint label value of the built-in data node taint
	KubeBlocksDataNodeTolerationValue = "true"
)

const (

	// ConfigSidecarIMAGE for config manager sidecar
	ConfigSidecarIMAGE       = "KUBEBLOCKS_IMAGE"
	ConfigSidecarName        = "config-manager"
	ConfigManagerGPRCPortEnv = "CONFIG_MANAGER_GRPC_PORT"

	PodMinReadySecondsEnv = "POD_MIN_READY_SECONDS"
	ConfigTemplateType    = "tpl"
	ConfigInstanceType    = "instance"
)

const (
	KBReplicationSetPrimaryPodName = "KB_PRIMARY_POD_NAME"
)
