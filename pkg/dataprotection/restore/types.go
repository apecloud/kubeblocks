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

package restore

var VolumeSnapshotGroup = "snapshot.storage.k8s.io"

// Restore condition constants
const (
	// condition types
	ConditionTypeRestoreValidationPassed = "ValidationPassed"
	ConditionTypeRestorePreparedData     = "PrepareData"
	ConditionTypeReadinessProbe          = "ReadinessProbe"
	ConditionTypeRestorePostReady        = "PostReady"

	// condition reasons
	ReasonRestoreStarting      = "RestoreStarting"
	ReasonRestoreCompleted     = "RestoreCompleted"
	ReasonRestoreFailed        = "RestoreFailed"
	ReasonValidateFailed       = "ValidateFailed"
	ReasonValidateSuccessfully = "ValidateSuccessfully"
	ReasonProcessing           = "Processing"
	ReasonFailed               = "Failed"
	ReasonSucceed              = "Succeed"
	reasonCreateRestoreJob     = "CreateRestoreJob"
	reasonCreateRestorePVC     = "CreateRestorePVC"
)

// labels key
const (
	DataProtectionRestoreLabelKey          = "dataprotection.kubeblocks.io/restore"
	DataProtectionRestoreNamespaceLabelKey = "dataprotection.kubeblocks.io/restore-namespace"
	DataProtectionPopulatePVCLabelKey      = "dataprotection.kubeblocks.io/populate-pvc"
)

// env name for restore

const (
	DPRestoreTime              = "DP_RESTORE_TIME"
	DPRestoreTimestamp         = "DP_RESTORE_TIMESTAMP"
	DPBaseBackupStartTime      = "DP_BASE_BACKUP_START_TIME"
	DPBaseBackupStartTimestamp = "DP_BASE_BACKUP_START_TIMESTAMP"
	DPBaseBackupStopTime       = "DP_BASE_BACKUP_STOP_TIME"
	DPBaseBackupStopTimestamp  = "DP_BASE_BACKUP_STOP_TIMESTAMP"
)

// Restore constant
const Restore = "restore"

var defaultBackoffLimit int32 = 3
