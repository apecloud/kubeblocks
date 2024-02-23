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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsDefinitionSpec defines the desired state of OpsDefinition
type OpsDefinitionSpec struct {

	// Specifies the preconditions that must be met to run the actions for the operation.
	// if set, it will check the condition before the component run this operation.
	// +optional
	PreConditions []PreCondition `json:"preConditions,omitempty"`

	// Defines the targetPodTemplate to be referenced by the action.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	TargetPodTemplates []TargetPodTemplate `json:"targetPodTemplates" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the types of componentDefinitions supported by the operation.
	// It can reference certain variables of the componentDefinition.
	// If set, any component not meeting these conditions will be intercepted.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ComponentDefinitionRefs []ComponentDefinitionRef `json:"componentDefinitionRefs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Describes the schema used for validation, pruning, and defaulting.
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

	// The actions to be executed in the opsRequest are performed sequentially.
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Required
	Actions []OpsAction `json:"actions" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type PreCondition struct {

	// Defines the conditions under which the operation can be executed.
	Rule *Rule `json:"rule,omitempty"`

	// Represents a job that will be run to execute the PreCondition.
	// The operation will only be executed if the job is successful.
	// +optional
	// Exec *PreConditionExec `json:"exec,omitempty"`
}

type PreConditionExec struct {
	// Specifies the name of the Docker image to be used for the execution.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Defines the environment variables to be set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the commands to be executed in the container.
	// +optional
	Command []string `json:"command,omitempty"`

	// Represents the arguments to be passed to the command in the container.
	// +optional
	Args []string `json:"args,omitempty"`
}

type Rule struct {
	// Defines how the operation can be executed using a Go template expression.
	// Should return either `true` or `false`. The built-in objects available for use in the expression include:
	// - `params`: These are the input parameters.
	// - `cluster`: This is the referenced cluster object.
	// - `component`: This is the referenced component object.
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`

	// Reported if the rule is not matched.
	// +kubebuilder:validation:Required
	Message string `json:"message"`
}

type TargetPodTemplate struct {
	// Represents the template name.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the environment variables that need to be referenced from the target component pod, and will be injected into the pod's containers.
	// +optional
	Vars []OpsEnvVar `json:"vars,omitempty"`

	// Used to identify the target pod.
	// +kubebuilder:validation:Required
	PodSelector PodSelector `json:"podSelector"`

	// Specifies the mount points for the volumes defined in the `Volumes` section for the action pod.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type OpsEnvVar struct {
	// Specifies the name of the variable. This must be a C_IDENTIFIER.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the source for the variable's value.
	// +kubebuilder:validation:Required
	ValueFrom *OpsVarSource `json:"valueFrom"`
}

// +kubebuilder:validation:XValidation:rule="has(self.envRef) || has(self.fieldPath)", message="either fieldPath and envRef."

