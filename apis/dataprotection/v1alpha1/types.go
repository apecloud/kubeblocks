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

import (
	"strconv"
	"strings"
	"time"
)

// CreatePVCPolicy the policy how to create the PersistentVolumeClaim for backup.
// +enum
// +kubebuilder:validation:Enum={IfNotPresent,Never}
type CreatePVCPolicy string

const (
	CreatePVCPolicyNever        CreatePVCPolicy = "Never"
	CreatePVCPolicyIfNotPresent CreatePVCPolicy = "IfNotPresent"
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

// ToDuration converts the RetentionPeriod to time.Duration.
func (r RetentionPeriod) ToDuration() time.Duration {
	if len(r.String()) == 0 {
		return time.Duration(0)
	}
	l := strings.ToLower(r.String())
	if strings.HasSuffix(l, "d") {
		days, _ := strconv.Atoi(strings.ReplaceAll(l, "d", ""))
		return time.Hour * 24 * time.Duration(days)
	}
	hours, _ := strconv.Atoi(strings.ReplaceAll(l, "h", ""))
	return time.Hour * time.Duration(hours)
}

func (r RetentionPeriod) String() string {
	return string(r)
}
