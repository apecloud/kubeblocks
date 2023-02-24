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

// BackupPolicyTemplatePhase defines phases for BackupPolicyTemplate CR.
// +enum
// +kubebuilder:validation:Enum={New,Available,InProgress,Failed}
type BackupPolicyTemplatePhase string

const (
	ConfigNew        BackupPolicyTemplatePhase = "New"
	ConfigAvailable  BackupPolicyTemplatePhase = "Available"
	ConfigInProgress BackupPolicyTemplatePhase = "InProgress"
	ConfigFailed     BackupPolicyTemplatePhase = "Failed"
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
