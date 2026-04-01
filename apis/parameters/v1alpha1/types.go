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

// CfgFileFormat defines formatter of configuration files.
// +enum
// +kubebuilder:validation:Enum={xml,ini,yaml,json,hcl,dotenv,toml,properties,redis,props-plus,props-ultra}
type CfgFileFormat string

const (
	Ini             CfgFileFormat = "ini"
	YAML            CfgFileFormat = "yaml"
	JSON            CfgFileFormat = "json"
	XML             CfgFileFormat = "xml"
	HCL             CfgFileFormat = "hcl"
	Dotenv          CfgFileFormat = "dotenv"
	TOML            CfgFileFormat = "toml"
	Properties      CfgFileFormat = "properties"
	RedisCfg        CfgFileFormat = "redis"
	PropertiesPlus  CfgFileFormat = "props-plus"
	PropertiesUltra CfgFileFormat = "props-ultra"
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

// Deprecated: It is retained for API compatibility with existing ParametersDefinition objects.
//
// ParameterDeletedMethod defines how to handle parameter remove
// +enum
// +kubebuilder:validation:Enum={RestoreToDefault, Reset}
type ParameterDeletedMethod string

const (
	PDPDefault ParameterDeletedMethod = "RestoreToDefault"
	PDPReset   ParameterDeletedMethod = "Reset"
)

// Deprecated: It is retained for API compatibility with existing ParamConfigRenderer objects.
//
// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={vscale,hscale,tls,shardingHScale}
type RerenderResourceType string

const (
	ComponentVScaleType         RerenderResourceType = "vscale"
	ComponentHScaleType         RerenderResourceType = "hscale"
	ComponentTLSType            RerenderResourceType = "tls"
	ShardingComponentHScaleType RerenderResourceType = "shardingHScale"
)

// ParameterPhase defines the Configuration FSM phase
// +enum
// +kubebuilder:validation:Enum={Creating,Init,Running,Pending,Merged,MergeFailed,FailedAndPause,Upgrading,Deleting,FailedAndRetry,Finished}
type ParameterPhase string

const (
	CCreatingPhase       ParameterPhase = "Creating"
	CInitPhase           ParameterPhase = "Init"
	CRunningPhase        ParameterPhase = "Running"
	CPendingPhase        ParameterPhase = "Pending"
	CFailedPhase         ParameterPhase = "FailedAndRetry"
	CFailedAndPausePhase ParameterPhase = "FailedAndPause"
	CMergedPhase         ParameterPhase = "Merged"
	CMergeFailedPhase    ParameterPhase = "MergeFailed"
	CDeletingPhase       ParameterPhase = "Deleting"
	CUpgradingPhase      ParameterPhase = "Upgrading"
	CFinishedPhase       ParameterPhase = "Finished"
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

	// Represents unmanaged parameter updates for a single configuration file.
	//
	// +optional
	UnmanagedUpdates []UnmanagedParameterSectionUpdate `json:"unmanagedUpdates,omitempty"`
}

type ComponentParameters map[string]*string

// MergedPolicy defines how to merge external imported templates into component templates.
// +enum
// +kubebuilder:validation:Enum={patch,replace,none}
type MergedPolicy string

const (
	PatchPolicy     MergedPolicy = "patch"
	ReplacePolicy   MergedPolicy = "replace"
	OnlyAddPolicy   MergedPolicy = "add"
	NoneMergePolicy MergedPolicy = "none"
)

type ConfigTemplateExtension struct {
	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Defines the strategy for merging externally imported templates into component templates.
	//
	// +kubebuilder:default="none"
	// +optional
	Policy MergedPolicy `json:"policy,omitempty"`
}
