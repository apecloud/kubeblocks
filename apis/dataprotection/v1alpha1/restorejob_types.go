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

// RestoreJobSpec defines the desired state of RestoreJob
type RestoreJobSpec struct {
	// Specified one backupJob to restore.
	// +kubebuilder:validation:Required
	BackupJobName string `json:"backupJobName"`

	// the target database workload to restore
	// +kubebuilder:validation:Required
	Target TargetCluster `json:"target"`

	// array of restore volumes .
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:pruning:PreserveUnknownFields
	TargetVolumes []corev1.Volume `json:"targetVolumes" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// array of restore volume mounts .
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:pruning:PreserveUnknownFields
	TargetVolumeMounts []corev1.VolumeMount `json:"targetVolumeMounts" patchStrategy:"merge" patchMergeKey:"mountPath"`

	// count of backup stop retries on fail.
	// +optional
	OnFailAttempted int32 `json:"onFailAttempted,omitempty"`
}

// RestoreJobStatus defines the observed state of RestoreJob
type RestoreJobStatus struct {

	// +optional
	Phase RestoreJobPhase `json:"phase,omitempty"`

	// The date and time when the Backup is eligible for garbage collection.
	// 'null' means the Backup is NOT be cleaned except delete manual.
	// +optional
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// Date/time when the backup started being processed.
	// +optional
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// Date/time when the backup finished being processed.
	// +optional
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Job failed reason.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Namespaced
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="COMPLETION-TIME",type=date,JSONPath=`.status.completionTimestamp`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`

// RestoreJob is the Schema for the restorejobs API (defined by User)
type RestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreJobSpec   `json:"spec,omitempty"`
	Status RestoreJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RestoreJobList contains a list of RestoreJob
type RestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestoreJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestoreJob{}, &RestoreJobList{})
}
