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

package types

const AppName = "kubeblocks-dataprotection"

// config keys used in viper
const (
	// CfgKeyGCFrequencySeconds is the key of gc frequency, its unit is second
	CfgKeyGCFrequencySeconds = "GC_FREQUENCY_SECONDS"
	// CfgKeyWorkerServiceAccountName is the key of service account name for worker
	CfgKeyWorkerServiceAccountName = "WORKER_SERVICE_ACCOUNT_NAME"
	// CfgKeyExecWorkerServiceAccountName is the key of service account name for worker that runs "kubectl exec"
	CfgKeyExecWorkerServiceAccountName = "EXEC_WORKER_SERVICE_ACCOUNT_NAME"
	// CfgKeyWorkerServiceAccountAnnotations is the key of annotations for the service account of the worker
	CfgKeyWorkerServiceAccountAnnotations = "WORKER_SERVICE_ACCOUNT_ANNOTATIONS"
	// CfgKeyWorkerClusterRoleName is the key of cluster role name for binding the service account of the worker
	CfgKeyWorkerClusterRoleName = "WORKER_CLUSTER_ROLE_NAME"
	// CfgDataProtectionReconcileWorkers the max reconcile workers for MaxConcurrentReconciles
	CfgDataProtectionReconcileWorkers = "DATAPROTECTION_RECONCILE_WORKERS"
)

// config default values
const (
	// DefaultGCFrequencySeconds is the default gc frequency, its unit is second
	DefaultGCFrequencySeconds = 60 * 60
)

const (
	// DataProtectionFinalizerName is the name of our custom finalizer
	DataProtectionFinalizerName = "dataprotection.kubeblocks.io/finalizer"
)

// annotation keys
const (
	// DefaultBackupPolicyAnnotationKey specifies the default backup policy.
	DefaultBackupPolicyAnnotationKey = "dataprotection.kubeblocks.io/is-default-policy"
	// DefaultBackupPolicyTemplateAnnotationKey specifies the default backup policy template.
	DefaultBackupPolicyTemplateAnnotationKey = "dataprotection.kubeblocks.io/is-default-policy-template"
	// DefaultBackupRepoAnnotationKey specifies the default backup repo.
	DefaultBackupRepoAnnotationKey = "dataprotection.kubeblocks.io/is-default-repo"
	// ReconfigureRefAnnotationKey specifies the reconfigure ref.
	ReconfigureRefAnnotationKey = "dataprotection.kubeblocks.io/reconfigure-ref"
	// ConnectionPasswordAnnotationKey specifies the password of the connection credential.
	ConnectionPasswordAnnotationKey = "dataprotection.kubeblocks.io/connection-password"
	// GeminiAcknowledgedAnnotationKey indicates whether Gemini has acknowledged the backup.
	GeminiAcknowledgedAnnotationKey = "dataprotection.kubeblocks.io/gemini-acknowledged"
)

// label keys
const (
	// ClusterUIDLabelKey specifies the cluster UID label key.
	ClusterUIDLabelKey = "dataprotection.kubeblocks.io/cluster-uid"
	// BackupNameLabelKey specifies the backup name label key.
	BackupNameLabelKey = "dataprotection.kubeblocks.io/backup-name"
	// BackupNamespaceLabelKey specifies the backup namespace label key.
	BackupNamespaceLabelKey = "dataprotection.kubeblocks.io/backup-namespace"
	// BackupScheduleLabelKey specifies the backup schedule label key.
	BackupScheduleLabelKey = "dataprotection.kubeblocks.io/backup-schedule"
	// BackupPolicyLabelKey specifies the backup policy label key.
	BackupPolicyLabelKey = "dataprotection.kubeblocks.io/backup-policy"
	// BackupMethodLabelKey specifies the backup method label key.
	BackupMethodLabelKey = "dataprotection.kubeblocks.io/backup-method"
	// BackupTypeLabelKey specifies the backup type label key.
	BackupTypeLabelKey = "dataprotection.kubeblocks.io/backup-type"
	// AutoBackupLabelKey specifies the auto backup label key.
	AutoBackupLabelKey = "dataprotection.kubeblocks.io/autobackup"
	// BackupTargetPodLabelKey specifies the backup target pod label key.
	BackupTargetPodLabelKey = "dataprotection.kubeblocks.io/target-pod-name"
)

