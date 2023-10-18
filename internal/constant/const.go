/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	// addon config keys
	CfgKeyAddonJobTTL        = "ADDON_JOB_TTL"
	CfgAddonJobImgPullPolicy = "ADDON_JOB_IMAGE_PULL_POLICY"

	// data plane config key
	CfgKeyDataPlaneTolerations = "DATA_PLANE_TOLERATIONS"
	CfgKeyDataPlaneAffinity    = "DATA_PLANE_AFFINITY"

	// storage config keys
	CfgKeyDefaultStorageClass = "DEFAULT_STORAGE_CLASS"
)

const (
	KBConnCredentialPlaceHolder     = "$(CONN_CREDENTIAL_SECRET_NAME)"
	KBComponentEnvCMPlaceHolder     = "$(COMP_ENV_CM_NAME)"
	KBCompNamePlaceHolder           = "$(KB_COMP_NAME)"
	KBClusterNamePlaceHolder        = "$(KB_CLUSTER_NAME)"
	KBClusterCompNamePlaceHolder    = "$(KB_CLUSTER_COMP_NAME)"
	KBClusterUIDPostfix8PlaceHolder = "$(KB_CLUSTER_UID_POSTFIX_8)"
	KBToolsImagePlaceHolder         = "$(KUBEBLOCKS_TOOLS_IMAGE)"
)

const (
	KBPrefix = "KB"
)

const (
	KBToolsImage             = "KUBEBLOCKS_TOOLS_IMAGE"
	KBImagePullPolicy        = "KUBEBLOCKS_IMAGE_PULL_POLICY"
	KBDataScriptClientsImage = "KUBEBLOCKS_DATASCRIPT_CLIENTS_IMAGE"
)

const (
	APIGroup = "kubeblocks.io"

	AppName = "kubeblocks"

	// K8s recommonded well-known labels and annotation keys
	AppInstanceLabelKey  = "app.kubernetes.io/instance"
	AppNameLabelKey      = "app.kubernetes.io/name"
	AppComponentLabelKey = "app.kubernetes.io/component"
	AppVersionLabelKey   = "app.kubernetes.io/version"
	AppManagedByLabelKey = "app.kubernetes.io/managed-by"
	RegionLabelKey       = "topology.kubernetes.io/region"
	ZoneLabelKey         = "topology.kubernetes.io/zone"

	// kubeblocks.io labels
	BackupProtectionLabelKey                 = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	BackupToolTypeLabelKey                   = "kubeblocks.io/backup-tool-type"
	AddonProviderLabelKey                    = "kubeblocks.io/provider" // AddonProviderLabelKey marks the addon provider
	RoleLabelKey                             = "kubeblocks.io/role"     // RoleLabelKey consensusSet and replicationSet role label key
	ModeKey                                  = "kubeblocks.io/mode"     // ModeKey is in enum of standalone/replication/raftGroup
	VolumeTypeLabelKey                       = "kubeblocks.io/volume-type"
	ClusterAccountLabelKey                   = "account.kubeblocks.io/name"
	KBAppComponentLabelKey                   = "apps.kubeblocks.io/component-name"
	KBAppComponentDefRefLabelKey             = "apps.kubeblocks.io/component-def-ref"
	AppConfigTypeLabelKey                    = "apps.kubeblocks.io/config-type"
	KBManagedByKey                           = "apps.kubeblocks.io/managed-by" // KBManagedByKey marks resources that auto created
	PVCNameLabelKey                          = "apps.kubeblocks.io/pvc-name"
	VolumeClaimTemplateNameLabelKey          = "apps.kubeblocks.io/vct-name"
	VolumeClaimTemplateNameLabelKeyForLegacy = "vct.kubeblocks.io/name" // Deprecated: only compatible with version 0.5, will be removed in 0.7
	WorkloadTypeLabelKey                     = "apps.kubeblocks.io/workload-type"
	ClassProviderLabelKey                    = "class.kubeblocks.io/provider"
	ClusterDefLabelKey                       = "clusterdefinition.kubeblocks.io/name"
	ClusterVerLabelKey                       = "clusterversion.kubeblocks.io/name"
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
	ServiceDescriptorNameLabelKey            = "servicedescriptor.kubeblocks.io/name"
	RestoreForHScaleLabelKey                 = "apps.kubeblocks.io/restore-for-hscale"

	// kubeblocks.io annotations
	ClusterSnapshotAnnotationKey                = "kubeblocks.io/cluster-snapshot"           // ClusterSnapshotAnnotationKey saves the snapshot of cluster.
	DefaultClusterVersionAnnotationKey          = "kubeblocks.io/is-default-cluster-version" // DefaultClusterVersionAnnotationKey specifies the default cluster version.
	OpsRequestAnnotationKey                     = "kubeblocks.io/ops-request"                // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	ReconcileAnnotationKey                      = "kubeblocks.io/reconcile"                  // ReconcileAnnotationKey Notify k8s object to reconcile
	RestartAnnotationKey                        = "kubeblocks.io/restart"                    // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	RestoreFromBackupAnnotationKey              = "kubeblocks.io/restore-from-backup"        // RestoreFromBackupAnnotationKey specifies the component to recover from the backup.
	SnapShotForStartAnnotationKey               = "kubeblocks.io/snapshot-for-start"
	ComponentReplicasAnnotationKey              = "apps.kubeblocks.io/component-replicas" // ComponentReplicasAnnotationKey specifies the number of pods in replicas
	BackupPolicyTemplateAnnotationKey           = "apps.kubeblocks.io/backup-policy-template"
	LastAppliedClusterAnnotationKey             = "apps.kubeblocks.io/last-applied-cluster"
	PVLastClaimPolicyAnnotationKey              = "apps.kubeblocks.io/pv-last-claim-policy"
	HaltRecoveryAllowInconsistentCVAnnotKey     = "clusters.apps.kubeblocks.io/allow-inconsistent-cv"
	HaltRecoveryAllowInconsistentResAnnotKey    = "clusters.apps.kubeblocks.io/allow-inconsistent-resource"
	LeaderAnnotationKey                         = "cs.apps.kubeblocks.io/leader"
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

	// kubeblocks.io well-known finalizers
	DBClusterFinalizerName             = "cluster.kubeblocks.io/finalizer"
	ConfigurationTemplateFinalizerName = "config.kubeblocks.io/finalizer"
	ServiceDescriptorFinalizerName     = "servicedescriptor.kubeblocks.io/finalizer"

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
	RSMKind                   = "ReplicatedStateMachine"
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
	ProbeInitContainerName             = "kb-initprobe"
	WeSyncerContainerName              = "kb-we-syncer"
	RoleProbeContainerName             = "kb-checkrole"
	StatusProbeContainerName           = "kb-checkstatus"
	RunningProbeContainerName          = "kb-checkrunning"
	VolumeProtectionProbeContainerName = "kb-volume-protection"

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
)

