/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ParametersDefinitionSpec defines the desired state of ParametersDefinition
type ParametersDefinitionSpec struct {
	// Specifies the config file name in the config template.
	//
	// +kubebuilder:validation:Required
	FileName string `json:"fileName"`

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

	// Specifies whether the config file can be injected into env.
	//
	// Scenes: An engine often has multiple configuration files. Some configurations need to be injected
	// into environment variables to take effect, while others can be directly referenced by the engine.
	// To support this scenario, it is currently necessary to split one configuration template into two (for example, Pulsar):
	// one configuration template will be injected into environment and another to be provided directly to the engine as a file.
	// By providing this API, these can be merged into a configuration template.
	//
	// Example:
	// ```yaml
	// configs:
	//   - name: broker-env
	//     templateRef: {{ include "pulsar.name" . }}-broker-env-tpl
	//     namespace: {{ .Release.Namespace }}
	//     constraintRef: pulsar-env-constraints
	//     keys:
	//       - conf
	//     injectEnvTo:
	//       - init-broker-cluster
	//       - broker
	//       - init-pulsar-client-config
	//     volumeName: broker-env
	//   - name: broker-config
	//     templateRef: {{ include "pulsar.name" . }}3-broker-config-tpl
	//     namespace: {{ .Release.Namespace }}
	//     constraintRef: pulsar3-brokers-cc
	//     volumeName: pulsar-config
	//
	// ## merger config template:
	//
	// configs:
	//   - name: broker-config
	//     templateRef: {{ include "pulsar.name" . }}3-broker-config-tpl
	//     namespace: {{ .Release.Namespace }}
	//     constraintRef: pulsar3-brokers-cc
	//     volumeName: pulsar-config
	//     injectEnvTo:
	//       - init-broker-cluster
	//       - broker
	//       - init-pulsar-client-config
	// ```
	//
	// +optional
	AsEnvFrom *bool `json:"asEnvFrom,omitempty"`

	// Specifies the policy when parameter be removed.
	//
	// +optional
	// DeletedPolicy *ParameterDeletedPolicy `json:"deletedPolicy,omitempty"`

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
	//
	// +kubebuilder:validation:Required
	DeletedMethod ParameterDeletedMethod `json:"deletedMethod"`

	// Specifies the value to use if DeletedMethod is PDPResetDefault.
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

// +genclient
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=paramsdef
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