// env names
const (
	// DPDBHost database host for dataProtection
	DPDBHost = "DP_DB_HOST"
	// DPDBUser database user for dataProtection
	DPDBUser = "DP_DB_USER"
	// DPDBPassword database password for dataProtection
	DPDBPassword = "DP_DB_PASSWORD"
	// DPDBEndpoint database endpoint for dataProtection
	DPDBEndpoint = "DP_DB_ENDPOINT"
	// DPDBPort database port for dataProtection
	DPDBPort = "DP_DB_PORT"
	// DPTargetPodName the target pod name
	DPTargetPodName = "DP_TARGET_POD_NAME"
	// DPTargetPodRole the target pod role
	DPTargetPodRole = "DP_TARGET_POD_ROLE"
	// DPBackupBasePath the base path for backup data in the storage
	// In a backup action pod, it equals ${DP_BACKUP_ROOT_PATH}/${DP_BACKUP_NAME}/${DP_TARGET_RELATIVE_PATH}
	DPBackupBasePath = "DP_BACKUP_BASE_PATH"
	// DPBackupRootPath the root path for backup data
	DPBackupRootPath = "DP_BACKUP_ROOT_PATH"
	// DPTargetRelativePath the relative path based on the backup data root path
	// ${DP_TARGET_RELATIVE_PATH}=${target.name}/${target.selectedTargetPod[*]}
	DPTargetRelativePath = "DP_TARGET_RELATIVE_PATH"
	// DPBackupName backup CR name
	DPBackupName = "DP_BACKUP_NAME"
	// DPParentBackupName backup CR name
	DPParentBackupName = "DP_PARENT_BACKUP_NAME"
	// DPBaseBackupName backup CR name
	DPBaseBackupName = "DP_BASE_BACKUP_NAME"
	// DPAncestorIncrementalBackupNames backup CR names
	// Used to restore incremental backups, recording the incremental ancestor backup names in order of end time
	// For example: ${DP_ANCESTOR_INCREMENTAL_BACKUP_NAMES}=incrementalBackupName1,incrementalBackupName2,incrementalBackupName3
	DPAncestorIncrementalBackupNames = "DP_ANCESTOR_INCREMENTAL_BACKUP_NAMES"
	// DPTTL backup time to live, reference the backup.spec.retentionPeriod
	DPTTL = "DP_TTL"
	// DPCheckInterval check interval for sync backup progress
	DPCheckInterval = "DP_CHECK_INTERVAL"
	// DPBackupInfoFile the file name which retains the backup.status info
	DPBackupInfoFile = "DP_BACKUP_INFO_FILE"
	// DPTimeFormat golang time format string
	DPTimeFormat = "DP_TIME_FORMAT"
	// DPTimeZone golang time zone string
	DPTimeZone = "DP_TIME_ZONE"
	// DPBackupStopTime backup stop time
	DPBackupStopTime = "DP_BACKUP_STOP_TIME" // backup stop time
	// DPDatasafedBinPath the path containing the datasafed binary
	DPDatasafedBinPath = "DP_DATASAFED_BIN_PATH"

	// NOTE: do not add 'DP_' prefix to the value of the following constants, they are the datasafed built-in environment.

	// DPDatasafedLocalBackendPath force datasafed to use local backend with the path
	DPDatasafedLocalBackendPath = "DATASAFED_LOCAL_BACKEND_PATH"
	// DPDatasafedKopiaRepoRoot specifies the root of the Kopia repository
	DPDatasafedKopiaRepoRoot = "DATASAFED_KOPIA_REPO_ROOT"
	// DPDatasafedEncryptionAlgorithm specifies the encryption algorithm for backup data
	DPDatasafedEncryptionAlgorithm = "DATASAFED_ENCRYPTION_ALGORITHM"
	// DPDatasafedEncryptionPassPhrase specifies the encryption key
	DPDatasafedEncryptionPassPhrase = "DATASAFED_ENCRYPTION_PASS_PHRASE"

	DPArchiveInterval      = "DP_ARCHIVE_INTERVAL"
	DPContinuousTTLSeconds = "DP_TTL_SECONDS"
)

const (
	BackupKind             = "Backup"
	RestoreKind            = "Restore"
	DataprotectionAPIGroup = "dataprotection.kubeblocks.io"
	KopiaRepoFolderName    = "kopia"
)

const (
	LogCollectorOutput = "Log Collector Output"
)