const (
	Primary   = "primary"
	Secondary = "secondary"

	Leader   = "leader"
	Follower = "follower"
	Learner  = "learner"
)

// switchover constants
const (
	KBJobTTLSecondsAfterFinished           = 5
	KBSwitchoverCandidateInstanceForAnyPod = "*"

	KBSwitchoverJobLabelKey      = "kubeblocks.io/switchover-job"
	KBSwitchoverJobLabelValue    = "kb-switchover-job"
	KBSwitchoverJobNamePrefix    = "kb-switchover-job"
	KBSwitchoverJobContainerName = "kb-switchover-job-container"

	KBSwitchoverCandidateName             = "KB_SWITCHOVER_CANDIDATE_NAME"
	KBSwitchoverCandidateFqdn             = "KB_SWITCHOVER_CANDIDATE_FQDN"
	KBSwitchoverReplicationPrimaryPodIP   = "KB_REPLICATION_PRIMARY_POD_IP"
	KBSwitchoverReplicationPrimaryPodName = "KB_REPLICATION_PRIMARY_POD_NAME"
	KBSwitchoverReplicationPrimaryPodFqdn = "KB_REPLICATION_PRIMARY_POD_FQDN"
	KBSwitchoverConsensusLeaderPodIP      = "KB_CONSENSUS_LEADER_POD_IP"
	KBSwitchoverConsensusLeaderPodName    = "KB_CONSENSUS_LEADER_POD_NAME"
	KBSwitchoverConsensusLeaderPodFqdn    = "KB_CONSENSUS_LEADER_POD_FQDN"
)

// username and password are keys in created secrets for others to refer to.
const (
	AccountNameForSecret   = "username"
	AccountPasswdForSecret = "password"
)

const DefaultBackupPvcInitCapacity = "20Gi"

const (
	ComponentStatusDefaultPodName = "Unknown"
)

const (
	FeatureGateReplicatedStateMachine = "REPLICATED_STATE_MACHINE" // enable rsm
)

const (
	KubernetesClusterDomainEnv = "KUBERNETES_CLUSTER_DOMAIN"
	DefaultDNSDomain           = "cluster.local"
)

const (
	ServiceDescriptorUsernameKey = "username"
	ServiceDescriptorPasswordKey = "password"
	ServiceDescriptorEndpointKey = "endpoint"
	ServiceDescriptorPortKey     = "port"
)

const (
	BackupNameKeyForRestore             = "name"
	BackupNamespaceKeyForRestore        = "namespace"
	VolumeManagementPolicyKeyForRestore = "managementPolicy"
	RestoreTimeKeyForRestore            = "restoreTime"
)
