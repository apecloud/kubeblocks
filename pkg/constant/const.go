/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package constant

const (
	// config keys used in viper, DON'T refactor the value without careful inspections
	CfgKeyServerInfo                    = "_KUBE_SERVER_INFO"
	CfgKeyCtrlrMgrNS                    = "CM_NAMESPACE"
	CfgKeyCtrlrMgrAffinity              = "CM_AFFINITY"
	CfgKeyCtrlrMgrNodeSelector          = "CM_NODE_SELECTOR"
	CfgKeyCtrlrMgrTolerations           = "CM_TOLERATIONS"
	CfgKeyCtrlrReconcileRetryDurationMS = "CM_RECON_RETRY_DURATION_MS"       // accept time
	CfgRecoverVolumeExpansionFailure    = "RECOVER_VOLUME_EXPANSION_FAILURE" // refer to feature gates RecoverVolumeExpansionFailure of k8s.
	CfgKeyProvider                      = "KUBE_PROVIDER"
	CfgHostPortConfigMapName            = "HOST_PORT_CM_NAME"
	CfgHostPortIncludeRanges            = "HOST_PORT_INCLUDE_RANGES"
	CfgHostPortExcludeRanges            = "HOST_PORT_EXCLUDE_RANGES"

	// addon config keys
	CfgKeyAddonJobTTL        = "ADDON_JOB_TTL"
	CfgAddonJobImgPullPolicy = "ADDON_JOB_IMAGE_PULL_POLICY"

	// data plane config key
	CfgKeyDataPlaneTolerations = "DATA_PLANE_TOLERATIONS"
	CfgKeyDataPlaneAffinity    = "DATA_PLANE_AFFINITY"

	// storage config keys
	CfgKeyDefaultStorageClass = "DEFAULT_STORAGE_CLASS"

	// customized encryption key for encrypting the password of connection credential.
	CfgKeyDPEncryptionKey                = "DP_ENCRYPTION_KEY"
	CfgKeyDPBackupEncryptionSecretKeyRef = "DP_BACKUP_ENCRYPTION_SECRET_KEY_REF"
	CfgKeyDPBackupEncryptionAlgorithm    = "DP_BACKUP_ENCRYPTION_ALGORITHM"

	CfgKBReconcileWorkers = "KUBEBLOCKS_RECONCILE_WORKERS"
	CfgClientQPS          = "CLIENT_QPS"
	CfgClientBurst        = "CLIENT_BURST"
)

const (
	// TODO: deprecated, will be removed later.
	KBConnCredentialPlaceHolder = "$(CONN_CREDENTIAL_SECRET_NAME)"
	KBComponentEnvCMPlaceHolder = "$(COMP_ENV_CM_NAME)"
	KBToolsImagePlaceHolder     = "$(KUBEBLOCKS_TOOLS_IMAGE)"
)

const (
	KBPrefix      = "KB"
	KBLowerPrefix = "kb"

	SlashScalingLowerSuffix = "scaling"
)

const (
	KBServiceAccountName     = "KUBEBLOCKS_SERVICEACCOUNT_NAME"
	KBToolsImage             = "KUBEBLOCKS_TOOLS_IMAGE"
	KBImagePullPolicy        = "KUBEBLOCKS_IMAGE_PULL_POLICY"
	KBDataScriptClientsImage = "KUBEBLOCKS_DATASCRIPT_CLIENTS_IMAGE"
)