type OpsVarSource struct {
	// Specifies a reference to a specific environment variable within a container.
	// Used to specify the source of the variable, which can be either "env" or "envFrom".
	// +optional
	EnvVarRef *EnvVarRef `json:"envRef,omitempty"`

	// Represents the JSONPath of the target pod. This is used to specify the exact location of the data within the JSON structure of the pod.
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`
}

type EnvVarRef struct {
	// Specifies the name of the container as defined in the componentDefinition or as injected by the kubeBlocks controller.
	// If not specified, the first container will be used by default.
	// +optional
	ContainerName string `json:"containerName,omitempty"`

	// Defines the name of the environment variable.
	// +kubebuilder:validation:Required
	EnvName string `json:"envName"`
}

type PodSelector struct {

	// Specifies the role of the target pod.
	// +optional
	Role string `json:"role,omitempty"`

	// Defines the policy for selecting the target pod when multiple pods match the podSelector.
	// It can be either 'Any' (select any one pod that matches the podSelector)
	// or 'All' (select all pods that match the podSelector).
	// +kubebuilder:default=Any
	// +kubebuilder:validation:Required
	SelectionPolicy PodSelectionPolicy `json:"selectionPolicy,omitempty"`

	// Indicates the desired availability status of the pods to be selected.
	// valid values:
	// - 'Available': selects only available pods and terminates the action if none are found.
	// - 'PreferredAvailable': prioritizes the selection of available pods。
	// - 'None': there are no requirements for the availability of pods.
	// +kubebuilder:default=PreferredAvailable
	// +kubebuilder:validation:Required
	Availability PodAvailabilityPolicy `json:"availability,omitempty"`
}

type ComponentDefinitionRef struct {

	// Refers to the name of the component definition. This is a required field with a maximum length of 32 characters.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Represents the account name of the component.
	// If provided, the account username and password will be injected into the job environment variables `KB_ACCOUNT_USERNAME` and `KB_ACCOUNT_PASSWORD`.
	// +optional
	AccountName string `json:"accountName,omitempty"`

	// References the name of the service.
	// If provided, the service name and ports will be mapped to the job environment variables `KB_COMP_SVC_NAME` and `KB_COMP_SVC_PORT_$(portName)`.
	// Note that the portName will replace the characters '-' with '_' and convert to uppercase.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
}

type ParametersSchema struct {
	// Defines the OpenAPI v3 schema used for the parameter schema.
	// The supported property types include:
	// - string
	// - number
	// - integer
	// - array: Note that only items of string type are supported.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +k8s:conversion-gen=false
	// +optional
	OpenAPIV3Schema *apiextensionsv1.JSONSchemaProps `json:"openAPIV3Schema,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.workload) || has(self.exec) || has(self.resourceModifier)", message="at least one action exists for workload, exec and resourceModifier."

type OpsAction struct {
	// action name.
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// failurePolicy is the failure policy of the action. valid values Fail and Ignore.
	// - Fail: if the action failed, the opsRequest will be failed.
	// - Ignore: opsRequest will ignore the failure if the action is failed.
	// +kubebuilder:validation:Enum={Ignore,Fail}
	// +kubebuilder:default=Fail
	// +optional
	FailurePolicy FailurePolicyType `json:"failurePolicy"`

	// Refers to the parameter of the ParametersSchema.
	// The parameter will be used in the action.
	// If it is a 'workload' and 'exec' Action, they will be injected into the corresponding environment variable.
	// If it is a 'resourceModifier' Action, parameter can be referenced using $() in completionProbe.matchExpressions and JsonPatches[*].Value.
	// +optional
	Parameters []string `json:"parameters,omitempty"`

	// Indicates the workload action and a corresponding workload will be created to execute this action.
	// +optional
	Workload *OpsWorkloadAction `json:"workload,omitempty"`

	// Represents the exec action. This will call the kubectl exec interface.
	// +optional
	Exec *OpsExecAction `json:"exec,omitempty"`

	// Specifies the resource modifier to update the custom resource.
	// +optional
	ResourceModifier *OpsResourceModifierAction `json:"resourceModifier,omitempty"`
}

type OpsWorkloadAction struct {
	// Defines the workload type of the action. Valid values include "Job" and "Pod".
	// "Job" creates a job to execute the action.
	// "Pod" creates a pod to execute the action. Note that unlike jobs, if a pod is manually deleted, it will not consume backoffLimit times.
	// +kubebuilder:validation:Required
	Type OpsWorkloadType `json:"type"`

	// Refers to the spec.targetPodTemplates.
	// This field defines the target pod for the current action.
	TargetPodTemplate string `json:"targetPodTemplate,omitempty"`

	// Specifies the number of retries before marking the action as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	BackoffLimit int32 `json:"backoffLimit,omitempty"`

	// Represents the pod spec of the workload.
	// +kubebuilder:validation:Required
	PodSpec corev1.PodSpec `json:"podSpec"`
}

type OpsExecAction struct {
	// Refers to the spec.targetPodTemplates. Defines the target pods that need to execute exec actions.
	// +kubebuilder:validation:Required
	TargetPodTemplate string `json:"targetPodTemplate"`

	// Specifies the number of retries before marking the action as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	BackoffLimit int32 `json:"backoffLimit,omitempty"`

	// The command to execute.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// The name of the container in the target pod to execute the command.
	// If not set, the first container is used.
	// +optional
	ContainerName string `json:"containerName"`
}

type OpsResourceModifierAction struct {
	// Refers to the Kubernetes objects that are required to be updated.
	// +kubebuilder:validation:Required
	Resource TypedObjectRef `json:"resource"`

	//  Defines the set of patches that are used to perform updates on the resource object.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	JsonPatches []JsonPatchOperation `json:"jsonPatches"`

	// Provides a method to check if the action has been completed.
	// +kubebuilder:validation:Required
	CompletionProbe CompletionProbe `json:"completionProbe"`
}

type JsonPatchOperation struct {
	// Represents the type of JSON patch operation. It supports the following values: 'add', 'remove', 'replace'.
	// +enum
	// +kubebuilder:validation:Enum={add,remove,replace}
	// +kubebuilder:validation:Required
	Operation string `json:"op"`

	// Represents the json patch path.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Represents the value to be used in the JSON patch operation.
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

type TypedObjectRef struct {
	// Defines the group for the resource being referenced.
	// If not specified, the referenced Kind must belong to the core API group.
	// For all third-party types, this is mandatory.
	// +kubebuilder:validation:Required
	APIGroup *string `json:"apiGroup"`

	// Specifies the type of resource being referenced.
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Indicates the name of the resource being referenced.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type CompletionProbe struct {
	// Specifies the number of seconds to wait after the resource has been patched before initiating completion probes.
	// The default value is 5 seconds, with a minimum value of 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// Defines the number of seconds after which the probe times out.
	// The default value is 60 seconds, with a minimum value of 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=60
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Indicates the frequency (in seconds) at which the probe should be performed.
	// The default value is 5 seconds, with a minimum value of 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Executes expressions regularly, based on the value of PeriodSeconds, to determine if the action has been completed.
	// +kubebuilder:validation:Required
	MatchExpressions MatchExpressions `json:"matchExpressions"`
}

type MatchExpressions struct {
	// Defines a failure condition for an action using a Go template expression.
	// Should evaluate to either `true` or `false`.
	// The current resource object is parsed into the Go template.
	// for example, you can use '{{ eq .spec.replicas 1 }}'.
	// +optional
	Failure string `json:"failure,omitempty"`

	// Defines a success condition for an action using a Go template expression.
	// Should evaluate to either `true` or `false`.
	// The current resource object is parsed into the Go template.
	// for example, using '{{ eq .spec.replicas 1 }}'
	// +kubebuilder:validation:Required
	Success string `json:"success"`
}

// OpsDefinitionStatus defines the observed state of OpsDefinition
type OpsDefinitionStatus struct {
	// Refers to the most recent generation observed for this OpsDefinition.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current state of the OpsDefinition. Valid values are ``, `Available`, `Unavailable`.
	// When the state is `Available`, the OpsDefinition is ready and can be used for related objects.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
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
