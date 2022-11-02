/*
Copyright ApeCloud Inc.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate
type BackupPolicyTemplateSpec struct {
	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +kubebuilder:default="0 7 * * *"
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// which backup tool to perform database backup, only support one tool.
	// +optional
	BackupToolName string `json:"backupToolName"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL metav1.Duration `json:"ttl,omitempty"`

	// database cluster service
	// +kubebuilder:validation:Required
	DatabaseEngine string `json:"databaseEngine"`

	// execute hook commands for backup.
	// +optional
	Hooks BackupPolicyHook `json:"hooks"`

	// array of remote volumes from CSI driver definition.
	// +optional
	RemoteVolume corev1.Volume `json:"remoteVolume"`

	// limit count of backup stop retries on fail.
	// if unset, retry unlimit attempted.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`
}

// The current phase. Valid values are New, Available, InProgress, Failed.
// +enum

type BackupPolicyTemplatePhase string

// These are the valid statuses of BackupPolicyTemplate.
const (
	PolicyNew BackupPolicyTemplatePhase = "New"

	PolicyAvailable BackupPolicyTemplatePhase = "Available"

	PolicyInProgress BackupPolicyTemplatePhase = "InProgress"

	PolicyFailed BackupPolicyTemplatePhase = "Failed"
)

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate
type BackupPolicyTemplateStatus struct {
	// +optional
	Phase BackupPolicyTemplatePhase `json:"phase,omitempty"`

	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={dbaas},scope=Cluster

// BackupPolicyTemplate is the Schema for the BackupPolicyTemplates API (defined by ISV)
type BackupPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupPolicyTemplateSpec   `json:"spec,omitempty"`
	Status BackupPolicyTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupPolicyTemplateList contains a list of BackupPolicyTemplate
type BackupPolicyTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupPolicyTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicyTemplate{}, &BackupPolicyTemplateList{})
}
