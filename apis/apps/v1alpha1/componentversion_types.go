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
	// Components is a list of component instances within this ComponentVersion.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c.componentDef == x.componentDef)) == 1)",message="Duplicated component"
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(x, size(self.filter(c, c.componentDef == x.componentDef)) == 1)",message="Component can not be deleted"
	Components []ComponentInstance `json:"components"`
}

// ComponentInstance represents an instance of a component within a ComponentVersion.
type ComponentInstance struct {
	// CompDefinition is the reference to the ComponentDefinition of this component.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CompDefinition is immutable"
	CompDefinition string `json:"compDefinition"`

	// Apps is a list of applications within this component.
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c.name == x.name)) == 1)",message="Duplicated component app"
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(x, size(self.filter(c, c.name == x.name)) == 1)",message="App can not be deleted"
	// +optional
	Apps []ComponentApp `json:"apps"`
}

// ComponentApp represents an application within a component.
type ComponentApp struct {
	// Name is the name of the application, it indicates the name of container within the referred ComponentDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Name is immutable"
	Name string `json:"name"`

	// AppVersions is a list of versions associated with this application.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +kubebuilder:validation:XValidation:rule="self.all(x, size(self.filter(c, c == x)) == 1)",message="Duplicated app version"
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(x, x in self)",message="AppVersion may only be added"
	AppVersions []ComponentAppVersion `json:"appVersions"`
}

// ComponentAppVersion represents the version information for a specific application.
type ComponentAppVersion struct {
	// Version is the version number of the application.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Version is immutable"
	Version string `json:"version"`

	// Image is the container image associated with this application version.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Image is immutable"
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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
