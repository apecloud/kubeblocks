/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

// BackupPhase The current phase. Valid values are New, InProgress, Completed, Failed.
// +enum
// +kubebuilder:validation:Enum={New,InProgress,Running,Completed,Failed,Deleting}
type BackupPhase string

const (
	BackupNew        BackupPhase = "New"
	BackupInProgress BackupPhase = "InProgress"
	BackupRunning    BackupPhase = "Running"
	BackupCompleted  BackupPhase = "Completed"
	BackupFailed     BackupPhase = "Failed"
	BackupDeleting   BackupPhase = "Deleting"
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

// BackupMethod the backup method
// +enum
// +kubebuilder:validation:Enum={snapshot,backupTool}
type BackupMethod string

const (
	BackupMethodSnapshot   BackupMethod = "snapshot"
	BackupMethodBackupTool BackupMethod = "backupTool"
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

// DeployKind which kind for run a backup tool.
// +enum
// +kubebuilder:validation:Enum={job,statefulSet}
type DeployKind string

const (
	DeployKindJob         DeployKind = "job"
	DeployKindStatefulSet DeployKind = "statefulSet"
)

// PodRestoreScope defines the scope pod for restore from backup.
// +enum
// +kubebuilder:validation:Enum={All,ReadWrite}
type PodRestoreScope string

const (
	PodRestoreScopeAll       = "All"
	PodRestoreScopeReadWrite = "ReadWrite"
)

// BackupRepoPhase defines phases for BackupRepo CR.
// +enum
// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
type BackupRepoPhase string

const (
	BackupRepoPreChecking BackupRepoPhase = "PreChecking"
	BackupRepoFailed      BackupRepoPhase = "Failed"
	BackupRepoReady       BackupRepoPhase = "Ready"
	BackupRepoDeleting    BackupRepoPhase = "Deleting"
)

// RetentionPeriod represents a duration in the format "1y2mo3w4d5h6m", where
// y=year, mo=month, w=week, d=day, h=hour, m=minute.
type RetentionPeriod string
