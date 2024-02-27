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
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigConstraintSpec defines the desired state of ConfigConstraint
type ConfigConstraintSpec struct {
	// Specifies whether the process supports reload. If set, the controller determines the behavior of the engine instance based on the configuration templates.
	// It will either restart or reload depending on whether any parameters in the StaticParameters have been modified.
	//
	// +optional
	ReloadOptions *ReloadOptions `json:"reloadOptions,omitempty"`

	// Indicates whether to execute hot update parameters when the pod needs to be restarted.
	// If set to true, the controller performs the hot update and then restarts the pod.
	//
	// +optional
	ForceHotUpdate *bool `json:"forceHotUpdate,omitempty"`

	// Used to configure the init container.
	//
	// +optional
	ToolsImageSpec *ToolsImageSpec `json:"toolsImageSpec,omitempty"`

	// ToolConfigs []ToolConfig `json:"toolConfigs,omitempty"`

	// Used to monitor pod fields.
	//
	// +optional
	DownwardAPIOptions []DownwardAPIOption `json:"downwardAPIOptions,omitempty"`

	// A list of ScriptConfig. These scripts can be used by volume trigger, downward trigger, or tool image.
	//
	// +optional
	// +patchMergeKey=scriptConfigMapRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=scriptConfigMapRef
	ScriptConfigs []ScriptConfig `json:"scriptConfigs,omitempty"`

	// The cue type name, which generates the openapi schema.
	//
	// +optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// Imposes restrictions on database parameter's rule.
	//
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// A list of StaticParameter. Modifications of these parameters trigger a process restart.
	//
	// +listType=set
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// A list of DynamicParameter. Modifications of these parameters trigger a config dynamic reload without process restart.
	//
	// +listType=set
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// Describes parameters that users are prohibited from modifying.
	//
	// +listType=set
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// Used to match the label on the pod. For example, a pod of the primary matches on the patroni cluster.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Describes the format of the configuration file. The controller will:
	// 1. Parse the configuration file
	// 2. Analyze the modified parameters
	// 3. Apply corresponding policies.
	//
	// +kubebuilder:validation:Required
	FormatterConfig *FormatterConfig `json:"formatterConfig"`
}

// Represents the observed state of a ConfigConstraint.

type ConfigConstraintStatus struct {

	// Specifies the status of the configuration template. When set to CCAvailablePhase, the ConfigConstraint can be referenced by ClusterDefinition or ClusterVersion.
	//
	// +optional
	Phase ConfigConstraintPhase `json:"phase,omitempty"`

	// Provides a description of any abnormal statuses that may be present.
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

type CustomParametersValidation struct {
	// Provides a mechanism that allows providers to validate the modified parameters using JSON.
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
	// Used to trigger a reload by sending a specific Unix signal to the process.
	//
	// +optional
	UnixSignalTrigger *UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// Used to perform the reload command via a shell script.
	//
	// +optional
	ShellTrigger *ShellTrigger `json:"shellTrigger,omitempty"`

	// Used to perform the reload command via a Go template script.
	//
	// +optional
	TPLScriptTrigger *TPLScriptTrigger `json:"tplScriptTrigger"`

	// Used to automatically perform the reload command when certain conditions are met.
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

	// Represents the name of the process to which the Unix signal is sent.
	//
	// +kubebuilder:validation:Required
	ProcessName string `json:"processName"`
}

type ToolsImageSpec struct {
	// Represents the name of the volume in the PodTemplate. This is where the configuration file, produced through the configuration template, will be mounted.
	// This volume name must be defined within podSpec.containers[*].volumeMounts.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=32
	// VolumeName string `json:"volumeName"`

	// Represents the location where the scripts file will be mounted.
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
	// Specifies the name of the initContainer. This must be a DNS_LABEL name.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`

	// Represents the name of the container image for the tools.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// Used to execute commands for init containers.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

type DownwardAPIOption struct {
	// Specifies the name of the field. This is a required field and must be a string of maximum length 63.
	// The name should match the regex pattern `^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the mount point of the scripts file. This is a required field and must be a string of maximum length 128.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=128
	MountPoint string `json:"mountPoint"`

	// Represents a list of downward API volume files. This is a required field.
	//
	// +kubebuilder:validation:Required
	Items []corev1.DownwardAPIVolumeFile `json:"items"`

	// The command used to execute for the downward API. This field is optional.
	//
	// +optional
	Command []string `json:"command,omitempty"`
}

type ScriptConfig struct {
	// Specifies the reference to the ConfigMap that contains the script to be executed for reload.
	//
	// +kubebuilder:validation:Required
	ScriptConfigMapRef string `json:"scriptConfigMapRef"`

	// Specifies the namespace where the referenced tpl script ConfigMap object resides.
	// If left empty, it defaults to the "default" namespace.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type ShellTrigger struct {
	// Specifies the list of strings used to execute for reload.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// Specifies whether to synchronize updates parameters to the config manager.
	//
	// +optional
	Sync *bool `json:"sync,omitempty"`
}

type TPLScriptTrigger struct {
	// The configuration for the script.
	ScriptConfig `json:",inline"`

	// Specifies whether to synchronize updates parameters to the config manager.
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
	// Represents the special options of the configuration file.
	// This is optional for now. If not specified, the default options will be used.
	//
	// +optional
	FormatterOptions `json:",inline"`

	// The configuration file format. Valid values are `ini`, `xml`, `yaml`, `json`,
	// `hcl`, `dotenv`, `properties` and `toml`. Each format has its own characteristics and use cases.
	//
	// - ini: a configuration file that consists of a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: https://en.wikipedia.org/wiki/INI_file
	// - xml: reference wiki: https://en.wikipedia.org/wiki/XML
	// - yaml: a configuration file support for complex data types and structures.
	// - json: reference wiki: https://en.wikipedia.org/wiki/JSON
	// - hcl: The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: https://www.linode.com/docs/guides/introduction-to-hcl/
	// - dotenv: this was a plain text file with simple key–value pairs, reference wiki: https://en.wikipedia.org/wiki/Configuration_file#MS-DOS
	// - properties: a file extension mainly used in Java, reference wiki: https://en.wikipedia.org/wiki/.properties
	// - toml: reference wiki: https://en.wikipedia.org/wiki/TOML
	// - props-plus: a file extension mainly used in Java, support CamelCase(e.g: brokerMaxConnectionsPerIp)
	//
	// +kubebuilder:validation:Required
	Format CfgFileFormat `json:"format"`
}

// Encapsulates the unique options for a configuration file.
// It is important to note that only one of its members can be specified at a time.

type FormatterOptions struct {

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

func (in *ConfigConstraintSpec) NeedForceUpdateHot() bool {
	if in.ForceHotUpdate != nil {
		return *in.ForceHotUpdate
	}
	return false
}
