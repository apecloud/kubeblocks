/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName={paramsdef,pd}
// +kubebuilder:printcolumn:name="FILE",type="string",JSONPath=".spec.fileName",description="config file name"
// +kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ParametersDefinition is the Schema for the parametersdefinitions API
type ParametersDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParametersDefinitionSpec   `json:"spec,omitempty"`
	Status ParametersDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParametersDefinitionList contains a list of ParametersDefinition
type ParametersDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ParametersDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ParametersDefinition{}, &ParametersDefinitionList{})
}

// ParametersDefinitionSpec defines the desired state of ParametersDefinition
type ParametersDefinitionSpec struct {
	// Specifies the config file name in the config template.
	//
	// +optional
	FileName string `json:"fileName,omitempty"`

	// Defines a list of parameters including their names, default values, descriptions,
	// types, and constraints (permissible values or the range of valid values).
	//
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

	// Specifies the dynamic reload (dynamic reconfiguration) actions supported by the engine.
	// When set, the controller executes the scripts defined in these actions to handle dynamic parameter updates.
	//
	// Dynamic reloading is triggered only if both of the following conditions are met:
	//
	// 1. The modified parameters are listed in the `dynamicParameters` field.
	//    If `dynamicParameterSelectedPolicy` is set to "all", modifications to `staticParameters`
	//    can also trigger a reload.
	// 2. `reloadAction` is set.
	//
	// If `reloadAction` is not set or the modified parameters are not listed in `dynamicParameters`,
	// dynamic reloading will not be triggered.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 1.2.0"
	// +optional
	ReloadAction *ReloadAction `json:"reloadAction,omitempty"`

	// Specifies the policy when parameter be removed.
	//
	// +optional
	ParameterDeletedPolicy *ParameterDeletedPolicy `json:"deletedPolicy,omitempty"`

	// Indicates whether to consolidate dynamic reload and restart actions into a single restart.
	//
	// - If true, updates requiring both actions will result in only a restart, merging the actions.
	// - If false, updates will trigger both actions executed sequentially: first dynamic reload, then restart.
	//
	// This flag allows for more efficient handling of configuration changes by potentially eliminating
	// an unnecessary reload step.
	//
	// +optional
	MergeReloadAndRestart *bool `json:"mergeReloadAndRestart,omitempty"`

	// Configures whether the dynamic reload specified in `reloadAction` applies only to dynamic parameters or
	// to all parameters (including static parameters).
	//
	// - false (default): Only modifications to the dynamic parameters listed in `dynamicParameters`
	//   will trigger a dynamic reload.
	// - true: Modifications to both dynamic parameters listed in `dynamicParameters` and static parameters
	//   listed in `staticParameters` will trigger a dynamic reload.
	//   The "all" option is for certain engines that require static parameters to be set
	//   via SQL statements before they can take effect on restart.
	//
	// +optional
	ReloadStaticParamsBeforeRestart *bool `json:"reloadStaticParamsBeforeRestart,omitempty"`

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
}

type ParameterDeletedPolicy struct {

	// Specifies the method to handle the deletion of a parameter.
	// If set to "RestoreToDefault", the parameter will be restored to its default value,
	// which requires engine support, such as pg.
	// If set to "Reset", the parameter will be re-rendered through the configuration template.
	//
	// +kubebuilder:validation:Required
	DeletedMethod ParameterDeletedMethod `json:"deletedMethod"`

	// Specifies the value to use if DeletedMethod is RestoreToDefault.
	// Example: pg
	// SET configuration_parameter TO DEFAULT;
	//
	// +optional
	DefaultValue *string `json:"defaultValue,omitempty"`
}

