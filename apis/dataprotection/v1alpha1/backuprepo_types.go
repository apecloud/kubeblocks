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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupRepoSpec defines the desired state of BackupRepo
type BackupRepoSpec struct {
	// The storage provider used by this backup repo.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="StorageProviderRef is immutable"
	// +kubebuilder:validation:Required
	StorageProviderRef string `json:"storageProviderRef"`

	// The requested capacity for the PVC created by this backup repo.
	// +optional
	VolumeCapacity resource.Quantity `json:"volumeCapacity,omitempty"`

	// The reclaim policy for the PV created by this backup repo.
	// +kubebuilder:validation:Enum={Delete,Retain}
	// +kubebuilder:validation:Required
	PVReclaimPolicy corev1.PersistentVolumeReclaimPolicy `json:"pvReclaimPolicy"`

	// Non-secret configurations for the storage provider.
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// A secret that contains the credentials needed by the storage provider.
	// +optional
	Credential *corev1.SecretReference `json:"credential,omitempty"`
}

// BackupRepoStatus defines the observed state of BackupRepo
type BackupRepoStatus struct {
	// Storage provider reconciliation phases. Valid values are PreChecking, Failed, Ready, Deleting.
	// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
	// +optional
	Phase BackupRepoPhase `json:"phase,omitempty"`

	// Describes the current state of the repo.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// GeneratedCSIDriverSecret references the generated secret used by the CSI driver.
	// +optional
	GeneratedCSIDriverSecret *corev1.SecretReference `json:"generatedCSIDriverSecret,omitempty"`

	// GeneratedStorageClassName indicates the generated storage class name.
	// +optional
	GeneratedStorageClassName string `json:"generatedStorageClassName,omitempty"`

	// BackupPVCName is the name of the PVC used to store backup data.
	// +optional
	BackupPVCName string `json:"backupPVCName,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=backuprepos,categories={kubeblocks},scope=Cluster

// BackupRepo is the Schema for the backuprepos API
type BackupRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupRepoSpec   `json:"spec,omitempty"`
	Status BackupRepoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupRepoList contains a list of BackupRepo
type BackupRepoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupRepo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupRepo{}, &BackupRepoList{})
}
