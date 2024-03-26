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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigConstraintSpec defines the desired state of ConfigConstraint
type ConfigConstraintSpec struct {
	// Specifies the dynamic reload actions supported by the engine. If set, the controller call the scripts defined in the actions for a dynamic parameter upgrade.
	// The actions are called only when the modified parameter is defined in dynamicParameters part && DynamicReloadAction != nil
	//
	// +optional
	DynamicReloadAction *DynamicReloadAction `json:"dynamicReloadAction,omitempty"`

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
	DynamicParameterSelectedPolicy *DynamicParameterSelectedPolicy `json:"dynamicParameterSelectedPolicy,omitempty"`

	// Tools used by the dynamic reload actions.
	// Usually it is referenced by the 'init container' for 'cp' it to a binary volume.
	//
	// +optional
	ReloadToolsImage *ReloadToolsImage `json:"reloadToolsImage,omitempty"`

	// A set of actions for regenerating local configs.
	//
	// It works when:
	// - different engine roles have different config, such as redis primary & secondary
	// - after a role switch, the local config will be regenerated with the help of DownwardActions
	//
	// +optional
	DownwardActions []DownwardAction `json:"downwardActions,omitempty"`

	// A list of ScriptConfig used by the actions defined in dynamic reload and downward actions.
	//
	// +optional
	// +patchMergeKey=scriptConfigMapRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=scriptConfigMapRef
	ScriptConfigs []ScriptConfig `json:"scriptConfigs,omitempty"`

	// Top level key used to get the cue rules to validate the config file.
	// It must exist in 'ConfigSchema'
	//
	// +optional
	ConfigSchemaTopLevelKey string `json:"configSchemaTopLevelKey,omitempty"`

	// List constraints rules for each config parameters.
	//
	// +optional
	ConfigSchema *ConfigSchema `json:"configSchema,omitempty"`

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
	DynamicReloadSelector *metav1.LabelSelector `json:"dynamicReloadSelector,omitempty"`

	// Describes the format of the config file.
	// The controller works as follows:
	// 1. Parse the config file
	// 2. Get the modified parameters
	// 3. Trigger the corresponding action
	//
	// +kubebuilder:validation:Required
	FormatterConfig *FormatterConfig `json:"formatterConfig"`
}

// Represents the observed state of a ConfigConstraint.

type ConfigConstraintStatus struct {

	// Specifies the status of the configuration template.
	// When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition or ClusterVersion.
	//
	// +optional
	Phase ConfigConstraintPhase `json:"phase,omitempty"`

	// Provides descriptions for abnormal states.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Refers to the most recent generation observed for this ConfigConstraint. This value is updated by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func (cs ConfigConstraintStatus) IsConfigConstraintTerminalPhases() bool {
	return cs.Phase == CCAvailablePhase
}

type ConfigSchema struct {
	// Transforms the schema from CUE to json for further OpenAPI validation
	//
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:ComponentDefRef=object
	// +kubebuilder:pruning:PreserveUnknownFields
	SchemaInJson *apiext.JSONSchemaProps `json:"schemaInJson,omitempty"`

	// Enables providers to verify user configurations using the CUE language.
	//
	// +optional
	CUE string `json:"cue,omitempty"`
}

// Defines the options for reloading a service or application within the Kubernetes cluster.
// Only one of its members may be specified at a time.

type DynamicReloadAction struct {
	// Used to trigger a reload by sending a Unix signal to the process.
	//
	// +optional
	UnixSignalTrigger *UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// Used to perform the reload command in shell script.
	//
	// +optional
	ShellTrigger *ShellTrigger `json:"shellTrigger,omitempty"`

	// Used to perform the reload command by Go template script.
	//
	// +optional
	TPLScriptTrigger *TPLScriptTrigger `json:"tplScriptTrigger"`

	// Used to automatically perform the reload command when conditions are met.
	//
	// +optional
	AutoTrigger *AutoTrigger `json:"autoTrigger,omitempty"`
}

type UnixSignalTrigger struct {
	// Represents a valid Unix signal.
	// Refer to the following URL for a list of all Unix signals: ../../pkg/configuration/configmap/handler.go:allUnixSignals
	//
	// +kubebuilder:validation:Required
	Signal SignalType `json:"signal"`

	// Represents the name of the process that the Unix signal sent to.
	//
	// +kubebuilder:validation:Required
	ProcessName string `json:"processName"`
}

