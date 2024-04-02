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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentVersionSpec defines the desired state of ComponentVersion
type ComponentVersionSpec struct {
	// CompatibilityRules defines compatibility rules between sets of component definitions and releases.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	CompatibilityRules []ComponentVersionCompatibilityRule `json:"compatibilityRules"`

	// Releases represents different releases of component instances within this ComponentVersion.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Releases []ComponentVersionRelease `json:"releases"`
}

// ComponentVersionCompatibilityRule defines the compatibility between a set of component definitions and a set of releases.
type ComponentVersionCompatibilityRule struct {
	// CompDefs specifies names for the component definitions associated with this ComponentVersion.
	// Each name in the list can represent an exact name, or a name prefix.
	//
	// For example:
	//
	// - "mysql-8.0.30-v1alpha1": Matches the exact name "mysql-8.0.30-v1alpha1"
	// - "mysql-8.0.30": Matches all names starting with "mysql-8.0.30"
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	CompDefs []string `json:"compDefs"`

	// Releases is a list of identifiers for the releases.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Releases []string `json:"releases"`
}

// ComponentVersionRelease represents a release of component instances within a ComponentVersion.
type ComponentVersionRelease struct {
	// Name is a unique identifier for this release.
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Name string `json:"name"`

	// Changes provides information about the changes made in this release.
	//
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Changes string `json:"changes,omitempty"`

	// ServiceVersion defines the version of the well-known service that the component provides.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	// If the release is used, it will serve as the service version for component instances, overriding the one defined in the component definition.
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	ServiceVersion string `json:"serviceVersion"`

	// Images define the new images for different containers within the release.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=128
	// +kubebuilder:validation:XValidation:rule="self.all(key, size(key) <= 32)",message="Container name may not exceed maximum length of 32 characters"
	// +kubebuilder:validation:XValidation:rule="self.all(key, size(self[key]) <= 256)",message="Image name may not exceed maximum length of 256 characters"
	Images map[string]string `json:"images"`
}

// ComponentVersionStatus defines the observed state of ComponentVersion
type ComponentVersionStatus struct {
	// ObservedGeneration is the most recent generation observed for this ComponentVersion.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase valid values are ``, `Available`, 'Unavailable`.
	// Available is ComponentVersion become available, and can be used for co-related objects.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Extra message for current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ServiceVersions represent the supported service versions of this ComponentVersion.
	// +optional
	ServiceVersions string `json:"serviceVersions,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cmpv
// +kubebuilder:printcolumn:name="Versions",type="string",JSONPath=".status.serviceVersions",description="service versions"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ComponentVersion is the Schema for the componentversions API
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty"`
	Status ComponentVersionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentVersionList contains a list of ComponentVersion
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentVersion{}, &ComponentVersionList{})
}
