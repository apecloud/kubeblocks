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

package legacy

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:skip

type InstanceTemplate struct {
	// Specifies the name of the template.
	// Each instance of the template derives its name from the ReplicatedStateMachine Name, the template's Name and the instance's ordinal.
	// The constructed instance name follows the pattern $(rsm.name)-$(template.name)-$(ordinal).
	// The ordinal starts from 0 by default.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Number of replicas of this template.
	// Default is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Defines annotations to override.
	// Add new or override existing annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Defines labels to override.
	// Add new or override existing labels.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Defines image to override.
	// Will override the first container's image of the pod.
	// +optional
	Image *string `json:"image,omitempty"`

	// Defines NodeName to override.
	// +optional
	NodeName *string `json:"nodeName,omitempty"`

	// Defines NodeSelector to override.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Defines Tolerations to override.
	// Add new or override existing tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Defines Resources to override.
	// Will override the first container's resources of the pod.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Defines Env to override.
	// Add new or override existing envs.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Defines Volumes to override.
	// Add new or override existing volumes.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// Defines VolumeMounts to override.
	// Add new or override existing volume mounts of the first container in the pod.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Defines VolumeClaimTemplates to override.
	// Add new or override existing volume claim templates.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}

// ReplicatedStateMachineSpec defines the desired state of ReplicatedStateMachine
type ReplicatedStateMachineSpec struct {
	// Specifies the desired number of replicas of the given Template.
	// These replicas are instantiations of the same Template, with each having a consistent identity.
	// Defaults to 1 if unspecified.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Defines the minimum number of seconds a newly created pod should be ready
	// without any of its container crashing to be considered available.
	// Defaults to 0, meaning the pod will be considered available as soon as it is ready.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// Represents a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	// Refers to the name of the service that governs this StatefulSet.
	// This service must exist before the StatefulSet and is responsible for
	// the network identity of the set. Pods get DNS/hostnames that follow a specific pattern.
	ServiceName string `json:"serviceName"`

	// Defines the behavior of a service spec.
	// Provides read-write service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Service *corev1.Service `json:"service,omitempty"`

	// Defines Alternative Services selector pattern specifier.
	// Can be used for creating Readonly service.
	// +optional
	AlternativeServices []corev1.Service `json:"alternativeServices,omitempty"`

	Template corev1.PodTemplateSpec `json:"template"`

	// Overrides values in default Template.
	//
	// Instance is the fundamental unit managed by KubeBlocks.
	// It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
	// An ReplicatedStateMachine manages instances with a total count of Replicas,
	// and by default, all these instances are generated from the same template.
	// The InstanceTemplate provides a way to override values in the default template,
	// allowing the ReplicatedStateMachine to manage instances from different templates.
	//
	// The naming convention for instances (pods) based on the ReplicatedStateMachine Name, InstanceTemplate Name, and ordinal.
	// The constructed instance name follows the pattern: $(instance_set.name)-$(template.name)-$(ordinal).
	// By default, the ordinal starts from 0 for each InstanceTemplate.
	// It is important to ensure that the Name of each InstanceTemplate is unique.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the ReplicatedStateMachine.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies instances to be scaled in with dedicated names in the list.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Represents a list of claims that pods are allowed to reference.
	// The ReplicatedStateMachine controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// Controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down.
	//
	// The default policy is `OrderedReady`, where pods are created in increasing order and the controller waits until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// The alternative policy is `Parallel` which will create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	//
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the ReplicatedStateMachine when a revision is made to
	// Template.
	// UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil
	UpdateStrategy appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// A list of roles defined in the system.
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// Provides method to probe role.
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Provides actions to do membership dynamic reconfiguration.
	// +optional
	MembershipReconfiguration *MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// Members(Pods) update strategy.
	//
	// - serial: update Members one by one that guarantee minimum component unavailable time.
	// - bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
	// - parallel: force parallel
	//
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`

	// Indicates that the ReplicatedStateMachine is paused, meaning the reconciliation of this ReplicatedStateMachine object will be paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Credential used to connect to DB engine
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// ReplicatedStateMachineStatus defines the observed state of ReplicatedStateMachine
type ReplicatedStateMachineStatus struct {
	appsv1.StatefulSetStatus `json:",inline"`

	// Defines the initial number of pods (members) when the cluster is first initialized.
	// This value is set to spec.Replicas at the time of object creation and remains constant thereafter.
	InitReplicas int32 `json:"initReplicas"`

	// Represents the number of pods (members) that have already reached the MembersStatus during the cluster initialization stage.
	// This value remains constant once it equals InitReplicas.
	//
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// When not empty, indicates the version of the ReplicatedStateMachine used to generate the underlying workload.
	//
	// +optional
	CurrentGeneration int64 `json:"currentGeneration,omitempty"`

	// Provides the status of each member in the cluster.
	//
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`

	// currentRevisions, if not empty, indicates the old version of the ReplicatedStateMachine used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	CurrentRevisions map[string]string `json:"currentRevisions,omitempty"`

	// updateRevisions, if not empty, indicates the new version of the ReplicatedStateMachine used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	UpdateRevisions map[string]string `json:"updateRevisions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},shortName=its
// +kubebuilder:printcolumn:name="LEADER",type="string",JSONPath=".status.membersStatus[?(@.role.isLeader==true)].podName",description="leader pod name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.replicas",description="total replicas."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ReplicatedStateMachine is the Schema for the replicatedstatemachines API.
type ReplicatedStateMachine struct {
	// The metadata for the type, like API version and kind.
	metav1.TypeMeta `json:",inline"`

	// Contains the metadata for the particular object, such as name, namespace, labels, and annotations.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of the state machine. It includes the configuration details for the state machine.
	//
	Spec ReplicatedStateMachineSpec `json:"spec,omitempty"`

	// Represents the current information about the state machine. This data may be out of date.
	//
	Status ReplicatedStateMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReplicatedStateMachineList contains a list of ReplicatedStateMachine
type ReplicatedStateMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicatedStateMachine `json:"items"`
}

type ReplicaRole struct {

	// Defines the role name of the replica.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// Specifies the service capabilities of this member.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode"`

	// Indicates if this member has voting rights.
	//
	// +kubebuilder:default=true
	// +optional
	CanVote bool `json:"canVote"`

	// Determines if this member is the leader.
	//
	// +kubebuilder:default=false
	// +optional
	IsLeader bool `json:"isLeader"`
}

// AccessMode defines SVC access mode enums.
// +enum
type AccessMode string

const (
	ReadWriteMode AccessMode = "ReadWrite"
	ReadonlyMode  AccessMode = "Readonly"
	NoneMode      AccessMode = "None"
)

// MemberUpdateStrategy defines Cluster Component update strategy.
// +enum
type MemberUpdateStrategy string

const (
	SerialUpdateStrategy             MemberUpdateStrategy = "Serial"
	BestEffortParallelUpdateStrategy MemberUpdateStrategy = "BestEffortParallel"
	ParallelUpdateStrategy           MemberUpdateStrategy = "Parallel"
)

// RoleUpdateMechanism defines the way how pod role label being updated.
// +enum
type RoleUpdateMechanism string

const (
	ReadinessProbeEventUpdate  RoleUpdateMechanism = "ReadinessProbeEventUpdate"
	DirectAPIServerEventUpdate RoleUpdateMechanism = "DirectAPIServerEventUpdate"
)

// RoleProbe defines how to observe role
type RoleProbe struct {
	// Specifies the builtin handler name to use to probe the role of the main container.
	// Available handlers include: mysql, postgres, mongodb, redis, etcd, kafka.
	// Use CustomHandler to define a custom role probe function if none of the built-in handlers meet the requirement.
	//
	// +optional
	BuiltinHandler *string `json:"builtinHandlerName,omitempty"`

	// Defines a custom method for role probing.
	// If the BuiltinHandler meets the requirement, use it instead.
	// Actions defined here are executed in series.
	// Upon completion of all actions, the final output should be a single string representing the role name defined in spec.Roles.
	// The latest [BusyBox](https://busybox.net/) image will be used if Image is not configured.
	// Environment variables can be used in Command:
	// - v_KB_ITS_LAST_STDOUT: stdout from the last action, watch for 'v_' prefix
	// - KB_ITS_USERNAME: username part of the credential
	// - KB_ITS_PASSWORD: password part of the credential
	//
	// +optional
	CustomHandler []Action `json:"customHandler,omitempty"`

	// Specifies the number of seconds to wait after the container has started before initiating role probing.
	//
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// Specifies the number of seconds after which the probe times out.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// Specifies the frequency (in seconds) of probe execution.
	//
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Specifies the minimum number of consecutive successes for the probe to be considered successful after having failed.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// Specifies the minimum number of consecutive failures for the probe to be considered failed after having succeeded.
	//
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// Specifies the method for updating the pod role label.
	//
	// +kubebuilder:default=ReadinessProbeEventUpdate
	// +kubebuilder:validation:Enum={ReadinessProbeEventUpdate, DirectAPIServerEventUpdate}
	// +optional
	RoleUpdateMechanism RoleUpdateMechanism `json:"roleUpdateMechanism,omitempty"`
}

type Credential struct {
	// Defines the user's name for the credential.
	// The corresponding environment variable will be KB_ITS_USERNAME.
	//
	// +kubebuilder:validation:Required
	Username CredentialVar `json:"username"`

	// Represents the user's password for the credential.
	// The corresponding environment variable will be KB_ITS_PASSWORD.
	//
	// +kubebuilder:validation:Required
	Password CredentialVar `json:"password"`
}

type CredentialVar struct {
	// Specifies the value of the environment variable. This field is optional and defaults to an empty string.
	// The value can include variable references in the format $(VAR_NAME) which will be expanded using previously defined environment variables in the container and any service environment variables.
	//
	// If a variable cannot be resolved, the reference in the input string will remain unchanged.
	// Double $$ can be used to escape the $(VAR_NAME) syntax, resulting in a single $ and producing the string literal "$(VAR_NAME)".
	// Escaped references will not be expanded, regardless of whether the variable exists or not.
	//
	// +optional
	Value string `json:"value,omitempty"`

	// Defines the source for the environment variable's value. This field is optional and cannot be used if the 'Value' field is not empty.
	//
	// +optional
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty"`
}

type MembershipReconfiguration struct {
	// Specifies the environment variables that can be used in all following Actions:
	// - KB_ITS_USERNAME: Represents the username part of the credential
	// - KB_ITS_PASSWORD: Represents the password part of the credential
	// - KB_ITS_LEADER_HOST: Represents the leader host
	// - KB_ITS_TARGET_HOST: Represents the target host
	// - KB_ITS_SERVICE_PORT: Represents the service port
	//
	// Defines the action to perform a switchover.
	// If the Image is not configured, the latest [BusyBox](https://busybox.net/) image will be used.
	//
	// +optional
	SwitchoverAction *Action `json:"switchoverAction,omitempty"`

	// Defines the action to add a member.
	// If the Image is not configured, the Image from the previous non-nil action will be used.
	//
	// +optional
	MemberJoinAction *Action `json:"memberJoinAction,omitempty"`

	// Defines the action to remove a member.
	// If the Image is not configured, the Image from the previous non-nil action will be used.
	//
	// +optional
	MemberLeaveAction *Action `json:"memberLeaveAction,omitempty"`

	// Defines the action to trigger the new member to start log syncing.
	// If the Image is not configured, the Image from the previous non-nil action will be used.
	//
	// +optional
	LogSyncAction *Action `json:"logSyncAction,omitempty"`

	// Defines the action to inform the cluster that the new member can join voting now.
	// If the Image is not configured, the Image from the previous non-nil action will be used.
	//
	// +optional
	PromoteAction *Action `json:"promoteAction,omitempty"`
}

type Action struct {
	// Refers to the utility image that contains the command which can be utilized to retrieve or process role information.
	//
	// +optional
	Image string `json:"image,omitempty"`

	// A set of instructions that will be executed within the Container to retrieve or process role information. This field is required.
	//
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// Additional parameters used to perform specific statements. This field is optional.
	//
	// +optional
	Args []string `json:"args,omitempty"`
}

type MemberStatus struct {
	// Represents the name of the pod.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`

	// Defines the role of the replica in the cluster.
	//
	// +optional
	ReplicaRole *ReplicaRole `json:"role,omitempty"`

	// Whether the corresponding Pod is in ready condition.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Indicates whether it is required for the ReplicatedStateMachine to have at least one primary pod ready.
	//
	// +optional
	ReadyWithoutPrimary bool `json:"readyWithoutPrimary"`
}

func init() {
	SchemeBuilder.Register(&ReplicatedStateMachine{}, &ReplicatedStateMachineList{})
}
