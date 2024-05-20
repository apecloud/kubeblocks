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
	// Specifies the dynamic reload action supported by the engine.
	// When set, the controller executes the method defined here to execute hot parameter updates.
	//
	// Dynamic reloading is triggered only if both of the following conditions are met:
	//
	// 1. The modified parameters are listed in the `dynamicParameters` field.
	//    If `reloadStaticParamsBeforeRestart` is set to true, modifications to `staticParameters`
	//    can also trigger a reload.
	// 2. `reloadOptions` is set.
	//
	// If `reloadOptions` is not set or the modified parameters are not listed in `dynamicParameters`,
	// dynamic reloading will not be triggered.
	//
	// Example:
	// ```yaml
	// reloadOptions:
	//  tplScriptTrigger:
	//    namespace: kb-system
	//    scriptConfigMapRef: mysql-reload-script
	//    sync: true
	// ```
	// +optional
	ReloadOptions *ReloadOptions `json:"reloadOptions,omitempty"`

	// Indicates whether to consolidate dynamic reload and restart actions into a single restart.
	//
	// - If true, updates requiring both actions will result in only a restart, merging the actions.
	// - If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.
	//
	// This flag allows for more efficient handling of configuration changes by potentially eliminating
	// an unnecessary reload step.
	//
	// +optional
	DynamicActionCanBeMerged *bool `json:"dynamicActionCanBeMerged,omitempty"`

	// Configures whether the dynamic reload specified in `reloadOptions` applies only to dynamic parameters or
	// to all parameters (including static parameters).
	//
	// - false (default): Only modifications to the dynamic parameters listed in `dynamicParameters`
	//   will trigger a dynamic reload.
	// - true: Modifications to both dynamic parameters listed in `dynamicParameters` and static parameters
	//   listed in `staticParameters` will trigger a dynamic reload.
	//   The "true" option is for certain engines that require static parameters to be set
	//   via SQL statements before they can take effect on restart.
	//
	// +optional
	ReloadStaticParamsBeforeRestart *bool `json:"reloadStaticParamsBeforeRestart,omitempty"`

	// Specifies the tools container image used by ShellTrigger for dynamic reload.
	// If the dynamic reload action is triggered by a ShellTrigger, this field is required.
	// This image must contain all necessary tools for executing the ShellTrigger scripts.
	//
	// Usually the specified image is referenced by the init container,
	// which is then responsible for copy the tools from the image to a bin volume.
	// This ensures that the tools are available to the 'config-manager' sidecar.
	//
	// +optional
	ToolsImageSpec *appsv1beta1.ToolsSetup `json:"toolsImageSpec,omitempty"`

	// Specifies a list of actions to execute specified commands based on Pod labels.
	//
	// It utilizes the K8s Downward API to mount label information as a volume into the pod.
	// The 'config-manager' sidecar container watches for changes in the role label and dynamically invoke
	// registered commands (usually execute some SQL statements) when a change is detected.
	//
	// It is designed for scenarios where:
	//
	// - Replicas with different roles have different configurations, such as Redis primary & secondary replicas.
	// - After a role switch (e.g., from secondary to primary), some changes in configuration are needed
	//   to reflect the new role.
	//
	// +optional
	DownwardAPIOptions []appsv1beta1.DownwardAPIChangeTriggeredAction `json:"downwardAPIOptions,omitempty"`

	// A list of ScriptConfig Object.
	//
	// Each ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
	// The scripts are mounted as volumes and can be referenced and executed by the dynamic reload
	// and DownwardAction to perform specific tasks or configurations.
	//
	// +optional
	// +patchMergeKey=scriptConfigMapRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=scriptConfigMapRef
	ScriptConfigs []appsv1beta1.ScriptConfig `json:"scriptConfigs,omitempty"`

	// Specifies the top-level key in the 'configurationSchema.cue' that organizes the validation rules for parameters.
	// This key must exist within the CUE script defined in 'configurationSchema.cue'.
	//
	// +optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// Defines a list of parameters including their names, default values, descriptions,
	// types, and constraints (permissible values or the range of valid values).
	//
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// List static parameters.
	// Modifications to any of these parameters require a restart of the process to take effect.
	//
	// +listType=set
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// List dynamic parameters.
	// Modifications to these parameters trigger a configuration reload without requiring a process restart.
	//
	// +listType=set
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// Lists the parameters that cannot be modified once set.
	// Attempting to change any of these parameters will be ignored.
	//
	// +listType=set
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// Used to match labels on the pod to determine whether a dynamic reload should be performed.
	//
	// In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
	// The `selector` allows you to specify label selectors to target the desired pods for the reload process.
	//
	// If the `selector` is not specified or is nil, all pods managed by the workload will be considered for the dynamic
	// reload.
	//
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Specifies the format of the configuration file and any associated parameters that are specific to the chosen format.
	// Supported formats include `ini`, `xml`, `yaml`, `json`, `hcl`, `dotenv`, `properties`, and `toml`.
	//
	// Each format may have its own set of parameters that can be configured.
	// For instance, when using the `ini` format, you can specify the section name.
	//
	// Example:
	// ```
	// formatterConfig:
	//  format: ini
	//  iniConfig:
	//    sectionName: mysqld
	// ```
	// +kubebuilder:validation:Required
	FormatterConfig *appsv1beta1.FileFormatConfig `json:"formatterConfig"`
}

