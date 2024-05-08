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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpsDefinitionSpec defines the desired state of OpsDefinition.
type OpsDefinitionSpec struct {
	// Specifies the preconditions that must be met to run the actions for the operation.
	// if set, it will check the condition before the Component runs this operation.
	// Example:
	// ```yaml
	//  preConditions:
	//  - rule:
	//      expression: '{{ eq .component.status.phase "Running" }}'
	//      message: Component is not in Running status.
	// ```
	// +optional
	PreConditions []PreCondition `json:"preConditions,omitempty"`

	// Specifies a list of PodInfoExtractor, each designed to select a specific Pod and extract selected runtime info
	// from its PodSpec.
	// The extracted information, such as environment variables, volumes and tolerations, are then injected into
	// Jobs or Pods that execute the OpsActions defined in `actions`.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	PodInfoExtractors []PodInfoExtractor `json:"podInfoExtractors,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies a list of ComponentDefinition for Components associated with this OpsDefinition.
	// It also includes connection credentials (address and account) for each Component.
	//
	// +patchMergeKey=componentDefinitionName
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefinitionName
	// +optional
	ComponentInfos []ComponentInfo `json:"componentInfos,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"componentDefinitionName"`

	// Specifies the schema for validating the data types and value ranges of parameters in OpsActions before their usage.
	//
	// +optional
	ParametersSchema *ParametersSchema `json:"parametersSchema,omitempty"`

	// Specifies a list of OpsAction where each customized action is executed sequentially.
	//
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Required
	Actions []OpsAction `json:"actions" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`
}

type PreCondition struct {
	// Specifies the conditions that must be met for the operation to execute.
	Rule *Rule `json:"rule,omitempty"`

	// Represents a job that will be run to execute the PreCondition.
	// The operation will only be executed if the job is successful.
	// +optional
	// Exec *PreConditionExec `json:"exec,omitempty"`
}

// PreConditionExec is deprecated.
type PreConditionExec struct {
	// Specifies the name of the image used for execution.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Specifies a list of environment variables to be set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Specifies the command to be executed in the container.
	// +optional
	Command []string `json:"command,omitempty"`

	// Specifies the arguments to be passed to the command in the container.
	// +optional
	Args []string `json:"args,omitempty"`
}

