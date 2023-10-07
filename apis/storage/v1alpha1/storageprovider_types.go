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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageProviderSpec defines the desired state of StorageProvider
type StorageProviderSpec struct {
	// The name of the CSI driver used by this StorageProvider.
	// +optional
	CSIDriverName string `json:"csiDriverName,omitempty"`

	// A Go template for rendering a secret which will be used by the CSI driver.
	// The template will be rendered with the following variables:
	// - Parameters: a map of parameters defined in the ParametersSchema.
	// +optional
	CSIDriverSecretTemplate string `json:"csiDriverSecretTemplate,omitempty"`

	// A Go template for rendering a storage class which will be used by the CSI driver.
	// The template will be rendered with the following variables:
	// - Parameters: a map of parameters defined in the ParametersSchema.
	// - CSIDriverSecretRef: the reference of the secret created by the CSIDriverSecretTemplate.
	// +optional
	StorageClassTemplate string `json:"storageClassTemplate,omitempty"`

	// A Go template for rendering a PersistentVolumeClaim.
	// The template will be rendered with the following variables:
	// - Parameters: a map of parameters defined in the ParametersSchema.
	// - GeneratedStorageClassName: the name of the storage class generated with the StorageClassTemplate.
	// +optional
	PersistentVolumeClaimTemplate string `json:"persistentVolumeClaimTemplate,omitempty"`

	// A Go template for rendering a config used by the datasafed command.
	// The template will be rendered with the following variables:
	// - Parameters: a map of parameters defined in the ParametersSchema.
	// +optional
	DatasafedConfigTemplate string `json:"datasafedConfigTemplate,omitempty"`

	// The schema describes the parameters required by this StorageProvider,
	// when rendering the templates.
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`
}

// ParametersSchema describes the parameters used by this StorageProvider.
type ParametersSchema struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
	// +optional
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`

	// credentialFields are the fields used to generate the secret.
	// +optional
	CredentialFields []string `json:"credentialFields,omitempty"`
}

// StorageProviderStatus defines the observed state of StorageProvider
type StorageProviderStatus struct {
	// Storage provider reconciliation phases. Valid values are NotReady, Ready.
	// +kubebuilder:validation:Enum={NotReady,Ready}
	Phase StorageProviderPhase `json:"phase,omitempty"`

	// Describes the current state of the storage provider.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="CSIDRIVER",type="string",JSONPath=".spec.csiDriverName"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// StorageProvider is the Schema for the storageproviders API
// StorageProvider describes how to provision PVCs for a specific storage system (e.g. S3, NFS, etc),
// by using the CSI driver.
type StorageProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageProviderSpec   `json:"spec,omitempty"`
	Status StorageProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorageProviderList contains a list of StorageProvider
type StorageProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageProvider{}, &StorageProviderList{})
}
