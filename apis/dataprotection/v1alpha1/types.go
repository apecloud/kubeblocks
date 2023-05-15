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

package v1alpha1

// BackupPhase The current phase. Valid values are New, InProgress, Completed, Failed.
// +enum
// +kubebuilder:validation:Enum={New,InProgress,Completed,Failed}
type BackupPhase string

const (
	BackupNew        BackupPhase = "New"
	BackupInProgress BackupPhase = "InProgress"
	BackupCompleted  BackupPhase = "Completed"
	BackupFailed     BackupPhase = "Failed"
)

// BackupType the backup type, marked backup set is datafile or logfile or snapshot.
// +enum
// +kubebuilder:validation:Enum={datafile,logfile,snapshot}
type BackupType string

const (
	BackupTypeDataFile BackupType = "datafile"
	BackupTypeLogFile  BackupType = "logfile"
	BackupTypeSnapshot BackupType = "snapshot"
)

// BaseBackupType the base backup type.
// +enum
// +kubebuilder:validation:Enum={full,snapshot}
type BaseBackupType string

// CreatePVCPolicy the policy how to create the PersistentVolumeClaim for backup.
// +enum
// +kubebuilder:validation:Enum={IfNotPresent,Never}
type CreatePVCPolicy string

const (
	CreatePVCPolicyNever        CreatePVCPolicy = "Never"
	CreatePVCPolicyIfNotPresent CreatePVCPolicy = "IfNotPresent"
)

// BackupPolicyPhase defines phases for BackupPolicy CR.
// +enum
// +kubebuilder:validation:Enum={Available,Failed}
type BackupPolicyPhase string

const (
	PolicyAvailable BackupPolicyPhase = "Available"
	PolicyFailed    BackupPolicyPhase = "Failed"
)

// RestoreJobPhase The current phase. Valid values are New, InProgressPhy, InProgressLogic, Completed, Failed.
// +enum
// +kubebuilder:validation:Enum={New,InProgressPhy,InProgressLogic,Completed,Failed}
type RestoreJobPhase string

const (
	RestoreJobNew             RestoreJobPhase = "New"
	RestoreJobInProgressPhy   RestoreJobPhase = "InProgressPhy"
	RestoreJobInProgressLogic RestoreJobPhase = "InProgressLogic"
	RestoreJobCompleted       RestoreJobPhase = "Completed"
	RestoreJobFailed          RestoreJobPhase = "Failed"
)
