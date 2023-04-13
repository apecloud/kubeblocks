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
	// config keys used in viper, DON'T refactor the value without careful inspections
	CfgKeyServerInfo                    = "_KUBE_SERVER_INFO"
	CfgKeyCtrlrMgrNS                    = "CM_NAMESPACE"
	CfgKeyCtrlrMgrAffinity              = "CM_AFFINITY"
	CfgKeyCtrlrMgrNodeSelector          = "CM_NODE_SELECTOR"
	CfgKeyCtrlrMgrTolerations           = "CM_TOLERATIONS"
	CfgKeyCtrlrReconcileRetryDurationMS = "CM_RECON_RETRY_DURATION_MS" // accept time

	// addon config keys
	CfgKeyAddonJobTTL        = "ADDON_JOB_TTL"
	CfgAddonJobImgPullPolicy = "ADDON_JOB_IMAGE_PULL_POLICY"
)

const (
	ConnCredentialPlaceHolder    = "$(CONN_CREDENTIAL_SECRET_NAME)"
	KBCompNamePlaceHolder        = "$(KB_COMP_NAME)"
	KBClusterNamePlaceHolder     = "$(KB_CLUSTER_NAME)"
	KBClusterCompNamePlaceHolder = "$(KB_CLUSTER_COMP_NAME)"
)

const (
	KBPrefix = "KB"
)

