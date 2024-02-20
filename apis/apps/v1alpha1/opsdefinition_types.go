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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsDefinitionSpec defines the desired state of OpsDefinition
type OpsDefinitionSpec struct {

	// Specifies the types of componentDefinitions that are supported by the operation.
	// It can refer to some variables of the componentDefinition.
	// If set, any component that does not meet the conditions will be intercepted.
	//
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ComponentDefinitionRefs []ComponentDefinitionRef `json:"componentDefinitionRefs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Defines the environment variables that need to be referenced from the target component pod, and will be injected into the job's containers.
	//
	// +optional
	VarsRef *VarsRef `json:"varsRef,omitempty"`

	// Describes the schema used for validation, pruning, and defaulting.
	//
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

	// Describes the job specification for the operation.
	//
	// +kubebuilder:validation:Required
	JobSpec batchv1.JobSpec `json:"jobSpec"`

	// Specifies the preconditions that must be met to run the job for the operation.
	//
	// +optional
	PreConditions []PreCondition `json:"preConditions,omitempty"`
}

type ComponentDefinitionRef struct {

	// Refers to the name of the component definition. This is a required field with a maximum length of 32 characters.
	//
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Represents the account name of the component.
	// If provided, the account username and password will be injected into the job environment variables `KB_ACCOUNT_USERNAME` and `KB_ACCOUNT_PASSWORD`.
	//
	// +optional
	AccountName string `json:"accountName,omitempty"`

	// References the name of the service.
	// If provided, the service name and ports will be mapped to the job environment variables `KB_COMP_SVC_NAME` and `KB_COMP_SVC_PORT_$(portName)`.
	// Note that the portName will replace the characters '-' with '_' and convert to uppercase.
	//
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// Defines the environment variables that need to be referenced from the target component pod and will be injected into the job's containers.
	// If this field is set, the global "varsRef" will be ignored.
	//
	// +optional
	VarsRef *VarsRef `json:"varsRef,omitempty"`
}

type ParametersSchema struct {
	// Defines the OpenAPI v3 schema used for the parameter schema.
	// The supported property types include:
	// - string
	// - number
	// - integer
	// - array: Note that only items of string type are supported.
	//
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
	// +optional
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

type VarsRef struct {
	// Defines the method to select the target component pod for variable references.
	// The strategy can be either 'PreferredAvailable' which prioritizes the selection of available pods,
	// or 'Available' which selects only available pods and terminates the operation if none are found.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=PreferredAvailable
	PodSelectionStrategy PodSelectionStrategy `json:"podSelectionStrategy"`

	// Represents a list of environment variables to be set in the job's container.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Vars []OpsEnvVar `json:"vars,omitempty"`
}

type OpsEnvVar struct {
	// Specifies the name of the variable. This must be a C_IDENTIFIER.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the source for the variable's value.
	//
	// +kubebuilder:validation:Required
	ValueFrom *OpsVarSource `json:"valueFrom"`
}

type OpsVarSource struct {
	// Specifies a reference to a specific environment variable within a container.
	// Used to specify the source of the variable, which can be either "env" or "envFrom".
	//
	EnvVarRef *EnvVarRef `json:"envVarRef,omitempty"`
}

type EnvVarRef struct {
	// Specifies the name of the container as defined in the componentDefinition or as injected by the kubeBlocks controller.
	// If not specified, the first container will be used by default.
	//
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// Defines the name of the environment variable.
	//
	// +kubebuilder:validation:Required
	EnvName string `json:"envName"`
}

// +kubebuilder:validation:XValidation:rule="has(self.rule) || has(self.exec)", message="at least one exists for rule and exec."

type PreCondition struct {

	// Defines the conditions under which the operation can be executed.
	Rule *Rule `json:"rule,omitempty"`

	// Represents a job that will be run to execute the PreCondition.
	// The operation will only be executed if the job is successful.
	//
	// +optional
	Exec *PreConditionExec `json:"exec,omitempty"`
}

type Rule struct {
	// Defines how the operation can be executed using a Go template expression.
	// Should return either `true` or `false`. The built-in objects available for use in the expression include:
	// - `params`: These are the input parameters.
	// - `cluster`: This is the referenced cluster object.
	// - `component`: This is the referenced component object.
	//
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`

	// Reported if the rule is not matched.
	//
	// +kubebuilder:validation:Required
	Message string `json:"message"`
}

type PreConditionExec struct {
	// Specifies the name of the Docker image to be used for the execution.
	//
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Defines the environment variables to be set in the container.
	//
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the commands to be executed in the container.
	//
	// +optional
	Command []string `json:"command,omitempty"`

	// Represents the arguments to be passed to the command in the container.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

// OpsDefinitionStatus defines the observed state of OpsDefinition
type OpsDefinitionStatus struct {
	// Refers to the most recent generation observed for this OpsDefinition.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current state of the OpsDefinition. Valid values are ``, `Available`, `Unavailable`.
	// When the state is `Available`, the OpsDefinition is ready and can be used for related objects.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},scope=Cluster,shortName=od
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="OpsDefinition status phase."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// OpsDefinition is the Schema for the opsdefinitions API
type OpsDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsDefinitionSpec   `json:"spec,omitempty"`
	Status OpsDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsDefinitionList contains a list of OpsDefinition
type OpsDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsDefinition{}, &OpsDefinitionList{})
}

func (o *OpsDefinition) GetComponentDefRef(compDefName string) *ComponentDefinitionRef {
	if o == nil {
		return nil
	}
	for _, v := range o.Spec.ComponentDefinitionRefs {
		if compDefName == v.Name {
			return &v
		}
	}
	return nil
}
