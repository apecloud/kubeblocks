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

// ConfigConstraintPhase defines the ConfigConstraint  CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable, Deleting}
type ConfigConstraintPhase string

const (
	CCAvailablePhase   ConfigConstraintPhase = "Available"
	CCUnavailablePhase ConfigConstraintPhase = "Unavailable"
	CCDeletingPhase    ConfigConstraintPhase = "Deleting"
)

// DynamicParameterSelectedPolicy determines how to select the parameters of dynamic reload actions
//
// +enum
// +kubebuilder:validation:Enum={all,dynamic}
type DynamicParameterSelectedPolicy string

const (
	SelectedAllParameters     DynamicParameterSelectedPolicy = "all"
	SelectedDynamicParameters DynamicParameterSelectedPolicy = "dynamic"
)

// CfgFileFormat defines formatter of configuration files.
// +enum
// +kubebuilder:validation:Enum={xml,ini,yaml,json,hcl,dotenv,toml,properties,redis,props-plus}
type CfgFileFormat string

const (
	Ini            CfgFileFormat = "ini"
	YAML           CfgFileFormat = "yaml"
	JSON           CfgFileFormat = "json"
	XML            CfgFileFormat = "xml"
	HCL            CfgFileFormat = "hcl"
	Dotenv         CfgFileFormat = "dotenv"
	TOML           CfgFileFormat = "toml"
	Properties     CfgFileFormat = "properties"
	RedisCfg       CfgFileFormat = "redis"
	PropertiesPlus CfgFileFormat = "props-plus"
)

// DynamicReloadType defines reload method.
// +enum
type DynamicReloadType string

const (
	UnixSignalType DynamicReloadType = "signal"
	SQLType        DynamicReloadType = "sql"
	ShellType      DynamicReloadType = "exec"
	HTTPType       DynamicReloadType = "http"
	TPLScriptType  DynamicReloadType = "tpl"
	AutoType       DynamicReloadType = "auto"
)

// SignalType defines which signals are valid.
// +enum
// +kubebuilder:validation:Enum={SIGHUP,SIGINT,SIGQUIT,SIGILL,SIGTRAP,SIGABRT,SIGBUS,SIGFPE,SIGKILL,SIGUSR1,SIGSEGV,SIGUSR2,SIGPIPE,SIGALRM,SIGTERM,SIGSTKFLT,SIGCHLD,SIGCONT,SIGSTOP,SIGTSTP,SIGTTIN,SIGTTOU,SIGURG,SIGXCPU,SIGXFSZ,SIGVTALRM,SIGPROF,SIGWINCH,SIGIO,SIGPWR,SIGSYS}
type SignalType string

const (
	SIGHUP    SignalType = "SIGHUP"
	SIGINT    SignalType = "SIGINT"
	SIGQUIT   SignalType = "SIGQUIT"
	SIGILL    SignalType = "SIGILL"
	SIGTRAP   SignalType = "SIGTRAP"
	SIGABRT   SignalType = "SIGABRT"
	SIGBUS    SignalType = "SIGBUS"
	SIGFPE    SignalType = "SIGFPE"
	SIGKILL   SignalType = "SIGKILL"
	SIGUSR1   SignalType = "SIGUSR1"
	SIGSEGV   SignalType = "SIGSEGV"
	SIGUSR2   SignalType = "SIGUSR2"
	SIGPIPE   SignalType = "SIGPIPE"
	SIGALRM   SignalType = "SIGALRM"
	SIGTERM   SignalType = "SIGTERM"
	SIGSTKFLT SignalType = "SIGSTKFLT"
	SIGCHLD   SignalType = "SIGCHLD"
	SIGCONT   SignalType = "SIGCONT"
	SIGSTOP   SignalType = "SIGSTOP"
	SIGTSTP   SignalType = "SIGTSTP"
	SIGTTIN   SignalType = "SIGTTIN"
	SIGTTOU   SignalType = "SIGTTOU"
	SIGURG    SignalType = "SIGURG"
	SIGXCPU   SignalType = "SIGXCPU"
	SIGXFSZ   SignalType = "SIGXFSZ"
	SIGVTALRM SignalType = "SIGVTALRM"
	SIGPROF   SignalType = "SIGPROF"
	SIGWINCH  SignalType = "SIGWINCH"
	SIGIO     SignalType = "SIGIO"
	SIGPWR    SignalType = "SIGPWR"
	SIGSYS    SignalType = "SIGSYS"
)

// ParametersDescPhase defines the ParametersDescription CR .status.phase
// +enum
// +kubebuilder:validation:Enum={Available,Unavailable, Deleting}
type ParametersDescPhase string

const (
	PDAvailablePhase   ParametersDescPhase = "Available"
	PDUnavailablePhase ParametersDescPhase = "Unavailable"
	PDDeletingPhase    ParametersDescPhase = "Deleting"
)

// ParameterDeletedMethod defines how to handle parameter remove
// +enum
// +kubebuilder:validation:Enum={RestoreToDefault, Reset}
type ParameterDeletedMethod string

const (
	PDPDefault ParameterDeletedMethod = "RestoreToDefault"
	PDPReset   ParameterDeletedMethod = "Reset"
)

// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={vscale,hscale,tls}
type RerenderResourceType string

const (
	ComponentVScaleType RerenderResourceType = "vscale"
	ComponentHScaleType RerenderResourceType = "hscale"
)

// ConfigurationPhase defines the Configuration FSM phase
// +enum
// +kubebuilder:validation:Enum={Creating,Init,Running,Pending,Merged,MergeFailed,FailedAndPause,Upgrading,Deleting,FailedAndRetry,Finished}
type ConfigurationPhase string

const (
	CCreatingPhase       ConfigurationPhase = "Creating"
	CInitPhase           ConfigurationPhase = "Init"
	CRunningPhase        ConfigurationPhase = "Running"
	CPendingPhase        ConfigurationPhase = "Pending"
	CFailedPhase         ConfigurationPhase = "FailedAndRetry"
	CFailedAndPausePhase ConfigurationPhase = "FailedAndPause"
	CMergedPhase         ConfigurationPhase = "Merged"
	CMergeFailedPhase    ConfigurationPhase = "MergeFailed"
	CDeletingPhase       ConfigurationPhase = "Deleting"
	CUpgradingPhase      ConfigurationPhase = "Upgrading"
	CFinishedPhase       ConfigurationPhase = "Finished"
)

type ParametersInFile struct {
	// Holds the configuration keys and values. This field is a workaround for issues found in kubebuilder and code-generator.
	// Refer to https://github.com/kubernetes-sigs/kubebuilder/issues/528 and https://github.com/kubernetes/code-generator/issues/50 for more details.
	//
	// Represents the content of the configuration file.
	//
	// +optional
	Content *string `json:"content"`

	// Represents the updated parameters for a single configuration file.
	//
	// +optional
	Parameters map[string]*string `json:"parameters,omitempty"`
}
