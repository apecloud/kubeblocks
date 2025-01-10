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

// ParamConfigRenderer is the Schema for the paramconfigrenderers API
type ParamConfigRenderer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParamConfigRendererSpec   `json:"spec,omitempty"`
	Status ParamConfigRendererStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParamConfigRendererList contains a list of ParamConfigRenderer
type ParamConfigRendererList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ParamConfigRenderer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ParamConfigRenderer{}, &ParamConfigRendererList{})
}

// ParamConfigRendererSpec defines the desired state of ParamConfigRenderer
type ParamConfigRendererSpec struct {
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

	// Specifies the name of the referenced componentTemplateSpec.
	//
	// +optional
	TemplateName string `json:"templateName"`

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

// ParamConfigRendererStatus defines the observed state of ParamConfigRenderer
type ParamConfigRendererStatus struct {
	// The most recent generation number of the ParamsDesc object that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Specifies the status of the configuration template.
	// When set to PDAvailablePhase, the ParamsDesc can be referenced by ComponentDefinition.
	//
	// +optional
	Phase ParametersDescPhase `json:"phase,omitempty"`
}
