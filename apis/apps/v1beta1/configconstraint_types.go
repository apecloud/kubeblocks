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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigConstraintSpec defines the desired state of ConfigConstraint
type ConfigConstraintSpec struct {
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
	// Example:
	// ```yaml
	// dynamicReloadAction:
	//  tplScriptTrigger:
	//    namespace: kb-system
	//    scriptConfigMapRef: mysql-reload-script
	//    sync: true
	// ```
	//
	// +optional
	ReloadAction *ReloadAction `json:"reloadAction,omitempty"`

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

	// TODO: migrate DownwardAPITriggeredActions to ComponentDefinition.spec.lifecycleActions
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
	DownwardAPIChangeTriggeredActions []DownwardAPIChangeTriggeredAction `json:"downwardAPIChangeTriggeredActions,omitempty"`

	// Defines a list of parameters including their names, default values, descriptions,
	// types, and constraints (permissible values or the range of valid values).
	//
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

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
}

// ConfigConstraintStatus represents the observed state of a ConfigConstraint.
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

// ReloadAction defines the mechanisms available for dynamically reloading a process within K8s without requiring a restart.
//
// Only one of the mechanisms can be specified at a time.
type ReloadAction struct {
	// Used to trigger a reload by sending a specific Unix signal to the process.
	//
	// +optional
	UnixSignalTrigger *UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// Allows to execute a custom shell script to reload the process.
	//
	// +optional
	ShellTrigger *ShellTrigger `json:"shellTrigger,omitempty"`

	// Enables reloading process using a Go template script.
	//
	// +optional
	TPLScriptTrigger *TPLScriptTrigger `json:"tplScriptTrigger"`

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

// UnixSignalTrigger is used to trigger a reload by sending a specific Unix signal to the process.
type UnixSignalTrigger struct {
	// Specifies a valid Unix signal to be sent.
	// For a comprehensive list of all Unix signals, see: ../../pkg/configuration/configmap/handler.go:allUnixSignals
	//
	// +kubebuilder:validation:Required
	Signal SignalType `json:"signal"`

	// Identifies the name of the process to which the Unix signal will be sent.
	//
	// +kubebuilder:validation:Required
	ProcessName string `json:"processName"`
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

	// Specifies the command to be executed by the init container.
	//
	// +optional
	Command []string `json:"command,omitempty"`
}

// DownwardAPIChangeTriggeredAction defines an action that triggers specific commands in response to changes in Pod labels.
// For example, a command might be executed when the 'role' label of the Pod is updated.
type DownwardAPIChangeTriggeredAction struct {
	// Specifies the name of the field. It must be a string of maximum length 63.
	// The name should match the regex pattern `^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the mount point of the Downward API volume.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	MountPoint string `json:"mountPoint"`

	// Represents a list of files under the Downward API volume.
	//
	// +kubebuilder:validation:Required
	Items []corev1.DownwardAPIVolumeFile `json:"items"`

	// Specifies the command to be triggered when changes are detected in Downward API volume files.
	// It relies on the inotify mechanism in the config-manager sidecar to monitor file changes.
	//
	// +optional
	Command []string `json:"command,omitempty"`

	// ScriptConfig object specifies a ConfigMap that contains script files that should be mounted inside the pod.
	// The scripts are mounted as volumes and can be referenced and executed by the DownwardAction to perform specific tasks or configurations.
	//
	// +optional
	ScriptConfig *ScriptConfig `json:"scriptConfig,omitempty"`
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

	// Controls whether parameter updates are processed individually or collectively in a batch:
	//
	// - 'True': Processes all changes in one batch reload.
	// - 'False': Processes each change individually.
	//
	// Defaults to 'False' if unspecified.
	//
	// +optional
	BatchReload *bool `json:"batchReload,omitempty"`

	// Specifies a Go template string for formatting batch input data.
	// It's used when `batchReload` is 'True' to format data passed into STDIN of the script.
	// The template accesses key-value pairs of updated parameters via the '$' variable.
	// This allows for custom formatting of the input data.
	//
	// Example template:
	//
	// ```yaml
	// batchParamsFormatterTemplate: |-
	// {{- range $pKey, $pValue := $ }}
	// {{ printf "%s:%s" $pKey $pValue }}
	// {{- end }}
	// ```
	//
	// This example generates batch input data in a key:value format, sorted by keys.
	// ```
	// key1:value1
	// key2:value2
	// key3:value3
	// ```
	//
	// If not specified, the default format is key=value, sorted by keys, for each updated parameter.
	// ```
	// key1=value1
	// key2=value2
	// key3=value3
	// ```
	//
	// +optional
	BatchParamsFormatterTemplate string `json:"batchParamsFormatterTemplate,omitempty"`

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

// TPLScriptTrigger Enables reloading process using a Go template script.
type TPLScriptTrigger struct {
	// Specifies the ConfigMap that contains the script to be executed for reload.
	//
	ScriptConfig `json:",inline"`

	// Determines whether parameter updates should be synchronized with the "config-manager".
	// Specifies the controller's reload strategy:
	//
	// - If set to 'True', the controller executes the reload action in synchronous mode,
	//   pausing execution until the reload completes.
	// - If set to 'False', the controller executes the reload action in asynchronous mode,
	//   updating the ConfigMap without waiting for the reload process to finish.
	//
	// +optional
	Sync *bool `json:"sync,omitempty"`
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

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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