type ReloadToolsImage struct {
	// Represents the name of the volume in the PodTemplate. This is where to mount the generated by the config template.
	// This volume name must be defined within podSpec.containers[*].volumeMounts.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// VolumeName string `json:"volumeName"`

	// Represents the point where the scripts file will be mounted.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	MountPoint string `json:"mountPoint"`

	// Used to configure the initialization container.
	//
	// +optional
	ToolConfigs []ToolConfig `json:"toolConfigs,omitempty"`
}

type ToolConfig struct {
	// Specifies the name of the initContainer.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`

	// Represents the url of the tool container image.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Commands to be executed when init containers.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

type DownwardAction struct {
	// Specifies the name of the field. It must be a string of maximum length 63.
	// The name should match the regex pattern `^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the mount point of the scripts file.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	MountPoint string `json:"mountPoint"`

	// Represents a list of downward API volume files.
	//
	// +kubebuilder:validation:Required
	Items []corev1.DownwardAPIVolumeFile `json:"items"`

	// The command used to execute for the downward API.
	//
	// +optional
	Command []string `json:"command,omitempty"`
}

type ScriptConfig struct {
	// Specifies the reference to the ConfigMap that contains the script to be executed for reload.
	//
	// +kubebuilder:validation:Required
	ScriptConfigMapRef string `json:"scriptConfigMapRef"`

	// Specifies the namespace where the referenced tpl script ConfigMap in.
	// If left empty, by default in the "default" namespace.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type ShellTrigger struct {
	// Specifies the list of commands for reload.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// Specifies whether to synchronize updates parameters to the config manager.
	// Specifies two ways of controller to reload the parameter:
	// - set to 'True', execute the reload action in sync mode, wait for the completion of reload
	// - set to 'False', execute the reload action in async mode, just update the 'Configmap', no need to wait
	//
	// +optional
	Sync *bool `json:"sync,omitempty"`
}

type TPLScriptTrigger struct {
	// Config for the script.
	//
	ScriptConfig `json:",inline"`

	// Specifies whether to synchronize updates parameters to the config manager.
	// Specifies two ways of controller to reload the parameter:
	// - set to 'True', execute the reload action in sync mode, wait for the completion of reload
	// - set to 'False', execute the reload action in async mode, just update the 'Configmap', no need to wait
	//
	// +optional
	Sync *bool `json:"sync,omitempty"`
}

type AutoTrigger struct {
	// The name of the process.
	//
	// +optional
	ProcessName string `json:"processName,omitempty"`
}

type FormatterConfig struct {
	// Represents the additional actions for formatting the config file.
	// If not specified, the default options will be applied.
	//
	// +optional
	FormatterAction `json:",inline"`

	// The config file format. Valid values are `ini`, `xml`, `yaml`, `json`,
	// `hcl`, `dotenv`, `properties` and `toml`. Each format has its own characteristics and use cases.
	//
	// - ini: is a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: https://en.wikipedia.org/wiki/INI_file
	// - xml: refers to wiki: https://en.wikipedia.org/wiki/XML
	// - yaml: supports for complex data types and structures.
	// - json: refers to wiki: https://en.wikipedia.org/wiki/JSON
	// - hcl: The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: https://www.linode.com/docs/guides/introduction-to-hcl/
	// - dotenv: is a plain text file with simple key–value pairs, reference wiki: https://en.wikipedia.org/wiki/Configuration_file#MS-DOS
	// - properties: a file extension mainly used in Java, reference wiki: https://en.wikipedia.org/wiki/.properties
	// - toml: refers to wiki: https://en.wikipedia.org/wiki/TOML
	// - props-plus: a file extension mainly used in Java, supports CamelCase(e.g: brokerMaxConnectionsPerIp)
	//
	// +kubebuilder:validation:Required
	Format CfgFileFormat `json:"format"`
}

// Encapsulates the unique options for a configuration file.
// It is important to note that only one of its members can be specified at a time.

type FormatterAction struct {

	// A pointer to an IniConfig struct that holds the ini options.
	//
	// +optional
	IniConfig *IniConfig `json:"iniConfig,omitempty"`

	// A pointer to an XMLConfig struct that holds the xml options.
	// XMLConfig *XMLConfig `json:"xmlConfig,omitempty"`
}

// Encapsulates the section name of an ini configuration.

type IniConfig struct {

	// A string that describes the name of the ini section.
	//
	// +optional
	SectionName string `json:"sectionName,omitempty"`
}

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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