const (
	APIGroup = "kubeblocks.io"

	AppName = "kubeblocks"

	// K8S recommended well-known labels and annotation keys

	// AppInstanceLabelKey refer cluster.Name
	AppInstanceLabelKey = "app.kubernetes.io/instance"
	// AppNameLabelKey refer clusterDefinition.Name before KubeBlocks Version 0.8.0 or refer ComponentDefinition.Name after KubeBlocks Version 0.8.0 (TODO：Pending)
	AppNameLabelKey = "app.kubernetes.io/name"
	// AppComponentLabelKey refer clusterDefinition.Spec.ComponentDefs[*].Name before KubeBlocks Version 0.8.0 or refer ComponentDefinition.Name after KubeBlocks Version 0.8.0
	AppComponentLabelKey = "app.kubernetes.io/component"
	// AppVersionLabelKey refer clusterVersion.Name before KubeBlocks Version 0.8.0 or refer ComponentDefinition.Name after KubeBlocks Version 0.8.0
	AppVersionLabelKey   = "app.kubernetes.io/version"
	AppManagedByLabelKey = "app.kubernetes.io/managed-by"
	RegionLabelKey       = "topology.kubernetes.io/region"
	ZoneLabelKey         = "topology.kubernetes.io/zone"

	// kubeblocks.io labels
	BackupProtectionLabelKey                 = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	RoleLabelKey                             = "kubeblocks.io/role"              // RoleLabelKey consensusSet and replicationSet role label key
	AccessModeLabelKey                       = "workloads.kubeblocks.io/access-mode"
	ReadyWithoutPrimaryKey                   = "kubeblocks.io/ready-without-primary"
	VolumeTypeLabelKey                       = "kubeblocks.io/volume-type"
	ClusterAccountLabelKey                   = "account.kubeblocks.io/name"
	KBAppClusterUIDLabelKey                  = "apps.kubeblocks.io/cluster-uid"
	KBAppComponentLabelKey                   = "apps.kubeblocks.io/component-name"
	KBAppShardingNameLabelKey                = "apps.kubeblocks.io/sharding-name"
	KBAppComponentDefRefLabelKey             = "apps.kubeblocks.io/component-def-ref" // refer clusterDefinition.Spec.ComponentDefs[*].Name before KubeBlocks Version 0.8.0 or refer ComponentDefinition.Name after KubeBlocks Version 0.8.0
	KBAppClusterDefTypeLabelKey              = "apps.kubeblocks.io/cluster-type"      // refer clusterDefinition.Spec.Type (deprecated)
	KBManagedByKey                           = "apps.kubeblocks.io/managed-by"        // KBManagedByKey marks resources that auto created
	PVCNameLabelKey                          = "apps.kubeblocks.io/pvc-name"
	VolumeClaimTemplateNameLabelKey          = "apps.kubeblocks.io/vct-name"
	KBAppComponentInstanceTemplatelabelKey   = "apps.kubeblocks.io/instance-template"
	KBAppServiceVersionKey                   = "apps.kubeblocks.io/service-version"
	WorkloadTypeLabelKey                     = "apps.kubeblocks.io/workload-type"
	KBAppPodNameLabelKey                     = "apps.kubeblocks.io/pod-name"
	ClassProviderLabelKey                    = "class.kubeblocks.io/provider"
	ClusterDefLabelKey                       = "clusterdefinition.kubeblocks.io/name"
	ClusterVerLabelKey                       = "clusterversion.kubeblocks.io/name"
	ComponentDefinitionLabelKey              = "componentdefinition.kubeblocks.io/name"
	ComponentVersionLabelKey                 = "componentversion.kubeblocks.io/name"
	CMConfigurationSpecProviderLabelKey      = "config.kubeblocks.io/config-spec"    // CMConfigurationSpecProviderLabelKey is ComponentConfigSpec name
	CMConfigurationCMKeysLabelKey            = "config.kubeblocks.io/configmap-keys" // CMConfigurationCMKeysLabelKey Specify configmap keys
	CMConfigurationTemplateNameLabelKey      = "config.kubeblocks.io/config-template-name"
	CMTemplateNameLabelKey                   = "config.kubeblocks.io/template-name"
	CMConfigurationTypeLabelKey              = "config.kubeblocks.io/config-type"
	CMInsConfigurationHashLabelKey           = "config.kubeblocks.io/config-hash"
	CMInsCurrentConfigurationHashLabelKey    = "config.kubeblocks.io/update-config-hash"
	CMConfigurationConstraintsNameLabelKey   = "config.kubeblocks.io/config-constraints-name"
	CMConfigurationTemplateVersion           = "config.kubeblocks.io/config-template-version"
	ConsensusSetAccessModeLabelKey           = "cs.apps.kubeblocks.io/access-mode"
	AddonNameLabelKey                        = "extensions.kubeblocks.io/addon-name"
	OpsRequestTypeLabelKey                   = "ops.kubeblocks.io/ops-type"
	OpsRequestNameLabelKey                   = "ops.kubeblocks.io/ops-name"
	OpsRequestNamespaceLabelKey              = "ops.kubeblocks.io/ops-namespace"
	ServiceDescriptorNameLabelKey            = "servicedescriptor.kubeblocks.io/name"
	RestoreForHScaleLabelKey                 = "apps.kubeblocks.io/restore-for-hscale"
	ResourceConstraintProviderLabelKey       = "resourceconstraint.kubeblocks.io/provider"
	VolumeClaimTemplateNameLabelKeyForLegacy = "vct.kubeblocks.io/name" // Deprecated: only compatible with version 0.5, will be removed in 0.7

	// kubeblocks.io annotations
	ClusterSnapshotAnnotationKey                = "kubeblocks.io/cluster-snapshot" // ClusterSnapshotAnnotationKey saves the snapshot of cluster.
	OpsRequestAnnotationKey                     = "kubeblocks.io/ops-request"      // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	ReconcileAnnotationKey                      = "kubeblocks.io/reconcile"        // ReconcileAnnotationKey Notify k8s object to reconcile
	RestartAnnotationKey                        = "kubeblocks.io/restart"          // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	RestoreFromBackupAnnotationKey              = "kubeblocks.io/restore-from-backup"
	RestoreDoneAnnotationKey                    = "kubeblocks.io/restore-done"
	BackupSourceTargetAnnotationKey             = "kubeblocks.io/backup-source-target" // RestoreFromBackupAnnotationKey specifies the component to recover from the backup.
	SnapShotForStartAnnotationKey               = "kubeblocks.io/snapshot-for-start"
	ComponentReplicasAnnotationKey              = "apps.kubeblocks.io/component-replicas" // ComponentReplicasAnnotationKey specifies the number of pods in replicas
	BackupPolicyTemplateAnnotationKey           = "apps.kubeblocks.io/backup-policy-template"
	LastAppliedClusterAnnotationKey             = "apps.kubeblocks.io/last-applied-cluster"
	PVLastClaimPolicyAnnotationKey              = "apps.kubeblocks.io/pv-last-claim-policy"
	HaltRecoveryAllowInconsistentCVAnnotKey     = "clusters.apps.kubeblocks.io/allow-inconsistent-cv"
	HaltRecoveryAllowInconsistentResAnnotKey    = "clusters.apps.kubeblocks.io/allow-inconsistent-resource"
	PrimaryAnnotationKey                        = "rs.apps.kubeblocks.io/primary"
	DisableUpgradeInsConfigurationAnnotationKey = "config.kubeblocks.io/disable-reconfigure"
	LastAppliedConfigAnnotationKey              = "config.kubeblocks.io/last-applied-configuration"
	LastAppliedOpsCRAnnotationKey               = "config.kubeblocks.io/last-applied-ops-name"
	UpgradePolicyAnnotationKey                  = "config.kubeblocks.io/reconfigure-policy"
	KBParameterUpdateSourceAnnotationKey        = "config.kubeblocks.io/reconfigure-source"
	UpgradeRestartAnnotationKey                 = "config.kubeblocks.io/restart"
	ConfigAppliedVersionAnnotationKey           = "config.kubeblocks.io/config-applied-version"
	KubeBlocksGenerationKey                     = "kubeblocks.io/generation"
	ExtraEnvAnnotationKey                       = "kubeblocks.io/extra-env"
	LastRoleSnapshotVersionAnnotationKey        = "apps.kubeblocks.io/last-role-snapshot-version"
	ComponentScaleInAnnotationKey               = "apps.kubeblocks.io/component-scale-in" // ComponentScaleInAnnotationKey specifies whether the component is scaled in
	DisableHAAnnotationKey                      = "kubeblocks.io/disable-ha"
	// kubeblocks.io well-known finalizers
	DBClusterFinalizerName         = "cluster.kubeblocks.io/finalizer"
	DBComponentFinalizerName       = "component.kubeblocks.io/finalizer"
	ConfigFinalizerName            = "config.kubeblocks.io/finalizer"
	ServiceDescriptorFinalizerName = "servicedescriptor.kubeblocks.io/finalizer"
	OpsRequestFinalizerName        = "opsrequest.kubeblocks.io/finalizer"

	// ConfigurationTplLabelPrefixKey clusterVersion or clusterdefinition using tpl
	ConfigurationTplLabelPrefixKey         = "config.kubeblocks.io/tpl"
	ConfigurationConstraintsLabelPrefixKey = "config.kubeblocks.io/constraints"

	// CMInsLastReconfigurePhaseKey defines the current phase
	CMInsLastReconfigurePhaseKey = "config.kubeblocks.io/last-applied-reconfigure-phase"

	// ConfigurationRevision defines the current revision
	// TODO support multi version
	ConfigurationRevision          = "config.kubeblocks.io/configuration-revision"
	LastConfigurationRevisionPhase = "config.kubeblocks.io/revision-reconcile-phase"

	// Deprecated: only compatible with version 0.6, will be removed in 0.8
	// CMInsEnableRerenderTemplateKey is used to enable rerender template
	CMInsEnableRerenderTemplateKey = "config.kubeblocks.io/enable-rerender"

	// IgnoreResourceConstraint is used to specify whether to ignore the resource constraint
	IgnoreResourceConstraint = "resource.kubeblocks.io/ignore-constraint"

	RBACRoleName        = "kubeblocks-cluster-pod-role"
	RBACClusterRoleName = "kubeblocks-volume-protection-pod-role"

	// FeatureReconciliationInCompactModeAnnotationKey indicates that the controller should run in compact mode,
	// means to try the best to cutoff useless objects.
	FeatureReconciliationInCompactModeAnnotationKey = "kubeblocks.io/compact-mode"
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
	ReplicaSetKind            = "ReplicaSet"
	VolumeSnapshotKind        = "VolumeSnapshot"
	ServiceKind               = "Service"
	ConfigMapKind             = "ConfigMap"
	DaemonSetKind             = "DaemonSet"
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
	LorryHTTPPortName                  = "lorry-http-port"
	LorryGRPCPortName                  = "lorry-grpc-port"
	LorryContainerName                 = "lorry"
	LorryInitContainerName             = "init-lorry"
	ProbeInitContainerName             = "kb-initprobe"
	RoleProbeContainerName             = "kb-checkrole"
	StatusProbeContainerName           = "kb-checkstatus"
	RunningProbeContainerName          = "kb-checkrunning"
	VolumeProtectionProbeContainerName = "kb-volume-protection"
	LorryRoleProbePath                 = "/v1.0/checkrole"
	LorryVolumeProtectPath             = "/v1.0/volumeprotection"

	// the filedpath name used in event.InvolvedObject.FieldPath
	ProbeCheckStatusPath  = "spec.containers{" + StatusProbeContainerName + "}"
	ProbeCheckRunningPath = "spec.containers{" + RunningProbeContainerName + "}"
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

	ConfigManagerPortName = "config-manager"
)