type Rule struct {
	// Specifies a Go template expression that determines how the operation can be executed.
	// The return value must be either `true` or `false`.
	// Available built-in objects that can be referenced in the expression include:
	//
	// - `params`: Input parameters.
	// - `cluster`: The referenced Cluster object.
	// - `component`: The referenced Component object.
	//
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`

	// Specifies the error or status message reported if the `expression` does not evaluate to `true`.
	//
	// +kubebuilder:validation:Required
	Message string `json:"message"`
}

type PodInfoExtractor struct {
	// Specifies the name of the PodInfoExtractor.
	//
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies a list of environment variables to be extracted from a selected Pod,
	// and injected into the containers executing each OpsAction.
	//
	// +optional
	Env []OpsEnvVar `json:"env,omitempty"`

	// Used to select the target Pod from which environment variables and volumes are extracted from its PodSpec.
	//
	// +kubebuilder:validation:Required
	PodSelector PodSelector `json:"podSelector"`

	// Specifies a list of volumes, along with their respective mount points, that are to be extracted from a selected Pod,
	// and mounted onto the containers executing each OpsAction.
	// This allows the containers to access shared or persistent data necessary for the operation.
	//
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

type OpsEnvVar struct {
	// Specifies the name of the environment variable to be injected into Pods executing OpsActions.
	// It must conform to the C_IDENTIFIER format, which includes only alphanumeric characters and underscores, and cannot begin with a digit.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the source of the environment variable's value.
	//
	// +kubebuilder:validation:Required
	ValueFrom *OpsVarSource `json:"valueFrom"`
}

// +kubebuilder:validation:XValidation:rule="has(self.envRef) || has(self.fieldPath)", message="either fieldPath and envRef."

type OpsVarSource struct {
	// Specifies a reference to a specific environment variable within a container.
	// Used to specify the source of the variable, which can be either "env" or "envFrom".
	//
	// +optional
	EnvVarRef *EnvVarRef `json:"envRef,omitempty"`

	// Represents the JSONPath expression pointing to the specific data within the JSON structure of the target Pod.
	// It is used to extract precise data locations for operations on the Pod.
	//
	// +optional
	FieldRef *corev1.ObjectFieldSelector `json:"fieldPath,omitempty"`
}

type EnvVarRef struct {
	// Specifies the container name in the target Pod.
	// If not specified, the first container will be used by default.
	//
	// +optional
	TargetContainerName string `json:"targetContainerName,omitempty"`

	// Defines the name of the environment variable.
	// This name can originate from an 'env' entry or be a data key from an 'envFrom' source.
	//
	// +kubebuilder:validation:Required
	EnvName string `json:"envName"`
}

// PodSelector selects the target Pod from which environment variables and volumes are extracted from its PodSpec.
type PodSelector struct {
	// Specifies the role of the target Pod.
	//
	// +optional
	Role string `json:"role,omitempty"`

	// Defines the policy for selecting the target pod when multiple pods match the podSelector.
	// It can be either 'Any' (select any one pod that matches the podSelector)
	// or 'All' (select all pods that match the podSelector).
	//
	// +kubebuilder:default=Any
	// +kubebuilder:validation:Required
	MultiPodSelectionPolicy PodSelectionPolicy `json:"multiPodSelectionPolicy,omitempty"`
}

type ComponentInfo struct {
	// Specifies the name of the ComponentDefinition.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Required
	ComponentDefinitionName string `json:"componentDefinitionName"`

	// Specifies the account name associated with the Component.
	// If set, the corresponding account username and password are injected into containers' environment variables
	// `KB_ACCOUNT_USERNAME` and `KB_ACCOUNT_PASSWORD`.
	//
	// +optional
	AccountName string `json:"accountName,omitempty"`

	// Specifies the name of the Service.
	// If set, the service name is injected as the `KB_COMP_SVC_NAME` environment variable in the containers,
	// and each service port is mapped to a corresponding environment variable named `KB_COMP_SVC_PORT_$(portName)`.
	// The `portName` is transformed by replacing '-' with '_' and converting to uppercase.
	//
	// +optional
	ServiceName string `json:"serviceName,omitempty"`
}

type ParametersSchema struct {
	// Defines the schema for parameters using the OpenAPI v3.
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

// OpsAction specifies a custom action defined in OpsDefinition for execution in a "Custom" OpsRequest.
//
// OpsAction can be of three types:
//
//   - workload: Creates a Job or Pod to run custom scripts, ideal for isolated or long-running tasks.
//   - exec: Executes commands directly within an existing container using the kubectl exec interface,
//     suitable for immediate, short-lived operations.
//   - resourceModifier: Modifies a K8s object using JSON patches, useful for updating the spec of some resource.
//
// +kubebuilder:validation:XValidation:rule="has(self.workload) || has(self.exec) || has(self.resourceModifier)", message="at least one action exists for workload, exec and resourceModifier."
type OpsAction struct {
	// Specifies the name of the OpsAction.
	// +kubebuilder:validation:MaxLength=20
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the failure policy of the OpsAction.
	// Valid values are:
	//
	// - "Fail": Marks the entire OpsRequest as failed if the action fails.
	// - "Ignore": The OpsRequest continues processing despite the failure of the action.
	// +kubebuilder:validation:Enum={Ignore,Fail}
	// +kubebuilder:default=Fail
	// +optional
	FailurePolicy FailurePolicyType `json:"failurePolicy"`

	// Specifies the parameters for the OpsAction. Their usage varies based on the action type:
	//
	// - For 'workload' or 'exec' actions, parameters are injected as environment variables.
	// - For 'resourceModifier' actions, parameter can be referenced using $() in fields
	// `resourceModifier.completionProbe.matchExpressions` and `resourceModifier.jsonPatches[*].value`.
	//
	// +optional
	Parameters []string `json:"parameters,omitempty"`

	// Specifies the configuration for a 'workload' action.
	// This action leads to the creation of a K8s workload, such as a Pod or Job, to execute specified tasks.
	//
	// +optional
	Workload *OpsWorkloadAction `json:"workload,omitempty"`

	// Specifies the configuration for a 'exec' action.
	// It creates a Pod and invokes a 'kubectl exec' to run command inside a specified container with the target Pod.
	// +optional
	Exec *OpsExecAction `json:"exec,omitempty"`

	// Specifies the configuration for a 'resourceModifier' action.
	// This action allows for modifications to existing K8s objects.
	//
	// Note: This feature has not been implemented yet.
	//
	// +optional
	ResourceModifier *OpsResourceModifierAction `json:"resourceModifier,omitempty"`
}

type OpsWorkloadAction struct {
	// Defines the workload type of the action. Valid values include "Job" and "Pod".
	//
	// - "Job": Creates a Job to execute the action.
	// - "Pod": Creates a Pod to execute the action.
	//    Note: unlike Jobs, manually deleting a Pod does not affect the `backoffLimit`.
	//
	// +kubebuilder:validation:Required
	Type OpsWorkloadType `json:"type"`

	// Specifies a PodInfoExtractor defined in the `opsDefinition.spec.podInfoExtractors`.
	PodInfoExtractorName string `json:"podInfoExtractorName,omitempty"`

	// Specifies the number of retries allowed before marking the action as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	BackoffLimit int32 `json:"backoffLimit,omitempty"`

	// Specifies the PodSpec of the 'workload' action.
	// +kubebuilder:validation:Required
	PodSpec corev1.PodSpec `json:"podSpec"`
}

type OpsExecAction struct {
	// Specifies a PodInfoExtractor defined in the `opsDefinition.spec.podInfoExtractors`.
	// +kubebuilder:validation:Required
	PodInfoExtractorName string `json:"podInfoExtractorName"`

	// Specifies the number of retries allowed before marking the action as failed.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	// +optional
	BackoffLimit int32 `json:"backoffLimit,omitempty"`

	// The command to be executed via 'kubectl exec --'.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// The name of the container in the target pod where the command should be executed.
	// This corresponds to the `-c {containerName}` option in `kubectl exec`.
	//
	// If not set, the first container is used.
	//
	// +optional
	ContainerName string `json:"containerName"`
}

type OpsResourceModifierAction struct {
	// Specifies the K8s object that is to be updated.
	//
	// +kubebuilder:validation:Required
	Resource TypedObjectRef `json:"resource"`

	// Specifies a list of patches for modifying the object.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	JSONPatches []JSONPatchOperation `json:"jsonPatches"`

	// Specifies a method to determine if the action has been completed.
	//
	// Note: This feature has not been implemented yet.
	//
	// +kubebuilder:validation:Required
	CompletionProbe CompletionProbe `json:"completionProbe"`
}

type JSONPatchOperation struct {
	// Specifies the type of JSON patch operation. It supports the following values: 'add', 'remove', 'replace'.
	// +enum
	// +kubebuilder:validation:Enum={add,remove,replace}
	// +kubebuilder:validation:Required
	Operation string `json:"op"`

	// Specifies the json patch path.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Specifies the value to be used in the JSON patch operation.
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

type TypedObjectRef struct {
	// Specifies the group for the resource being referenced.
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

	// Specifies the number of seconds after which the probe times out.
	// The default value is 60 seconds, with a minimum value of 1.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=60
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Specifies the frequency (in seconds) at which the probe should be performed.
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
	// Specifies a failure condition for an action using a Go template expression.
	// Should evaluate to either `true` or `false`.
	// The current resource object is parsed into the Go template.
	// for example, you can use '{{ eq .spec.replicas 1 }}'.
	// +optional
	Failure string `json:"failure,omitempty"`

	// Specifies a success condition for an action using a Go template expression.
	// Should evaluate to either `true` or `false`.
	// The current resource object is parsed into the Go template.
	// for example, using '{{ eq .spec.replicas 1 }}'
	// +kubebuilder:validation:Required
	Success string `json:"success"`
}

// OpsDefinitionStatus defines the observed state of OpsDefinition
type OpsDefinitionStatus struct {
	// Represents the most recent generation observed of this OpsDefinition.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the current state of the OpsDefinition.
	// Valid values are "", "Available", "Unavailable".
	// When it equals to "Available", the OpsDefinition is ready and can be used in a "Custom" OpsRequest.
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

// OpsDefinition is the Schema for the OpsDefinitions API.
type OpsDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpsDefinitionSpec   `json:"spec,omitempty"`
	Status OpsDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpsDefinitionList contains a list of OpsDefinition.
type OpsDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpsDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpsDefinition{}, &OpsDefinitionList{})
}

func (o *OpsDefinition) GetComponentInfo(compDefName string) *ComponentInfo {
	if o == nil {
		return nil
	}
	for _, v := range o.Spec.ComponentInfos {
		if compDefName == v.ComponentDefinitionName {
			return &v
		}
	}
	return nil
}
