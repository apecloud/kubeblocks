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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentVersionSpec defines the desired state of ComponentVersion
type ComponentVersionSpec struct {
	// CompatibilityRules defines compatibility rules between sets of component definitions and releases.
	// +kubebuilder:validation:Required
	CompatibilityRules []ComponentVersionCompatibilityRule `json:"compatibilityRules"`

	// Releases represents different releases of component instances within this ComponentVersion.
	// +kubebuilder:validation:Required
	Releases []ComponentVersionRelease `json:"releases"`
}

// ComponentVersionCompatibilityRule defines the compatibility between a set of component definitions and a set of releases.
type ComponentVersionCompatibilityRule struct {
	// CompDefs specifies names for the component definitions associated with this ComponentVersion.
	// Each name in the list can represent an exact name, or a name with wildcards.
	// Examples:
	// - "mysql-8.0.30-v1alpha1": Matches the exact name "mysql-8.0.30-v1alpha1"
	// - "mysql-8.0.30-*": Matches all names starting with "mysql-8.0.30-"
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	CompDefs []string `json:"compDefs"`

	// Versions is a list of version identifiers for the releases.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Versions []string `json:"versions"`
}

// ComponentVersionRelease represents a release of component instances within a ComponentVersion.
type ComponentVersionRelease struct {
	// Version is a unique identifier for this release.
	// If used, it will serve as the service version for component instances, overriding the one defined in the component definition.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Version string `json:"version"`

	// Changes provides information about the changes made in this release.
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Changes string `json:"changes,omitempty"`

	// Apps represents different application versions within this release.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	Apps []ComponentAppVersion `json:"apps,omitempty"`
}

// ComponentAppVersion represents the version information for a specific application.
type ComponentAppVersion struct {
	// Name is the name of the application, indicating the container name within the referenced ComponentDefinition.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Name string `json:"name"`

	// Version is the version number of the application.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	Version string `json:"version"`

	// Image is the container image associated with this application version.
	// Cannot be updated.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	Image string `json:"image"`
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
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cmpv

// ComponentVersion is the Schema for the componentversions API
type ComponentVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentVersionSpec   `json:"spec,omitempty"`
	Status ComponentVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentVersionList contains a list of ComponentVersion
type ComponentVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentVersion{}, &ComponentVersionList{})
}
