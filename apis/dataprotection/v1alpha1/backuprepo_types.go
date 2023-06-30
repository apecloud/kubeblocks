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

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

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
	PVReclaimPolicy v1.PersistentVolumeReclaimPolicy `json:"pvReclaimPolicy"`

	// Non-secret configurations for the storage provider.
	Config map[string]string `json:"config,omitempty"`

	// A secret that contains the credentials needed by the storage provider.
	// +optional
	Credential *v1.SecretReference `json:"credential,omitempty"`
}

// BackupRepoStatus defines the observed state of BackupRepo
type BackupRepoStatus struct {
	// Storage provider reconciliation phases. Valid values are PreChecking, Failed, Ready, Deleting.
	// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
	Phase BackupRepoPhase `json:"phase,omitempty"`

	// Describes the current state of the repo.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// GeneratedCSIDriverSecret references the generated secret used by the CSI driver.
	// +optional
	GeneratedCSIDriverSecret *v1.SecretReference `json:"generatedCSIDriverSecret,omitempty"`

	// GeneratedStorageClassName indicates the generated storage class name.
	// +optional
	GeneratedStorageClassName string `json:"generatedStorageClassName,omitempty"`

	// BackupPVCName is the name of the PVC used to store backup data.
	// +optional
	BackupPVCName string `json:"backupPVCName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// BackupRepo is the Schema for the backuprepoes API
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
