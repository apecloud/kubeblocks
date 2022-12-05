/*
Copyright ApeCloud Inc.

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
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigurationTemplateSpec defines the desired state of ConfigurationTemplate
type ConfigurationTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster

	// TplRef is a reference to the configmap object, the configmap determines how to generate the configurations.
	// +kubebuilder:validation:Required
	TplRef string `json:"tplRef,omitempty"`

	// CfgSchemaTopLevelName is cue type name, which generates openapi schema.
	// +optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// ConfigurationSchema imposes restrictions on database parameter's rule.
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// StaticParameters, list of StaticParameter, modifications of them trigger a process restart.
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// DynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// ImmutableParameters describes parameters that prohibit user from modification.
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// Formatter describes the format of the configuration file, the controller
	// 1. parses configuration file
	// 2. analyzes the modified parameters
	// 3. applies corresponding policies.
	// +kubebuilder:default:Enum=yaml
	// +kubebuilder:validation:Enum={dotenv,ini,yaml,json,hcl}
	Formatter ConfigurationFormatter `json:"formatter,omitempty"`
}

// ConfigurationTemplateStatus defines the observed state of ConfigurationTemplate.
type ConfigurationTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is status of configuration template, when set to AvailablePhase, it can be referenced by ClusterDefinition or AppVersion.
	// +kubebuilder:validation:Enum={Available,Unavailable,Deleting}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Message field describes the reasons of abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It refers to the ConfigurationTemplate's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type CustomParametersValidation struct {
	// Schema provides a way for providers to validate the changed parameters through json.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`

	// Cue that to let provider verify user configuration through cue language.
	// +optional
	Cue *string `json:"cue,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas, all},scope=Namespaced,shortName=ctpl
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigurationTemplate is the Schema for the configurationtemplates API
type ConfigurationTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationTemplateSpec   `json:"spec,omitempty"`
	Status ConfigurationTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigurationTemplateList contains a list of ConfigurationTemplates.
type ConfigurationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigurationTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurationTemplate{}, &ConfigurationTemplateList{})
}