// ParametersDefinitionStatus defines the observed state of ParametersDefinition
type ParametersDefinitionStatus struct {
	// The most recent generation number of the ParamsDesc object that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Specifies the status of the configuration template.
	// When set to PDAvailablePhase, the ParamsDesc can be referenced by ComponentDefinition.
	//
	// +optional
	Phase ParametersDescPhase `json:"phase,omitempty"`

	// Represents a list of detailed status of the ParametersDescription object.
	//
	// This field is crucial for administrators and developers to monitor and respond to changes within the ParametersDescription.
	// It provides a history of state transitions and a snapshot of the current state that can be used for
	// automated logic or direct inspection.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ReloadAction defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.
//
// Only one of the mechanisms can be specified at a time.
type ReloadAction struct {
	// Allows to execute a custom shell script to reload the process.
	//
	// +optional
	ShellTrigger *ShellTrigger `json:"shellTrigger,omitempty"`

	// Automatically perform the reload when specified conditions are met.
	//
	// +optional
	AutoTrigger *AutoTrigger `json:"autoTrigger,omitempty"`

	// Used to match labels on the pod to determine whether a dynamic reload should be performed.
	//
	// In some scenarios, only specific pods (e.g., primary replicas) need to undergo a dynamic reload.
	// The `reloadedPodSelector` allows you to specify label selectors to target the desired pods for the reload process.
	//
	// If the `reloadedPodSelector` is not specified or is nil, all pods managed by the workload will be considered for the dynamic
	// reload.
	//
	// +optional
	TargetPodSelector *metav1.LabelSelector `json:"targetPodSelector,omitempty"`
}

// ToolsSetup prepares the tools for dynamic reloads used in ShellTrigger from a specified container image.
//
// Example:
// ```yaml
//
//	toolsSetup:
//	 mountPoint: /kb_tools
//	 toolConfigs:
//	   - name: kb-tools
//	     command:
//	       - cp
//	       - /bin/ob-tools
//	       - /kb_tools/obtools
//	     image: docker.io/apecloud/obtools
//
// ```
// This example copies the "/bin/ob-tools" binary from the image to "/kb_tools/obtools".
type ToolsSetup struct {
	// Represents the name of the volume in the PodTemplate. This is where to mount the generated by the config template.
	// This volume name must be defined within podSpec.containers[*].volumeMounts.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// VolumeName string `json:"volumeName"`

	// Specifies the directory path in the container where the tools-related files are to be copied.
	// This field is typically used with an emptyDir volume to ensure a temporary, empty directory is provided at pod creation.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	MountPoint string `json:"mountPoint"`

	// Specifies a list of settings of init containers that prepare tools for dynamic reload.
	//
	// +optional
	ToolConfigs []ToolConfig `json:"toolConfigs,omitempty"`
}

// ToolConfig specifies the settings of an init container that prepare tools for dynamic reload.
type ToolConfig struct {
	// Specifies the name of the init container.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`

	// Indicates whether the tool image should be used as the container image for a sidecar.
	// This is useful for large tool images, such as those for C++ tools, which may depend on
	// numerous libraries (e.g., *.so files).
	//
	// If enabled, the tool image is deployed as a sidecar container image.
	//
	// Examples:
	// ```yaml
	//  toolsSetup::
	//    mountPoint: /kb_tools
	//    toolConfigs:
	//      - name: kb-tools
	//        asContainerImage: true
	//        image:  apecloud/oceanbase:4.2.0.0-100010032023083021
	// ```
	//
	// generated containers:
	// ```yaml
	// initContainers:
	//  - name: install-config-manager-tool
	//    image: apecloud/kubeblocks-tools:${version}
	//    command:
	//    - cp
	//    - /bin/config_render
	//    - /opt/tools
	//    volumemounts:
	//    - name: kb-tools
	//      mountpath: /opt/tools
	//
	// containers:
	//  - name: config-manager
	//    image: apecloud/oceanbase:4.2.0.0-100010032023083021
	//    imagePullPolicy: IfNotPresent
	// 	  command:
	//    - /opt/tools/reloader
	//    - --log-level
	//    - info
	//    - --operator-update-enable
	//    - --tcp
	//    - "9901"
	//    - --config
	//    - /opt/config-manager/config-manager.yaml
	//    volumemounts:
	//    - name: kb-tools
	//      mountpath: /opt/tools
	// ```
	//
	// +optional
	AsContainerImage *bool `json:"asContainerImage,omitempty"`

	// Specifies the tool container image.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Defines image mapping for different service versions.
	// +optional
	ImageMappings []ImageMapping `json:"imageMappings,omitempty"`

	// Specifies the command to be executed by the init container.
	//
	// +optional
	Command []string `json:"command,omitempty"`
}

type ImageMapping struct {
	// ServiceVersions is a list of service versions that this mapping applies to.
	// +kubebuilder:validation:Required
	ServiceVersions []string `json:"serviceVersions"`

	// Image is the container image addresses to use for the matched service versions.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
}

type ScriptConfig struct {
	// Specifies the reference to the ConfigMap containing the scripts.
	//
	// +kubebuilder:validation:Required
	ScriptConfigMapRef string `json:"scriptConfigMapRef"`

	// Specifies the namespace for the ConfigMap.
	// If not specified, it defaults to the "default" namespace.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ShellTrigger allows to execute a custom shell script to reload the process.
type ShellTrigger struct {
	// Specifies the command to execute in order to reload the process. It should be a valid shell command.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// Determines the synchronization mode of parameter updates with "config-manager".
	//
	// - 'True': Executes reload actions synchronously, pausing until completion.
	// - 'False': Executes reload actions asynchronously, without waiting for completion.
	//
	// +optional
	Sync *bool `json:"sync,omitempty"`

	// Specifies the tools container image used by ShellTrigger for dynamic reload.
	// If the dynamic reload action is triggered by a ShellTrigger, this field is required.
	// This image must contain all necessary tools for executing the ShellTrigger scripts.
	//
	// Usually the specified image is referenced by the init container,
	// which is then responsible for copy the tools from the image to a bin volume.
	// This ensures that the tools are available to the 'config-manager' sidecar.
	//
	// +optional
	ToolsSetup *ToolsSetup `json:"toolsSetup,omitempty"`

	// ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
	// The scripts are mounted as volumes and can be referenced and executed by the dynamic reload.
	//
	// +optional
	ScriptConfig *ScriptConfig `json:"scriptConfig,omitempty"`
}

// AutoTrigger automatically perform the reload when specified conditions are met.
type AutoTrigger struct {
	// The name of the process.
	//
	// +optional
	ProcessName string `json:"processName,omitempty"`
}

// FileFormatConfig specifies the format of the configuration file and any associated parameters
// that are specific to the chosen format.
type FileFormatConfig struct {
	// Each format may have its own set of parameters that can be configured.
	// For instance, when using the `ini` format, you can specify the section name.
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

// FormatterAction configures format-specific options for different configuration file format.
// Note: Only one of its members should be specified at any given time.
type FormatterAction struct {
	// Holds options specific to the 'ini' file format.
	//
	// +optional
	IniConfig *IniConfig `json:"iniConfig,omitempty"`

	// Holds options specific to the 'xml' file format.
	// XMLConfig *XMLConfig `json:"xmlConfig,omitempty"`
}

// IniConfig holds options specific to the 'ini' file format.
type IniConfig struct {
	// A string that describes the name of the ini section.
	//
	// +optional
	SectionName string `json:"sectionName,omitempty"`
}

// ParametersSchema Defines a list of configuration items with their names, default values, descriptions,
// types, and constraints.
type ParametersSchema struct {
	// Specifies the top-level key in the 'configSchema.cue' that organizes the validation rules for parameters.
	// This key must exist within the CUE script defined in 'configSchema.cue'.
	//
	// +optional
	TopLevelKey string `json:"topLevelKey,omitempty"`

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
	SchemaInJSON *apiext.JSONSchemaProps `json:"schemaInJSON,omitempty"`
}
