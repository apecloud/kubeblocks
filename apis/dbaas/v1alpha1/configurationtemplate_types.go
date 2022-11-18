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

	// ConfigurationSchema that imposes restrictions on engine parameter's rule.
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// StaticParameters, list of StaticParameter, modification of it triggers a process restart.
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// DynamicParameters, list of DynamicParameter, modification of it triggers a config dynamic reload without process restart.
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// ImmutableParameters describes parameters that prohibit user from modification.
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// Node: static parameters and unknown parameters are all in StaticParameters.
	// UpgradeMode describes parameter update mode.
	// For ISVs, it's impossible to enumerate all parameters, and when the user modify unknown parameters, or ISVs does not configure StaticParameters or DynamicParameters field,
	// these parameters should be treated as dynamic parameter or static parameter?
	// if it is treated as static parameter, the engine instance will be restarted, otherwise it will be reloaded.
	// +kubebuilder:default:Enum=dynamic
	// +kubebuilder:validation:Enum={dynamic,static}
	// +optional
	// UpgradeMode UpdateMode `json:"upgradeMode,omitempty"`

	// Formatter describes the format of the configuration file,
	// the controller parses configuration file based on formatter, and analyzes the modified parameters list,
	// and then applies corresponding policies.
	// +kubebuilder:default:Enum=yaml
	// +kubebuilder:validation:Enum={dotenv,ini,yaml,json,hcl}
	Formatter ConfigurationFormatter `json:"formatter,omitempty"`

	// Node: remove immutable
	// The configuration template is common, if it is modified incorrectly, it will affect all clusters using this configuration,
	// we do not recommend modifying the template that is already online, but modifications are inevitable during development or CICD,
	// so this control parameter is provided.
	// if set to true, Configmap object referenced by TplRef will also be modified to immutable. defaulted to true.
	// +kubebuilder:default:true
	// Immutable bool `json:"immutable,omitempty"`
}

// ConfigurationTemplateStatus defines the observed state of ConfigurationTemplate.
type ConfigurationTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is configuration template status, if it is set to AvailablePhase,
	// ConfigurationTemplate be used by ClusterDefinition or AppVersion.
	// +kubebuilder:validation:Enum={Available,Unavailable,Deleting}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// If the configuration template is incorrect, Message field describes the error information.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It corresponds to the ConfigurationTemplate's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type CustomParametersValidation struct {
	// TODO(zt) DAY2 support schema
	// Schema provides a way for providers to verify that the parameter is legal.
	// fix controller-gen doesn't work with k8s.io/apiextensions-apiserver: https://github.com/kubernetes-sigs/controller-tools/issues/291
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`

	// Cue that to let ISV verify user configuration through cue language.
	// +optional
	Cue *string `json:"cue,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster,shortName=ctpl
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

// ConfigurationTemplateList contains a list of ConfigurationTemplate.
type ConfigurationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigurationTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurationTemplate{}, &ConfigurationTemplateList{})
}