const (
	Primary   = "primary"
	Secondary = "secondary"

	Leader    = "leader"
	Follower  = "follower"
	Learner   = "learner"
	Candidate = "candidate"
)

// username and password are keys in created secrets for others to refer to.
const (
	AccountNameForSecret   = "username"
	AccountPasswdForSecret = "password"
)

const (
	KubernetesClusterDomainEnv = "KUBERNETES_CLUSTER_DOMAIN"
	DefaultDNSDomain           = "cluster.local"
)

const (
	ServiceDescriptorUsernameKey = "username"
	ServiceDescriptorPasswordKey = "password"
	ServiceDescriptorEndpointKey = "endpoint"
	ServiceDescriptorHostKey     = "host"
	ServiceDescriptorPortKey     = "port"
)

const (
	BackupNameKeyForRestore           = "name"
	BackupNamespaceKeyForRestore      = "namespace"
	VolumeRestorePolicyKeyForRestore  = "volumeRestorePolicy"
	DoReadyRestoreAfterClusterRunning = "doReadyRestoreAfterClusterRunning"
	RestoreTimeKeyForRestore          = "restoreTime"
	ConnectionPassword                = "connectionPassword"
)

const (
	KBGeneratedVirtualCompDefPrefix = "KB_GENERATED_VIRTUAL_COMP_DEF"
)

const (
	KubeblocksAPIConversionTypeAnnotationName = "api.kubeblocks.io/converted"
	SourceAPIVersionAnnotationName            = "api.kubeblocks.io/source"

	SourceAPIVersion   = "source"
	MigratedAPIVersion = "migrated"
	ReviewAPIVersion   = "reviewer"
)

const (
	FeatureGateIgnoreConfigTemplateDefaultMode = "IGNORE_CONFIG_TEMPLATE_DEFAULT_MODE"
)

const InvalidContainerPort int32 = 0
