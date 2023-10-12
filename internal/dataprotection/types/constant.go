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

package types

// config keys used in viper
const (
	// CfgKeyGCFrequencySeconds is the key of gc frequency, its unit is second
	CfgKeyGCFrequencySeconds = "GC_FREQUENCY_SECONDS"
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
	// BackupDataPathPrefixAnnotationKey specifies the backup data path prefix.
	BackupDataPathPrefixAnnotationKey = "dataprotection.kubeblocks.io/path-prefix"
	// ReconfigureRefAnnotationKey specifies the reconfigure ref.
	ReconfigureRefAnnotationKey = "dataprotection.kubeblocks.io/reconfigure-ref"
)

// label keys
const (
	// DataProtectionLabelClusterUIDKey specifies the cluster UID label key.
	DataProtectionLabelClusterUIDKey = "dataprotection.kubeblocks.io/cluster-uid"
	// BackupTypeLabelKeyKey specifies the backup type label key.
	BackupTypeLabelKeyKey = "dataprotection.kubeblocks.io/backup-type"
	// DataProtectionLabelBackupNameKey specifies the backup name label key.
	DataProtectionLabelBackupNameKey = "dataprotection.kubeblocks.io/backup-name"
	// DataProtectionLabelBackupScheduleKey specifies the backup schedule label key.
	DataProtectionLabelBackupScheduleKey = "dataprotection.kubeblocks.io/backup-schedule"
	// DataProtectionLabelBackupPolicyKey specifies the backup policy label key.
	DataProtectionLabelBackupPolicyKey = "dataprotection.kubeblocks.io/backup-policy"
	// DataProtectionLabelBackupMethodKey specifies the backup method label key.
	DataProtectionLabelBackupMethodKey = "dataprotection.kubeblocks.io/backup-method"
	// DataProtectionLabelBackupTypeKey specifies the backup type label key.
	DataProtectionLabelBackupTypeKey = "dataprotection.kubeblocks.io/backup-type"
	// DataProtectionLabelAutoBackupKey specifies the auto backup label key.
	DataProtectionLabelAutoBackupKey = "dataprotection.kubeblocks.io/autobackup"
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
	// DPBackupBasePath the base path for backup data in the storage
	DPBackupBasePath = "DP_BACKUP_BASE_PATH"
	// DPBackupName backup CR name
	DPBackupName = "DP_BACKUP_NAME"
	// DPTTL backup time to live, reference the backupPolicy.spec.retention.ttl
	DPTTL = "DP_TTL"
	// DPCheckInterval check interval for continue backup
	DPCheckInterval = "DP_CHECK_INTERVAL"
	// DPBackupInfoFile the file name which retains the backup.status info
	DPBackupInfoFile = "DP_BACKUP_INFO_FILE"
	// DPTimeFormat golang time format string
	DPTimeFormat = "TIME_FORMAT"
	// DPVolumeDataDIR the volume data dir
	DPVolumeDataDIR = "VOLUME_DATA_DIR" //
	// DPKBRecoveryTime recovery time
	DPKBRecoveryTime = "KB_RECOVERY_TIME" // recovery time
	// DPKBRecoveryTimestamp recovery timestamp
	DPKBRecoveryTimestamp = "KB_RECOVERY_TIMESTAMP" // recovery timestamp
	// DPBaseBackupStartTime base backup start time for pitr
	DPBaseBackupStartTime = "BASE_BACKUP_START_TIME" // base backup start time for pitr
	// DPBaseBackupStartTimestamp base backup start timestamp for pitr
	DPBaseBackupStartTimestamp = "BASE_BACKUP_START_TIMESTAMP" // base backup start timestamp for pitr
	// DPBackupStopTime backup stop time
	DPBackupStopTime = "BACKUP_STOP_TIME" // backup stop time
	// DPDatasafedLocalBackendPath force datasafed to use local backend with the path
	DPDatasafedLocalBackendPath = "DATASAFED_LOCAL_BACKEND_PATH"
	// DPDatasafedBinPath the path containing the datasafed binary
	DPDatasafedBinPath = "DP_DATASAFED_BIN_PATH"
)

const (
	RestoreKind            = "Restore"
	DataprotectionAPIGroup = "dataprotection.kubeblocks.io"
)
