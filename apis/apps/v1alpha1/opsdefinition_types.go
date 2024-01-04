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

	// componentDefinitionRefs indicates which types of componentDefinitions are supported by the operation,
	// and can refer some vars of the componentDefinition.
	// if it is set, the component that does not meet the conditions will be intercepted.
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ComponentDefinitionRefs []ComponentDefinitionRef `json:"componentDefinitionRefs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// varsRef defines the envs that need to be referenced from the target component pod, and will inject to job's containers.
	// +optional
	VarsRef *VarsRef `json:"varsRef,omitempty"`

	// parametersSchema describes the schema used for validation, pruning, and defaulting.
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

	// jobSpec describes the job spec for the operation.
	// +kubebuilder:validation:Required
	JobSpec batchv1.JobSpec `json:"jobSpec"`

	// preCondition if it meets the requirements to run the job for the operation.
	// +optional
	PreConditions []PreCondition `json:"preConditions,omitempty"`
}

type ComponentDefinitionRef struct {

	// refer to componentDefinition name.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// the account name of the component.
	// will inject the account username and password to KB_ACCOUNT_USERNAME and KB_ACCOUNT_PASSWORD in env of the job.
	// +optional
	AccountName string `json:"accountName,omitempty"`

	// reference the services[*].name.
	// will map the service name and ports to KB_COMP_SVC_NAME and KB_COMP_SVC_PORT_<portName> in env of the job.
	// portName will replace the characters '-' to '_' and convert to uppercase.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// varsRef defines the envs that need to be referenced from the target component pod, and will inject to job's containers.
	// if it is set, will ignore the global "varsRef".
	// +optional
	VarsRef *VarsRef `json:"varsRef,omitempty"`
}

type ParametersSchema struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for parameter schema.
	// supported properties types:
	// - string
	// - number
	// - integer
	// - array: only supported the item with string type.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
	// +optional
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

type VarsRef struct {
	// podSelectionStrategy how to select the target component pod for variable references based on the strategy.
	// - PreferredAvailable: prioritize the selection of available pod.
	// - Available: only select available pod. if not found, terminating the operation.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=PreferredAvailable
	PodSelectionStrategy PodSelectionStrategy `json:"podSelectionStrategy"`

	// List of environment variables to set in the job's container.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Vars []OpsEnvVar `json:"vars,omitempty"`
}

type OpsEnvVar struct {
	// Name of the variable. Must be a C_IDENTIFIER.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Source for the variable's value. Cannot be used if value is not empty.
	// +kubebuilder:validation:Required
	ValueFrom *OpsVarSource `json:"valueFrom"`
}

type OpsVarSource struct {
	// envVarRef defines which container and env that the variable references from.
	// source: "env" or "envFrom" of the container.
	EnvVarRef *EnvVarRef `json:"envVarRef,omitempty"`
}

type EnvVarRef struct {
	// container name which defines in componentDefinition or is injected by kubeBlocks controller.
	// +kubebuilder:validation:Required
	ContainerName string `json:"containerName"`

	// env name, it will .
	// +kubebuilder:validation:Required
	EnvName string `json:"envName"`
}

// +kubebuilder:validation:XValidation:rule="has(self.rule) || has(self.exec)", message="at least one exists for rule and exec."

type PreCondition struct {

	// condition declares how the operation can be executed.
	Rule *Rule `json:"rule,omitempty"`

	// a job will be run to execute preCondition.
	// and the operation will be executed when the exec job is succeed.
	// +optional
	Exec *PreConditionExec `json:"exec,omitempty"`
}

type Rule struct {
	// expression declares how the operation can be executed using go template expression.
	// it should return "true" or "false", built-in objects:
	// - "params" are input parameters.
	// - "cluster" is referenced cluster object.
	// - "component" is referenced the component Object.
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`

	// report the message if the rule is not matched.
	// +kubebuilder:validation:Required
	Message string `json:"message"`
}

type PreConditionExec struct {
	// image name.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// container env.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// container commands.
	// +optional
	Command []string `json:"command,omitempty"`

	// container args.
	// +optional
	Args []string `json:"args,omitempty"`
}

// OpsDefinitionStatus defines the observed state of OpsDefinition
type OpsDefinitionStatus struct {
	// ObservedGeneration is the most recent generation observed for this OpsDefinition.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase valid values are ``, `Available`, 'Unavailable`.
	// Available is OpsDefinition become available, and can be used for co-related objects.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Extra message for current phase.
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

//+kubebuilder:object:root=true

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
