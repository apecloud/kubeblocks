/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

// AccessMethod represents an enumeration type that outlines
// how the `BackupRepo` can be accessed.
type AccessMethod string

const (
	// AccessMethodMount suggests that the storage is mounted locally
	// which allows for remote files to be accessed akin to local ones.
	AccessMethodMount AccessMethod = "Mount"
	// AccessMethodTool indicates the utilization of a command-line
	// tool for accessing the storage.
	AccessMethodTool AccessMethod = "Tool"
)

// BackupRepoSpec defines the desired state of `BackupRepo`.
type BackupRepoSpec struct {
	// Specifies the name of the `StorageProvider` used by this backup repository.
	//
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="StorageProviderRef is immutable"
	// +kubebuilder:validation:Required
	StorageProviderRef string `json:"storageProviderRef"`

	// Specifies the access method of the backup repository.
	//
	// +kubebuilder:validation:Enum={Mount,Tool}
	// +kubebuilder:default=Mount
	// +optional
	AccessMethod AccessMethod `json:"accessMethod,omitempty"`

	// Specifies the capacity of the PVC created by this backup repository.
	//
	// +optional
	VolumeCapacity resource.Quantity `json:"volumeCapacity,omitempty"`

	// Specifies reclaim policy of the PV created by this backup repository.
	//
	// +kubebuilder:validation:Enum={Delete,Retain}
	// +kubebuilder:validation:Required
	PVReclaimPolicy corev1.PersistentVolumeReclaimPolicy `json:"pvReclaimPolicy"`

	// Stores the non-secret configuration parameters for the `StorageProvider`.
	//
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// References to the secret that holds the credentials for the `StorageProvider`.
	//
	// +optional
	Credential *corev1.SecretReference `json:"credential,omitempty"`

	// Specifies the prefix of the path for storing backup data.
	//
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9-_]+/?)*$`
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`
}

// BackupRepoStatus defines the observed state of `BackupRepo`.
type BackupRepoStatus struct {
	// Represents the current phase of reconciliation for the backup repository.
	// Permissible values are PreChecking, Failed, Ready, Deleting.
	//
	// +kubebuilder:validation:Enum={PreChecking,Failed,Ready,Deleting}
	// +optional
	Phase BackupRepoPhase `json:"phase,omitempty"`

	// Provides a detailed description of the current state of the backup repository.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Represents the latest generation of the resource that the controller has observed.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Refers to the generated secret for the `StorageProvider`.
	//
	// +optional
	GeneratedCSIDriverSecret *corev1.SecretReference `json:"generatedCSIDriverSecret,omitempty"`

	// Represents the name of the generated storage class.
	//
	// +optional
	GeneratedStorageClassName string `json:"generatedStorageClassName,omitempty"`

	// Represents the name of the PVC that stores backup data.
	//
	// +optional
	BackupPVCName string `json:"backupPVCName,omitempty"`

	// Represents the name of the secret that contains the configuration for the tool.
	//
	// +optional
	ToolConfigSecretName string `json:"toolConfigSecretName,omitempty"`

	// Indicates if this backup repository is the default one.\
	//
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

// BackupRepo is a repository for storing backup data.
type BackupRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupRepoSpec   `json:"spec,omitempty"`
	Status BackupRepoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupRepoList contains a list of `BackupRepo`.
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
