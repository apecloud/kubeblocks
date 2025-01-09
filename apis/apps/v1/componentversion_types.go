/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

// ComponentVersionCompatibilityRule defines the compatibility between a set of component definitions and a set of releases.
type ComponentVersionCompatibilityRule struct {
	// CompDefs specifies names for the component definitions associated with this ComponentVersion.
	// Each name in the list can represent an exact name, a name prefix, or a regular expression pattern.
	//
	// For example:
	//
	// - "mysql-8.0.30-v1alpha1": Matches the exact name "mysql-8.0.30-v1alpha1"
	// - "mysql-8.0.30": Matches all names starting with "mysql-8.0.30"
	// - "^mysql-8.0.\d{1,2}$": Matches all names starting with "mysql-8.0." followed by one or two digits.
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
	//
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
	//
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	// If the release is used, it will serve as the service version for component instances, overriding the one defined in the component definition.
	//
	// Cannot be updated.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	ServiceVersion string `json:"serviceVersion"`

	// Images define the new images for containers, actions or external applications within the release.
	//
	// If an image is specified for a lifecycle action, the key should be the field name (case-insensitive) of
	// the action in the LifecycleActions struct.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=128
	// +kubebuilder:validation:XValidation:rule="self.all(key, size(key) <= 32)",message="Container, action or external application name may not exceed maximum length of 32 characters"
	// +kubebuilder:validation:XValidation:rule="self.all(key, size(self[key]) <= 256)",message="Image name may not exceed maximum length of 256 characters"
	Images map[string]string `json:"images"`

	// Status represents the status of the release (e.g., "alpha", "beta", "stable", "deprecated").
	//
	// For a release in the "deprecated" state, the release is no longer supported and the controller will prevent new instances from employing it.
	//
	// +kubebuilder:validation:Required
	Status ReleaseStatus `json:"status"`

	// Reason is the detailed reason for the release status.
	//
	// For a release in the "deprecated" state, the reason should explain why the release is deprecated. For example:
	//   {
	//		Description: "Vulnerability in logging library",
	//		Severity:    "high",
	//		Mitigation:  "Upgrade to logging library v2.0.0",
	//		CVE:         "CVE-2024-12345",
	//   }
	//
	// +kubebuilder:validation:MaxLength=256
	// +optional
	Reason string `json:"reason,omitempty"`

	// ReleaseDate is the date when this version was released.
	//
	// +optional
	ReleaseDate time.Time `json:"releaseDate,omitempty"`

	// EndOfLifeDate is the date when this version is no longer supported.
	//
	// When this field is set and the release has passed its end-of-life (EOL) date, the controller will prevent new instances from employing it.
	//
	// +optional
	EndOfLifeDate time.Time `json:"endOfLifeDate,omitempty"`
}

// ReleaseStatus represents the status of a release.
//
// +kubebuilder:validation:Enum={alpha,beta,stable,deprecated}
type ReleaseStatus string

const (
	ReleaseStatusAlpha      ReleaseStatus = "alpha"
	ReleaseStatusBeta       ReleaseStatus = "beta"
	ReleaseStatusStable     ReleaseStatus = "stable"
	ReleaseStatusDeprecated ReleaseStatus = "deprecated"
)
