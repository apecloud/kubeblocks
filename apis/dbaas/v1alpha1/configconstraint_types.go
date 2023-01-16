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
	// +optional
	StaticParameters []string `json:"staticParameters,omitempty"`

	// dynamicParameters, list of DynamicParameter, modifications of them trigger a config dynamic reload without process restart.
	// +optional
	DynamicParameters []string `json:"dynamicParameters,omitempty"`

	// immutableParameters describes parameters that prohibit user from modification.
	// +optional
	ImmutableParameters []string `json:"immutableParameters,omitempty"`

	// formatterConfig describes the format of the configuration file, the controller
	// 1. parses configuration file
	// 2. analyzes the modified parameters
	// 3. applies corresponding policies.
	// +kubebuilder:validation:Required
	FormatterConfig *FormatterConfig `json:"formatterConfig"`
}

// ConfigConstraintStatus defines the observed state of ConfigConstraint.
type ConfigConstraintStatus struct {
	// phase is status of configuration template, when set to AvailablePhase, it can be referenced by ClusterDefinition or AppVersion.
	// +kubebuilder:validation:Enum={Available,Unavailable,Deleting}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// message field describes the reasons of abnormal status.
	// +optional
	Message string `json:"message,omitempty"`

	// observedGeneration is the latest generation observed for this
	// ClusterDefinition. It refers to the ConfigConstraint's generation, which is
	// updated by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type CustomParametersValidation struct {
	// schema provides a way for providers to validate the changed parameters through json.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
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
}

type UnixSignalTrigger struct {
	// signal is valid for unix signal.
	// e.g: SIGHUP
	// url: ../../internal/configuration/configmap/handler.go:allUnixSignals
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={SIGHUP,SIGINT,SIGQUIT,SIGILL,SIGTRAP,SIGABRT,SIGBUS,SIGFPE,SIGKILL,SIGUSR1,SIGSEGV,SIGUSR2,SIGPIPE,SIGALRM,SIGTERM,SIGSTKFLT,SIGCHLD,SIGCONT,SIGSTOP,SIGTSTP,SIGTTIN,SIGTTOU,SIGURG,SIGXCPU,SIGXFSZ,SIGVTALRM,SIGPROF,SIGWINCH,SIGIO,SIGPWR,SIGSYS}
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
}

type FormatterConfig struct {
	// formatter describes the format of the configuration file
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={xml,ini,yaml,json}
	// Formatter ConfigurationFormatter `json:"formatter"`
	Formatter ConfigurationFormatter `json:"formatter"`

	// The FormatterOptions represents the special options of configuration file.
	// This is optional for now. If not specified.
	// +optional
	FormatterOptions `json:",inline"`
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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster,shortName=ctpl
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigConstraint is the Schema for the configconstraint API
type ConfigConstraint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigConstraintSpec   `json:"spec,omitempty"`
	Status ConfigConstraintStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConfigConstraintList contains a list of ConfigConstraints.
type ConfigConstraintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigConstraint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigConstraint{}, &ConfigConstraintList{})
}