const (
	KBToolsImage      = "KUBEBLOCKS_TOOLS_IMAGE"
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
	KBAppComponentDefRefLabelKey    = "apps.kubeblocks.io/component-def-ref"
	ConsensusSetAccessModeLabelKey  = "cs.apps.kubeblocks.io/access-mode"
	AppConfigTypeLabelKey           = "apps.kubeblocks.io/config-type"
	WorkloadTypeLabelKey            = "apps.kubeblocks.io/workload-type"
	VolumeClaimTemplateNameLabelKey = "vct.kubeblocks.io/name"
	RoleLabelKey                    = "kubeblocks.io/role"              // RoleLabelKey consensusSet and replicationSet role label key
	BackupProtectionLabelKey        = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	AddonNameLabelKey               = "extensions.kubeblocks.io/addon-name"
	ClusterAccountLabelKey          = "account.kubeblocks.io/name"
	VolumeTypeLabelKey              = "kubeblocks.io/volume-type"
	KBManagedByKey                  = "apps.kubeblocks.io/managed-by" // KBManagedByKey marks resources that auto created during operation
	ClassProviderLabelKey           = "class.kubeblocks.io/provider"

	// kubeblocks.io annotations
	OpsRequestAnnotationKey           = "kubeblocks.io/ops-request" // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	ReconcileAnnotationKey            = "kubeblocks.io/reconcile"   // ReconcileAnnotationKey Notify k8s object to reconcile
	RestartAnnotationKey              = "kubeblocks.io/restart"     // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	SnapShotForStartAnnotationKey     = "kubeblocks.io/snapshot-for-start"
	RestoreFromBackUpAnnotationKey    = "kubeblocks.io/restore-from-backup" // RestoreFromBackUpAnnotationKey specifies the component to recover from the backup.
	ClusterSnapshotAnnotationKey      = "kubeblocks.io/cluster-snapshot"    // ClusterSnapshotAnnotationKey saves the snapshot of cluster.
	LeaderAnnotationKey               = "cs.apps.kubeblocks.io/leader"
	DefaultBackupPolicyAnnotationKey  = "dataprotection.kubeblocks.io/is-default-policy"
	BackupPolicyTemplateAnnotationKey = "apps.kubeblocks.io/backup-policy-template"

	// ConfigurationTplLabelPrefixKey clusterVersion or clusterdefinition using tpl
	ConfigurationTplLabelPrefixKey         = "config.kubeblocks.io/tpl"
	ConfigurationConstraintsLabelPrefixKey = "config.kubeblocks.io/constraints"

	LastAppliedOpsCRAnnotation                  = "config.kubeblocks.io/last-applied-ops-name"
	LastAppliedConfigAnnotation                 = "config.kubeblocks.io/last-applied-configuration"
	DisableUpgradeInsConfigurationAnnotationKey = "config.kubeblocks.io/disable-reconfigure"
	UpgradePolicyAnnotationKey                  = "config.kubeblocks.io/reconfigure-policy"
	UpgradeRestartAnnotationKey                 = "config.kubeblocks.io/restart"
	KBParameterUpdateSourceAnnotationKey        = "config.kubeblocks.io/reconfigure-source"

	// CMConfigurationTypeLabelKey configmap is config template type, e.g: "tpl", "instance"
	CMConfigurationTypeLabelKey            = "config.kubeblocks.io/config-type"
	CMConfigurationTemplateNameLabelKey    = "config.kubeblocks.io/config-template-name"
	CMConfigurationConstraintsNameLabelKey = "config.kubeblocks.io/config-constraints-name"
	CMInsConfigurationHashLabelKey         = "config.kubeblocks.io/config-hash"

	// CMImportedConfigTemplateLabelKey configmap is imported config template
	CMImportedConfigTemplateLabelKey = "config.kubeblocks.io/imported-template-signature"

	// CMConfigurationSpecProviderLabelKey is ComponentConfigSpec name
	CMConfigurationSpecProviderLabelKey = "config.kubeblocks.io/config-spec"

	// CMConfigurationCMKeysLabelKey Specify keys
	CMConfigurationCMKeysLabelKey = "config.kubeblocks.io/configmap-keys"

	// CMInsConfigurationLabelKey configmap is configuration file for component
	// CMInsConfigurationLabelKey = "config.kubeblocks.io/ins-configure"

	// CMInsLastReconfigurePhaseKey defines the current phase
	CMInsLastReconfigurePhaseKey = "config.kubeblocks.io/last-applied-reconfigure-phase"

	// CMInsEnableRerenderTemplateKey is used to enable rerender template
	CMInsEnableRerenderTemplateKey = "config.kubeblocks.io/enable-rerender"

	// configuration finalizer
	ConfigurationTemplateFinalizerName = "config.kubeblocks.io/finalizer"

	// ClassAnnotationKey is used to specify the class of components
	ClassAnnotationKey = "cluster.kubeblocks.io/component-class"
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
	CronJobKind               = "CronJob"
	JobKind                   = "Job"
	ReplicaSetKind            = "ReplicaSetKind"
	VolumeSnapshotKind        = "VolumeSnapshot"
	ServiceKind               = "Service"
	ConfigMapKind             = "ConfigMap"
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
	ProbeHTTPPortName         = "probe-http-port"
	ProbeGRPCPortName         = "probe-grpc-port"
	RoleProbeContainerName    = "kb-checkrole"
	StatusProbeContainerName  = "kb-checkstatus"
	RunningProbeContainerName = "kb-checkrunning"

	// the filedpath name used in event.InvolvedObject.FieldPath
	ProbeCheckRolePath    = "spec.containers{" + RoleProbeContainerName + "}"
	ProbeCheckStatusPath  = "spec.containers{" + StatusProbeContainerName + "}"
	ProbeCheckRunningPath = "spec.containers{" + RunningProbeContainerName + "}"

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
	ConfigSidecarName        = "config-manager"
	ConfigManagerGPRCPortEnv = "CONFIG_MANAGER_GRPC_PORT"
	ConfigManagerLogLevel    = "CONFIG_MANAGER_LOG_LEVEL"

	PodMinReadySecondsEnv = "POD_MIN_READY_SECONDS"
	ConfigTemplateType    = "tpl"
	ConfigInstanceType    = "instance"

	ReconfigureManagerSource  = "manager"
	ReconfigureUserSource     = "ops"
	ReconfigureTemplateSource = "external-template"
)

const (
	KBReplicationSetPrimaryPodName = "KB_PRIMARY_POD_NAME"
)

// username and password are keys in created secrets for others to refer to.
const (
	AccountNameForSecret   = "username"
	AccountPasswdForSecret = "password"
)
