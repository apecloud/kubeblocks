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

// BackupType the backup type, marked backup set is full or incremental or snapshot.
// +enum
// +kubebuilder:validation:Enum={full,incremental,snapshot}
type BackupType string

const (
	BackupTypeFull        BackupType = "full"
	BackupTypeIncremental BackupType = "incremental"
	BackupTypeSnapshot    BackupType = "snapshot"
)

// BaseBackupType the base backup type.
// +enum
// +kubebuilder:validation:Enum={full,snapshot}
type BaseBackupType string

const (
	BaseBackupTypeFull     BaseBackupType = "full"
	BaseBackupTypeSnapshot BaseBackupType = "snapshot"
)

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
