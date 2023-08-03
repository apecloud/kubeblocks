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
	CfgKeyBackupPVCName                 = "BACKUP_PVC_NAME"                  // the global persistent volume claim to store the backup data
	CfgKeyBackupPVCInitCapacity         = "BACKUP_PVC_INIT_CAPACITY"         // the init capacity for creating the pvc, e.g. 10Gi.
	CfgKeyBackupPVCStorageClass         = "BACKUP_PVC_STORAGE_CLASS"         // the pvc storage class name.
	CfgKeyBackupPVCCreatePolicy         = "BACKUP_PVC_CREATE_POLICY"         // the pvc creation policy, choice is "IfNotPresent" or "Never"
	CfgKeyBackupPVConfigmapName         = "BACKUP_PV_CONFIGMAP_NAME"         // the configmap containing the persistentVolume template.
	CfgKeyBackupPVConfigmapNamespace    = "BACKUP_PV_CONFIGMAP_NAMESPACE"    // the configmap namespace containing the persistentVolume template.
	CfgRecoverVolumeExpansionFailure    = "RECOVER_VOLUME_EXPANSION_FAILURE" // refer to feature gates RecoverVolumeExpansionFailure of k8s.

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
	KBChartsImage            = "KUBEBLOCKS_CHARTS_IMAGE"
	KBDataScriptClientsImage = "KUBEBLOCKS_DATASCRIPT_CLIENTS_IMAGE"
)

