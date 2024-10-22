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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ParameterDrivenConfigRenderSpec defines the desired state of ParameterDrivenConfigRender
type ParameterDrivenConfigRenderSpec struct {
	// Specifies the ComponentDefinition custom resource (CR) that defines the Component's characteristics and behavior.
	//
	// +kubebuilder:validation:Required
	ComponentDef string `json:"componentDef"`

	// ServiceVersion specifies the version of the Service expected to be provisioned by this Component.
	// The version should follow the syntax and semantics of the "Semantic Versioning" specification (http://semver.org/).
	// If no version is specified, the latest available version will be used.
	//
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Specifies the ParametersDefinition custom resource (CR) that defines the Component parameter's schema and behavior.
	//
	// +optional
	ParametersDefs []string `json:"parametersDefs,omitempty"`

	// Specifies the configuration files.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	Configs []ComponentConfigDescription `json:"configs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type ComponentConfigDescription struct {
	// Specifies the config file name in the config template.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
	// Supported formats include `ini`, `xml`, `yaml`, `json`, `hcl`, `dotenv`, `properties`, and `toml`.
	//
	// Each format may have its own set of parameters that can be configured.
	// For instance, when using the `ini` format, you can specify the section name.
	//
	// Example:
	// ```
	// fileFormatConfig:
	//  format: ini
	//  iniConfig:
	//    sectionName: mysqld
	// ```
	//
	// +kubebuilder:validation:Required
	FileFormatConfig *FileFormatConfig `json:"fileFormatConfig"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	//
	// +listType=set
	// +optional
	InjectEnvTo []string `json:"injectEnvTo,omitempty"`

	// Specifies whether the configuration needs to be re-rendered after v-scale or h-scale operations to reflect changes.
	//
	// In some scenarios, the configuration may need to be updated to reflect the changes in resource allocation
	// or cluster topology. Examples:
	//
	// - Redis: adjust maxmemory after v-scale operation.
	// - MySQL: increase max connections after v-scale operation.
	// - Zookeeper: update zoo.cfg with new node addresses after h-scale operation.
	//
	// +listType=set
	// +optional
	ReRenderResourceTypes []RerenderResourceType `json:"reRenderResourceTypes,omitempty"`
}

// ParameterDrivenConfigRenderStatus defines the observed state of ParameterDrivenConfigRender
type ParameterDrivenConfigRenderStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=paramstemplate
// +kubebuilder:printcolumn:name="COMPD",type="string",JSONPath=".spec.componentDef",description="componentdefinition name"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ParameterDrivenConfigRender is the Schema for the parameterdrivenconfigrenders API
type ParameterDrivenConfigRender struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParameterDrivenConfigRenderSpec   `json:"spec,omitempty"`
	Status ParameterDrivenConfigRenderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParameterDrivenConfigRenderList contains a list of ParameterDrivenConfigRender
type ParameterDrivenConfigRenderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ParameterDrivenConfigRender `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ParameterDrivenConfigRender{}, &ParameterDrivenConfigRenderList{})
}
