/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigConstraintSpec defines the desired state of ConfigConstraint
type ConfigConstraintSpec struct {
	// reloadOptions indicates whether the process supports reload.
	// if set, the controller will determine the behavior of the engine instance based on the configuration templates,
	// restart or reload depending on whether any parameters in the StaticParameters have been modified.
	// +optional
	ReloadOptions *ReloadOptions `json:"reloadOptions,omitempty"`

	// cfgSchemaTopLevelName is cue type name, which generates openapi schema.
	// +optional
	CfgSchemaTopLevelName string `json:"cfgSchemaTopLevelName,omitempty"`

	// configurationSchema imposes restrictions on database parameter's rule.
	// +optional
	ConfigurationSchema *CustomParametersValidation `json:"configurationSchema,omitempty"`

	// staticParameters, list of StaticParameter, modifications of them trigger a process restart.
	// +listType=set
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.
	// +listType=set
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// immutableParameters describes parameters that prohibit user from modification.
	// +listType=set
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// selector is used to match the label on the pod,
	// for example, a pod of the primary is match on the patroni cluster.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// formatterConfig describes the format of the configuration file, the controller
	// 1. parses configuration file
	// 2. analyzes the modified parameters
	// 3. applies corresponding policies.
	// +kubebuilder:validation:Required
	FormatterConfig *FormatterConfig `json:"formatterConfig"`
}

// ConfigConstraintStatus defines the observed state of ConfigConstraint.
type ConfigConstraintStatus struct {
	// phase is status of configuration template, when set to CCAvailablePhase, it can be referenced by ClusterDefinition or ClusterVersion.
	// +optional
	Phase ConfigConstraintPhase `json:"phase,omitempty"`

	// message field describes the reasons of abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It refers to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

func (cs ConfigConstraintStatus) IsConfigConstraintTerminalPhases() bool {
	return cs.Phase == CCAvailablePhase
}

type CustomParametersValidation struct {
	// schema provides a way for providers to validate the changed parameters through json.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:ComponentDefRef=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Schema *apiext.JSONSchemaProps `json:"schema,omitempty"`

	// cue that to let provider verify user configuration through cue language.
	// +optional
	CUE string `json:"cue,omitempty"`
}

// ReloadOptions defines reload options
// Only one of its members may be specified.
type ReloadOptions struct {
	// unixSignalTrigger used to reload by sending a signal.
	// +optional
	UnixSignalTrigger *UnixSignalTrigger `json:"unixSignalTrigger,omitempty"`

	// shellTrigger performs the reload command.
	// +optional
	ShellTrigger *ShellTrigger `json:"shellTrigger,omitempty"`

	// goTplTrigger performs the reload command.
	// +optional
	TPLScriptTrigger *TPLScriptTrigger `json:"tplScriptTrigger"`
}

type UnixSignalTrigger struct {
	// signal is valid for unix signal.
	// e.g: SIGHUP
	// url: ../../internal/configuration/configmap/handler.go:allUnixSignals
	// +kubebuilder:validation:Required
	Signal SignalType `json:"signal"`

	// processName is process name, sends unix signal to proc.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ProcessName string `json:"processName"`
}

type ShellTrigger struct {
	// exec used to execute for reload.
	// +kubebuilder:validation:Required
	Exec string `json:"exec"`

	// scriptConfigMapRef used to execute for reload.
	// +kubebuilder:validation:Required
	ScriptConfigMapRef string `json:"scriptConfigMapRef"`

	// Specify the namespace of the referenced the tpl script ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type TPLScriptTrigger struct {
	// scriptConfigMapRef used to execute for reload.
	// +kubebuilder:validation:Required
	ScriptConfigMapRef string `json:"scriptConfigMapRef"`

	// Specify the namespace of the referenced the tpl script ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Specify synchronize updates parameters to the config manager.
	// +optional
	Sync *bool `json:"sync,omitempty"`
}

type FormatterConfig struct {
	// The FormatterOptions represents the special options of configuration file.
	// This is optional for now. If not specified.
	// +optional
	FormatterOptions `json:",inline"`

	// The configuration file format. Valid values are ini, xml, yaml, json,
	// hcl, dotenv, properties and toml.
	//
	// ini: a configuration file that consists of a text-based content with a structure and syntax comprising key–value pairs for properties, reference wiki: https://en.wikipedia.org/wiki/INI_file
	// xml: reference wiki: https://en.wikipedia.org/wiki/XML
	// yaml: a configuration file support for complex data types and structures.
	// json: reference wiki: https://en.wikipedia.org/wiki/JSON
	// hcl: : The HashiCorp Configuration Language (HCL) is a configuration language authored by HashiCorp, reference url: https://www.linode.com/docs/guides/introduction-to-hcl/
	// dotenv: this was a plain text file with simple key–value pairs, reference wiki: https://en.wikipedia.org/wiki/Configuration_file#MS-DOS
	// properties: a file extension mainly used in Java, reference wiki: https://en.wikipedia.org/wiki/.properties
	// toml: reference wiki: https://en.wikipedia.org/wiki/TOML
	// +kubebuilder:validation:Required
	Format CfgFileFormat `json:"format"`
}

// FormatterOptions represents the special options of configuration file.
// Only one of its members may be specified.
type FormatterOptions struct {
	// iniConfig represents the ini options.
	// +optional
	IniConfig *IniConfig `json:"iniConfig,omitempty"`

	// xmlConfig represents the ini options.
	// XMLConfig *XMLConfig `json:"xmlConfig,omitempty"`
}

type IniConfig struct {
	// sectionName describes ini section.
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
