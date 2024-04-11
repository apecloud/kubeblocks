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
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
)

// ConfigConstraintSpec defines the desired state of ConfigConstraint
type ConfigConstraintSpec struct {
	// Specifies the dynamic reload actions supported by the engine. If set, the controller call the scripts defined in the actions for a dynamic parameter upgrade.
	// The actions are called only when the modified parameter is defined in dynamicParameters part && ReloadOptions != nil
	//
	// +optional
	ReloadOptions *ReloadOptions `json:"reloadOptions,omitempty"`

	// Indicates the dynamic reload action and restart action can be merged to a restart action.
	//
	// When a batch of parameters updates incur both restart & dynamic reload, it works as:
	// - set to true, the two actions merged to only one restart action
	// - set to false, the two actions cannot be merged, the actions executed in order [dynamic reload, restart]
	//
	// +optional
	DynamicActionCanBeMerged *bool `json:"dynamicActionCanBeMerged,omitempty"`

	// Specifies the policy for selecting the parameters of dynamic reload actions.
	//
	// +optional
	DynamicParameterSelectedPolicy *appsv1beta1.DynamicParameterSelectedPolicy `json:"dynamicParameterSelectedPolicy,omitempty"`

	// Tools used by the dynamic reload actions.
	// Usually it is referenced by the 'init container' for 'cp' it to a binary volume.
	//
	// +optional
	ToolsImageSpec *appsv1beta1.ReloadToolsImage `json:"toolsImageSpec,omitempty"`

	// A set of actions for regenerating local configs.
	//
	// It works when:
	// - different engine roles have different config, such as redis primary & secondary
	// - after a role switch, the local config will be regenerated with the help of DownwardActions
	//
	// +optional
	DownwardAPIOptions []appsv1beta1.DownwardAction `json:"downwardAPIOptions,omitempty"`

	// A list of ScriptConfig used by the actions defined in dynamic reload and downward actions.
	//
	// +optional
	// +patchMergeKey=scriptConfigMapRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=scriptConfigMapRef
	ScriptConfigs []appsv1beta1.ScriptConfig `json:"scriptConfigs,omitempty"`

	// Top level key used to get the cue rules to validate the config file.
	// It must exist in 'ConfigSchema'
	//
	// +optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// List constraints rules for each config parameters.
	//
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// A list of StaticParameter. Modifications of static parameters trigger a process restart.
	//
	// +listType=set
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// A list of DynamicParameter. Modifications of dynamic parameters trigger a reload action without process restart.
	//
	// +listType=set
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// Describes parameters that are prohibited to do any modifications.
	//
	// +listType=set
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// Used to match labels on the pod to do a dynamic reload
	//
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Describes the format of the config file.
	// The controller works as follows:
	// 1. Parse the config file
	// 2. Get the modified parameters
	// 3. Trigger the corresponding action
	//
	// +kubebuilder:validation:Required
	FormatterConfig *appsv1beta1.FormatterConfig `json:"formatterConfig"`
}

// Represents the observed state of a ConfigConstraint.

type ConfigConstraintStatus struct {

	// Specifies the status of the configuration template.
	// When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition.
	//
	// +optional
	Phase appsv1beta1.ConfigConstraintPhase `json:"phase,omitempty"`

	// Provides descriptions for abnormal states.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Refers to the most recent generation observed for this ConfigConstraint. This value is updated by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type CustomParametersValidation struct {
	// Transforms the schema from CUE to json for further OpenAPI validation
	//
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:ComponentDefRef=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`

	// Enables providers to verify user configurations using the CUE language.
	//
	// +optional
	CUE string `json:"cue,omitempty"`
}

// Defines the options for reloading a service or application within the Kubernetes cluster.
// Only one of its members may be specified at a time.

type ReloadOptions struct {
	// Used to trigger a reload by sending a Unix signal to the process.
	//
	// +optional
	UnixSignalTrigger *appsv1beta1.UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// Used to perform the reload command in shell script.
	//
	// +optional
	ShellTrigger *appsv1beta1.ShellTrigger `json:"shellTrigger,omitempty"`

	// Used to perform the reload command by Go template script.
	//
	// +optional
	TPLScriptTrigger *appsv1beta1.TPLScriptTrigger `json:"tplScriptTrigger"`

	// Used to automatically perform the reload command when conditions are met.
	//
	// +optional
	AutoTrigger *appsv1beta1.AutoTrigger `json:"autoTrigger,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cc
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigConstraint is the Schema for the configconstraint API
type ConfigConstraint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigConstraintSpec   `json:"spec,omitempty"`
	Status ConfigConstraintStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigConstraintList contains a list of ConfigConstraints.
type ConfigConstraintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigConstraint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigConstraint{}, &ConfigConstraintList{})
}