const (
	APIGroup = "kubeblocks.io"

	AppName = "kubeblocks"

	// K8s recommonded well-known labels and annotation keys
	AppInstanceLabelKey  = "app.kubernetes.io/instance"
	AppNameLabelKey      = "app.kubernetes.io/name"
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
	CMConfigurationConstraintsNameLabelKey   = "config.kubeblocks.io/config-constraints-name"
	ConsensusSetAccessModeLabelKey           = "cs.apps.kubeblocks.io/access-mode"
	BackupTypeLabelKeyKey                    = "dataprotection.kubeblocks.io/backup-type"
	DataProtectionLabelBackupNameKey         = "dataprotection.kubeblocks.io/backup-name"
	AddonNameLabelKey                        = "extensions.kubeblocks.io/addon-name"
	OpsRequestTypeLabelKey                   = "ops.kubeblocks.io/ops-type"
	OpsRequestNameLabelKey                   = "ops.kubeblocks.io/ops-name"

	// kubeblocks.io annotations
	ClusterSnapshotAnnotationKey                = "kubeblocks.io/cluster-snapshot"            // ClusterSnapshotAnnotationKey saves the snapshot of cluster.
	DefaultClusterVersionAnnotationKey          = "kubeblocks.io/is-default-cluster-version"  // DefaultClusterVersionAnnotationKey specifies the default cluster version.
	OpsRequestAnnotationKey                     = "kubeblocks.io/ops-request"                 // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	ReconcileAnnotationKey                      = "kubeblocks.io/reconcile"                   // ReconcileAnnotationKey Notify k8s object to reconcile
	RestartAnnotationKey                        = "kubeblocks.io/restart"                     // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	RestoreFromTimeAnnotationKey                = "kubeblocks.io/restore-from-time"           // RestoreFromTimeAnnotationKey specifies the time to recover from the backup.
	RestoreFromSrcClusterAnnotationKey          = "kubeblocks.io/restore-from-source-cluster" // RestoreFromSrcClusterAnnotationKey specifies the source cluster to recover from the backup.
	RestoreFromBackUpAnnotationKey              = "kubeblocks.io/restore-from-backup"         // RestoreFromBackUpAnnotationKey specifies the component to recover from the backup.
	SnapShotForStartAnnotationKey               = "kubeblocks.io/snapshot-for-start"
	ComponentReplicasAnnotationKey              = "apps.kubeblocks.io/component-replicas" // ComponentReplicasAnnotationKey specifies the number of pods in replicas
	BackupPolicyTemplateAnnotationKey           = "apps.kubeblocks.io/backup-policy-template"
	LastAppliedClusterAnnotationKey             = "apps.kubeblocks.io/last-applied-cluster"
	PVLastClaimPolicyAnnotationKey              = "apps.kubeblocks.io/pv-last-claim-policy"
	HaltRecoveryAllowInconsistentCVAnnotKey     = "clusters.apps.kubeblocks.io/allow-inconsistent-cv"
	HaltRecoveryAllowInconsistentResAnnotKey    = "clusters.apps.kubeblocks.io/allow-inconsistent-resource"
	LeaderAnnotationKey                         = "cs.apps.kubeblocks.io/leader"
	PrimaryAnnotationKey                        = "rs.apps.kubeblocks.io/primary"
	DefaultBackupPolicyAnnotationKey            = "dataprotection.kubeblocks.io/is-default-policy"          // DefaultBackupPolicyAnnotationKey specifies the default backup policy.
	DefaultBackupPolicyTemplateAnnotationKey    = "dataprotection.kubeblocks.io/is-default-policy-template" // DefaultBackupPolicyTemplateAnnotationKey specifies the default backup policy template.
	DefaultBackupRepoAnnotationKey              = "dataprotection.kubeblocks.io/is-default-repo"            // DefaultBackupRepoAnnotationKey specifies the default backup repo.
	BackupDataPathPrefixAnnotationKey           = "dataprotection.kubeblocks.io/path-prefix"                // BackupDataPathPrefixAnnotationKey specifies the backup data path prefix.
	ReconfigureRefAnnotationKey                 = "dataprotection.kubeblocks.io/reconfigure-ref"
	DataProtectionLabelClusterUIDKey            = "dataprotection.kubeblocks.io/cluster-uid"
	DisableUpgradeInsConfigurationAnnotationKey = "config.kubeblocks.io/disable-reconfigure"
	LastAppliedConfigAnnotationKey              = "config.kubeblocks.io/last-applied-configuration"
	LastAppliedOpsCRAnnotationKey               = "config.kubeblocks.io/last-applied-ops-name"
	UpgradePolicyAnnotationKey                  = "config.kubeblocks.io/reconfigure-policy"
	KBParameterUpdateSourceAnnotationKey        = "config.kubeblocks.io/reconfigure-source"
	UpgradeRestartAnnotationKey                 = "config.kubeblocks.io/restart"
	KubeBlocksGenerationKey                     = "kubeblocks.io/generation"
	ExtraEnvAnnotationKey                       = "kubeblocks.io/extra-env"

	// kubeblocks.io well-known finalizers
	DBClusterFinalizerName             = "cluster.kubeblocks.io/finalizer"
	ConfigurationTemplateFinalizerName = "config.kubeblocks.io/finalizer"

	// ConfigurationTplLabelPrefixKey clusterVersion or clusterdefinition using tpl
	ConfigurationTplLabelPrefixKey         = "config.kubeblocks.io/tpl"
	ConfigurationConstraintsLabelPrefixKey = "config.kubeblocks.io/constraints"

	// CMInsLastReconfigurePhaseKey defines the current phase
	CMInsLastReconfigurePhaseKey = "config.kubeblocks.io/last-applied-reconfigure-phase"

	// CMInsEnableRerenderTemplateKey is used to enable rerender template
	CMInsEnableRerenderTemplateKey = "config.kubeblocks.io/enable-rerender"

	// IgnoreResourceConstraint is used to specify whether to ignore the resource constraint
	IgnoreResourceConstraint = "resource.kubeblocks.io/ignore-constraint"
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
	ProbeHTTPPortName                  = "probe-http-port"
	ProbeGRPCPortName                  = "probe-grpc-port"
	ProbeInitContainerName             = "kb-initprobe"
	RoleProbeContainerName             = "kb-checkrole"
	StatusProbeContainerName           = "kb-checkstatus"
	RunningProbeContainerName          = "kb-checkrunning"
	VolumeProtectionProbeContainerName = "kb-volume-protection"

	// the filedpath name used in event.InvolvedObject.FieldPath
	ProbeCheckRolePath    = "spec.containers{" + RoleProbeContainerName + "}"
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
	// dataProtection env names

	DPDBHost                   = "DB_HOST"                     // db host for dataProtection
	DPDBUser                   = "DB_USER"                     // db user for dataProtection
	DPDBPassword               = "DB_PASSWORD"                 // db password for dataProtection
	DPBackupDIR                = "BACKUP_DIR"                  // the dest directory for backup data
	DPLogFileDIR               = "BACKUP_LOGFILE_DIR"          // logfile dir
	DPBackupName               = "BACKUP_NAME"                 // backup cr name
	DPTTL                      = "TTL"                         // backup time to live, reference the backupPolicy.spec.retention.ttl
	DPLogfileTTL               = "LOGFILE_TTL"                 // ttl for logfile backup, one more day than backupPolicy.spec.retention.ttl
	DPLogfileTTLSecond         = "LOGFILE_TTL_SECOND"          // ttl seconds with LOGFILE_TTL, integer format
	DPArchiveInterval          = "ARCHIVE_INTERVAL"            // archive interval for statefulSet deploy kind, trans from the schedule cronExpression for logfile
	DPBackupInfoFile           = "BACKUP_INFO_FILE"            // the file name which retains the backup.status info
	DPTimeFormat               = "TIME_FORMAT"                 // golang time format string
	DPVolumeDataDIR            = "VOLUME_DATA_DIR"             //
	DPKBRecoveryTime           = "KB_RECOVERY_TIME"            // recovery time
	DPKBRecoveryTimestamp      = "KB_RECOVERY_TIMESTAMP"       // recovery timestamp
	DPBaseBackupStartTime      = "BASE_BACKUP_START_TIME"      // base backup start time for pitr
	DPBaseBackupStartTimestamp = "BASE_BACKUP_START_TIMESTAMP" // base backup start timestamp for pitr
	DPBackupStopTime           = "BACKUP_STOP_TIME"            // backup stop time
)

const (
	KubernetesClusterDomainEnv = "KUBERNETES_CLUSTER_DOMAIN"
	DefaultDNSDomain           = "cluster.local"
)
