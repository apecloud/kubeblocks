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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupPolicyTemplateSpec defines the desired state of BackupPolicyTemplate
type BackupPolicyTemplateSpec struct {
	// The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// which backup tool to perform database backup, only support one tool.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	BackupToolName string `json:"backupToolName"`

	// TTL is a time.Duration-parseable string describing how long
	// the Backup should be retained for.
	// +optional
	TTL *metav1.Duration `json:"ttl,omitempty"`

	// limit count of backup stop retries on fail.
	// if unset, retry unlimit attempted.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`

	// execute hook commands for backup.
	// +optional
	Hooks *BackupPolicyHook `json:"hooks,omitempty"`

	// CredentialKeyword determines backupTool connection credential keyword in secret.
	// the backupTool gets the credentials according to the user and password keyword defined by secret
	// +optional
	CredentialKeyword *BackupPolicyCredentialKeyword `json:"credentialKeyword,omitempty"`

	// define how to update metadata for backup status.
	// +optional
	BackupStatusUpdates []BackupStatusUpdate `json:"backupStatusUpdates,omitempty"`

	// PointInTimeRecovery determines scripts and configurations of the recovery point in time.
	// +optional
	PointInTimeRecovery *BackupPointInTimeRecovery `json:"pointInTimeRecovery,omitempty"`
}

// BackupPolicyCredentialKeyword defined for the target database secret that backup tool can connect.
type BackupPolicyCredentialKeyword struct {
	// UserKeyword the map keyword of the user in the connection credential secret
	// +kubebuilder:default=username
	// +optional
	UserKeyword string `json:"userKeyword,omitempty"`

	// PasswordKeyword the map keyword of the password in the connection credential secret
	// +kubebuilder:default=password
	// +optional
	PasswordKeyword string `json:"passwordKeyword,omitempty"`
}

// BackupPointInTimeRecovery defines the backup point in time recovery info of BackupPolicyTemplate
type BackupPointInTimeRecovery struct {
	// +optional
	Scripts *corev1.Container `json:"scripts,omitempty"`

	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// BackupPolicyTemplateStatus defines the observed state of BackupPolicyTemplate
type BackupPolicyTemplateStatus struct {
	// +optional
	Phase BackupPolicyTemplatePhase `json:"phase,omitempty"`

	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster

// BackupPolicyTemplate is the Schema for the BackupPolicyTemplates API (defined by provider)
type BackupPolicyTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupPolicyTemplateSpec   `json:"spec,omitempty"`
	Status BackupPolicyTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupPolicyTemplateList contains a list of BackupPolicyTemplate
type BackupPolicyTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupPolicyTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupPolicyTemplate{}, &BackupPolicyTemplateList{})
}
