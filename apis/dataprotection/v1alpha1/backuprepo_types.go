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

// AccessMethod is an enum type that defines the access method of the backup repo.
type AccessMethod string

const (
	// AccessMethodMount means that the storage is mounted locally,
	// so that remote files can be accessed just like a local file.
	AccessMethodMount AccessMethod = "Mount"
	// AccessMethodTool means to access the storage with a command-line tool,
	// which helps to transfer files between the storage and local.
	AccessMethodTool AccessMethod = "Tool"
)

// BackupRepoSpec defines the desired state of BackupRepo
type BackupRepoSpec struct {
	// The storage provider used by this backup repo.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="StorageProviderRef is immutable"
	// +kubebuilder:validation:Required
	StorageProviderRef string `json:"storageProviderRef"`

	// Specifies the access method of the backup repo.
	// +kubebuilder:validation:Enum={Mount,Tool}
	// +kubebuilder:default=Mount
	// +optional
	AccessMethod AccessMethod `json:"accessMethod,omitempty"`

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
	// Backup repo reconciliation phases. Valid values are PreChecking, Failed, Ready, Deleting.
	// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
	// +optional
	Phase BackupRepoPhase `json:"phase,omitempty"`

	// conditions describes the current state of the repo.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// generatedCSIDriverSecret references the generated secret used by the CSI driver.
	// +optional
	GeneratedCSIDriverSecret *corev1.SecretReference `json:"generatedCSIDriverSecret,omitempty"`

	// generatedStorageClassName indicates the generated storage class name.
	// +optional
	GeneratedStorageClassName string `json:"generatedStorageClassName,omitempty"`

	// backupPVCName is the name of the PVC used to store backup data.
	// +optional
	BackupPVCName string `json:"backupPVCName,omitempty"`

	// toolConfigSecretName is the name of the secret containing the configuration for the access tool.
	// +optional
	ToolConfigSecretName string `json:"toolConfigSecretName,omitempty"`

	// isDefault indicates whether this backup repo is the default one.
	// +optional
	IsDefault bool `json:"isDefault,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=backuprepos,categories={kubeblocks},scope=Cluster
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="STORAGEPROVIDER",type="string",JSONPath=".spec.storageProviderRef"
// +kubebuilder:printcolumn:name="ACCESSMETHOD",type="string",JSONPath=".spec.accessMethod"
// +kubebuilder:printcolumn:name="DEFAULT",type="boolean",JSONPath=`.status.isDefault`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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

func (repo *BackupRepo) AccessByMount() bool {
	return repo.Spec.AccessMethod == "" || repo.Spec.AccessMethod == AccessMethodMount
}

func (repo *BackupRepo) AccessByTool() bool {
	return repo.Spec.AccessMethod == AccessMethodTool
}
