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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

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
	// +kubebuilder:validation:Required
	StorageClassTemplate string `json:"storageClassTemplate,omitempty"`

	// The schema describes the parameters required by this StorageProvider,
	// when rendering the templates.
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`
}

// ParametersSchema describes the parameters used by this StorageProvider.
type ParametersSchema struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster

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