// ConfigConstraintStatus represents the observed state of a ConfigConstraint.
type ConfigConstraintStatus struct {

	// Specifies the status of the configuration template.
	// When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition or ClusterVersion.
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

// CustomParametersValidation Defines a list of configuration items with their names, default values, descriptions,
// types, and constraints.
type CustomParametersValidation struct {
	// Hold a string that contains a script written in CUE language that defines a list of configuration items.
	// Each item is detailed with its name, default value, description, type (e.g. string, integer, float),
	// and constraints (permissible values or the valid range of values).
	//
	// CUE (Configure, Unify, Execute) is a declarative language designed for defining and validating
	// complex data configurations.
	// It is particularly useful in environments like K8s where complex configurations and validation rules are common.
	//
	// This script functions as a validator for user-provided configurations, ensuring compliance with
	// the established specifications and constraints.
	//
	// +optional
	CUE string `json:"cue,omitempty"`

	// Generated from the 'cue' field and transformed into a JSON format.
	//
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:ComponentDefRef=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`
}

// ReloadOptions defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.
//
// Only one of the mechanisms can be specified at a time.
type ReloadOptions struct {
	// Used to trigger a reload by sending a specific Unix signal to the process.
	//
	// +optional
	UnixSignalTrigger *appsv1beta1.UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// Allows to execute a custom shell script to reload the process.
	//
	// +optional
	ShellTrigger *appsv1beta1.ShellTrigger `json:"shellTrigger,omitempty"`

	// Enables reloading process using a Go template script.
	//
	// +optional
	TPLScriptTrigger *appsv1beta1.TPLScriptTrigger `json:"tplScriptTrigger"`

	// Automatically perform the reload when specified conditions are met.
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

// ConfigConstraint manages the parameters across multiple configuration files contained in a single configure template.
// These configuration files should have the same format (e.g. ini, xml, properties, json).
//
// It provides the following functionalities:
//
// 1. **Parameter Value Validation**: Validates and ensures compliance of parameter values with defined constraints.
// 2. **Dynamic Reload on Modification**: Monitors parameter changes and triggers dynamic reloads to apply updates.
// 3. **Parameter Rendering in Templates**: Injects parameters into templates to generate up-to-date configuration files.
type ConfigConstraint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigConstraintSpec   `json:"spec,omitempty"`
	Status ConfigConstraintStatus `json:"status,omitempty"`
}

// ConfigConstraintList contains a list of ConfigConstraints.
//
// +kubebuilder:object:root=true
type ConfigConstraintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigConstraint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigConstraint{}, &ConfigConstraintList{})
}
