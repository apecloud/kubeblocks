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
)

// env names
const (
	DPTargetPodName = "DP_TARGET_POD_NAME"
	// DPDBHost database host for dataProtection
	DPDBHost = "DB_HOST"
	// DPDBUser database user for dataProtection
	DPDBUser = "DB_USER"
	// DPDBPassword database password for dataProtection
	DPDBPassword = "DB_PASSWORD"
	// DPBackupDIR the dest directory for backup data
	DPBackupDIR = "BACKUP_DIR"
	// DPLogFileDIR log file dir
	DPLogFileDIR = "BACKUP_LOGFILE_DIR"
	// DPBackupName backup CR name
	DPBackupName = "BACKUP_NAME"
	// DPTTL backup time to live, reference the backupPolicy.spec.retention.ttl
	DPTTL = "TTL"
	// DPLogfileTTL ttl for logfile backup, one more day than backupPolicy.spec.retention.ttl
	DPLogfileTTL = "LOGFILE_TTL"
	// DPLogfileTTLSecond ttl seconds with LOGFILE_TTL, integer format
	DPLogfileTTLSecond = "LOGFILE_TTL_SECOND"
	// DPArchiveInterval archive interval for statefulSet deploy kind, trans from the schedule cronExpression for logfile
	DPArchiveInterval = "ARCHIVE_INTERVAL"
	// DPBackupInfoFile the file name which retains the backup.status info
	DPBackupInfoFile = "BACKUP_INFO_FILE"
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
)
