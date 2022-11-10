/*
Copyright 2022 The KubeBlocks Authors

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CustomParametersValidation struct {
	// TODO(zt) DAY2 support schema
	// Schema provides a way for ISVs to verify the validity of user change parameters through json schema
	// controller-gen doesn't work with k8s.io/apiextensions-apiserver: https://github.com/kubernetes-sigs/controller-tools/issues/291
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`

	// Cue that to let ISV verify user configuration through cue language
	// +optional
	Cue *string `json:"cue,omitempty"`
}

// ConfigurationTemplateSpec defines the desired state of ConfigurationTemplate
type ConfigurationTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ConfigurationTemplate. Edit configurationtemplate_types.go to remove/update
	// Foo string `json:"foo,omitempty"`

	// TplRef is a reference to the configmap object, the configmap determines how to generate the configurations
	// +kubebuilder:validation:Required
	TplRef string `json:"tplRef,omitempty"`

	// CfgSchemaTopLevelName is cue type name, which generate openapi schema
	// +kubebuilder:validation:Optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// ConfigurationSchema that impose restrictions on engine parameter's rule
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// StaticParameters require db instance restart
	// +kubebuilder:validation:Optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// DynamicParameters support reload
	// +kubebuilder:validation:Optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// ImmutableParameters describe not modify parameters by user
	// +kubebuilder:validation:Optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// UpgradeMode describe parameter update mode
	// +kubebuilder:default:Enum=dynamic
	// +kubebuilder:validation:Enum={dynamic,static}
	// +kubebuilder:validation:Optional
	UpgradeMode UpdateMode `json:"upgradeMode,omitempty"`

	// +kubebuilder:default:Enum=yaml
	// +kubebuilder:validation:Enum={dotenv,ini,yaml,json,hcl}
	Formatter ConfigurationFormatter `json:"formatter,omitempty"`

	// Immutable, if set to true, ensures that data stored in the ConfigMap cannot be updated (only object metadata can be modified).
	// If set to true, Configmap object referenced by TplRef will also be modified to immutable
	// Defaulted to true
	// It is recommended to turn this option on only during the development or testing phase.
	// +kubebuilder:default:true
	Immutable bool `json:"immutable,omitempty"`
}

// ConfigurationTemplateStatus defines the observed state of ConfigurationTemplate
type ConfigurationTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase is configuration template status, if it is set to AvailablePhase,
	// ConfigurationTemplate be used by ClusterDefinition or AppVersion
	// +kubebuilder:validation:Enum={Available,Unavailable,Deleting}
	// +kubebuilder:validation:Optional
	Phase Phase `json:"phase,omitempty"`

	// +kubebuilder:validation:Optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the most recent generation observed for this
	// ClusterDefinition. It corresponds to the ClusterDefinition's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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

// ConfigurationTemplateList contains a list of ConfigurationTemplate
type ConfigurationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigurationTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurationTemplate{}, &ConfigurationTemplateList{})
}
