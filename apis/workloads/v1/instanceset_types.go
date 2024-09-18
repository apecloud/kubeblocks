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

package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks},shortName=its
// +kubebuilder:printcolumn:name="LEADER",type="string",JSONPath=".status.membersStatus[?(@.role.isLeader==true)].podName",description="leader instance name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.replicas",description="total replicas."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// InstanceSet is the Schema for the instancesets API.
type InstanceSet struct {
	// The metadata for the type, like API version and kind.
	metav1.TypeMeta `json:",inline"`

	// Contains the metadata for the particular object, such as name, namespace, labels, and annotations.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Defines the desired state of the state machine. It includes the configuration details for the state machine.
	//
	Spec InstanceSetSpec `json:"spec,omitempty"`

	// Represents the current information about the state machine. This data may be out of date.
	//
	Status InstanceSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceSetList contains a list of InstanceSet
type InstanceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InstanceSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InstanceSet{}, &InstanceSetList{})
}

// InstanceSetSpec defines the desired state of InstanceSet
type InstanceSetSpec struct {
	// Specifies the desired number of replicas of the given Template.
	// These replicas are instantiations of the same Template, with each having a consistent identity.
	// Defaults to 1 if unspecified.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the desired Ordinals of the default template.
	// The Ordinals used to specify the ordinal of the instance (pod) names to be generated under the default template.
	//
	// For example, if Ordinals is {ranges: [{start: 0, end: 1}], discrete: [7]},
	// then the instance names generated under the default template would be
	// $(cluster.name)-$(component.name)-0、$(cluster.name)-$(component.name)-1 and $(cluster.name)-$(component.name)-7
	DefaultTemplateOrdinals Ordinals `json:"defaultTemplateOrdinals,omitempty"`

	// Defines the minimum number of seconds a newly created pod should be ready
	// without any of its container crashing to be considered available.
	// Defaults to 0, meaning the pod will be considered available as soon as it is ready.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// Represents a label query over pods that should match the desired replica count indicated by the `replica` field.
	// It must match the labels defined in the pod template.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	// Defines the behavior of a service spec.
	// Provides read-write service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	//
	// Note: This field will be removed in future version.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Service *corev1.Service `json:"service,omitempty"`

	Template corev1.PodTemplateSpec `json:"template"`

	// Overrides values in default Template.
	//
	// Instance is the fundamental unit managed by KubeBlocks.
	// It represents a Pod with additional objects such as PVCs, Services, ConfigMaps, etc.
	// An InstanceSet manages instances with a total count of Replicas,
	// and by default, all these instances are generated from the same template.
	// The InstanceTemplate provides a way to override values in the default template,
	// allowing the InstanceSet to manage instances from different templates.
	//
	// The naming convention for instances (pods) based on the InstanceSet Name, InstanceTemplate Name, and ordinal.
	// The constructed instance name follows the pattern: $(instance_set.name)-$(template.name)-$(ordinal).
	// By default, the ordinal starts from 0 for each InstanceTemplate.
	// It is important to ensure that the Name of each InstanceTemplate is unique.
	//
	// The sum of replicas across all InstanceTemplates should not exceed the total number of Replicas specified for the InstanceSet.
	// Any remaining replicas will be generated using the default template and will follow the default naming rules.
	//
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	Instances []InstanceTemplate `json:"instances,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Specifies the names of instances to be transitioned to offline status.
	//
	// Marking an instance as offline results in the following:
	//
	// 1. The associated pod is stopped, and its PersistentVolumeClaim (PVC) is retained for potential
	//    future reuse or data recovery, but it is no longer actively used.
	// 2. The ordinal number assigned to this instance is preserved, ensuring it remains unique
	//    and avoiding conflicts with new instances.
	//
	// Setting instances to offline allows for a controlled scale-in process, preserving their data and maintaining
	// ordinal consistency within the cluster.
	// Note that offline instances and their associated resources, such as PVCs, are not automatically deleted.
	// The cluster administrator must manually manage the cleanup and removal of these resources when they are no longer needed.
	//
	// +optional
	OfflineInstances []string `json:"offlineInstances,omitempty"`

	// Specifies a list of PersistentVolumeClaim templates that define the storage requirements for each replica.
	// Each template specifies the desired characteristics of a persistent volume, such as storage class,
	// size, and access modes.
	// These templates are used to dynamically provision persistent volumes for replicas upon their creation.
	// The final name of each PVC is generated by appending the pod's identifier to the name specified in volumeClaimTemplates[*].name.
	//
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
	// Note: This field will be removed in future version.
	//
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// Controls the concurrency of pods during initial scale up, when replacing pods on nodes,
	// or when scaling down. It only used when `PodManagementPolicy` is set to `Parallel`.
	// The default Concurrency is 100%.
	//
	// +optional
	ParallelPodManagementConcurrency *intstr.IntOrString `json:"parallelPodManagementConcurrency,omitempty"`

	// PodUpdatePolicy indicates how pods should be updated
	//
	// - `StrictInPlace` indicates that only allows in-place upgrades.
	// Any attempt to modify other fields will be rejected.
	// - `PreferInPlace` indicates that we will first attempt an in-place upgrade of the Pod.
	// If that fails, it will fall back to the ReCreate, where pod will be recreated.
	// Default value is "PreferInPlace"
	//
	// +optional
	PodUpdatePolicy PodUpdatePolicyType `json:"podUpdatePolicy,omitempty"`

	// Indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the InstanceSet when a revision is made to
	// Template.
	// UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil
	//
	// Note: This field will be removed in future version.
	UpdateStrategy appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// A list of roles defined in the system.
	//
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// Provides method to probe role.
	//
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// Provides actions to do membership dynamic reconfiguration.
	//
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

	// Indicates that the InstanceSet is paused, meaning the reconciliation of this InstanceSet object will be paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Credential used to connect to DB engine
	//
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// InstanceSetStatus defines the observed state of InstanceSet
type InstanceSetStatus struct {
	// observedGeneration is the most recent generation observed for this InstanceSet. It corresponds to the
	// InstanceSet's generation, which is updated on mutation by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// replicas is the number of instances created by the InstanceSet controller.
	Replicas int32 `json:"replicas"`

	// readyReplicas is the number of instances created for this InstanceSet with a Ready Condition.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// currentReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by CurrentRevisions.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// updatedReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by UpdateRevisions.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// currentRevision, if not empty, indicates the version of the InstanceSet used to generate instances in the
	// sequence [0,currentReplicas).
	CurrentRevision string `json:"currentRevision,omitempty"`

	// updateRevision, if not empty, indicates the version of the InstanceSet used to generate instances in the sequence
	// [replicas-updatedReplicas,replicas)
	UpdateRevision string `json:"updateRevision,omitempty"`

	// Represents the latest available observations of an instanceset's current state.
	// Known .status.conditions.type are: "InstanceFailure", "InstanceReady"
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// Total number of available instances (ready for at least minReadySeconds) targeted by this InstanceSet.
	//
	// +optional
	AvailableReplicas int32 `json:"availableReplicas"`

	// Defines the initial number of instances when the cluster is first initialized.
	// This value is set to spec.Replicas at the time of object creation and remains constant thereafter.
	// Used only when spec.roles set.
	//
	// +optional
	InitReplicas int32 `json:"initReplicas"`

	// Represents the number of instances that have already reached the MembersStatus during the cluster initialization stage.
	// This value remains constant once it equals InitReplicas.
	// Used only when spec.roles set.
	//
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// Provides the status of each member in the cluster.
	//
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`

	// Indicates whether it is required for the InstanceSet to have at least one primary instance ready.
	//
	// +optional
	ReadyWithoutPrimary bool `json:"readyWithoutPrimary,omitempty"`

	// currentRevisions, if not empty, indicates the old version of the InstanceSet used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	CurrentRevisions map[string]string `json:"currentRevisions,omitempty"`

	// updateRevisions, if not empty, indicates the new version of the InstanceSet used to generate the underlying workload.
	// key is the pod name, value is the revision.
	//
	// +optional
	UpdateRevisions map[string]string `json:"updateRevisions,omitempty"`

	// TemplatesStatus represents status of each instance generated by InstanceTemplates
	// +optional
	TemplatesStatus []InstanceTemplateStatus `json:"templatesStatus,omitempty"`

	// LifeCycleActionsStatus represents the execution status of lifecycle actions for each pod
	// +optional
	LifeCycleActionsStatus map[string]ActionStatus `json:"LifeCycleActionsStatus,omitempty"`
}

type ActionStatus struct {
	MemberLeaveStatus string
}

// Range represents a range with a start and an end value.
// It is used to define a continuous segment.
type Range struct {
	Start int32 `json:"start"`
	End   int32 `json:"end"`
}

// Ordinals represents a combination of continuous segments and individual values.
type Ordinals struct {
	Ranges   []Range `json:"ranges,omitempty"`
	Discrete []int32 `json:"discrete,omitempty"`
}

// InstanceTemplate allows customization of individual replica configurations within a Component,
// without altering the base component template defined in ClusterComponentSpec.
// It enables the application of distinct settings to specific instances (replicas),
// providing flexibility while maintaining a common configuration baseline.
type InstanceTemplate struct {
	// Name specifies the unique name of the instance Pod created using this InstanceTemplate.
	// This name is constructed by concatenating the component's name, the template's name, and the instance's ordinal
	// using the pattern: $(cluster.name)-$(component.name)-$(template.name)-$(ordinal). Ordinals start from 0.
	// The specified name overrides any default naming conventions or patterns.
	//
	// +kubebuilder:validation:MaxLength=54
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the number of instances (Pods) to create from this InstanceTemplate.
	// This field allows setting how many replicated instances of the component,
	// with the specific overrides in the InstanceTemplate, are created.
	// The default value is 1. A value of 0 disables instance creation.
	//
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Specifies the desired Ordinals of this InstanceTemplate.
	// The Ordinals used to specify the ordinal of the instance (pod) names to be generated under this InstanceTemplate.
	//
	// For example, if Ordinals is {ranges: [{start: 0, end: 1}], discrete: [7]},
	// then the instance names generated under this InstanceTemplate would be
	// $(cluster.name)-$(component.name)-$(template.name)-0、$(cluster.name)-$(component.name)-$(template.name)-1 and
	// $(cluster.name)-$(component.name)-$(template.name)-7
	Ordinals Ordinals `json:"ordinals,omitempty"`

	// Specifies a map of key-value pairs to be merged into the Pod's existing annotations.
	// Existing keys will have their values overwritten, while new keys will be added to the annotations.
	//
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specifies a map of key-value pairs that will be merged into the Pod's existing labels.
	// Values for existing keys will be overwritten, and new keys will be added.
	//
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Specifies an override for the first container's image in the pod.
	//
	// +optional
	Image *string `json:"image,omitempty"`

	// Specifies the scheduling policy for the Component.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies an override for the resource requirements of the first container in the Pod.
	// This field allows for customizing resource allocation (CPU, memory, etc.) for the container.
	//
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

// SchedulingPolicy the scheduling policy.
// Deprecated: Unify with apps/v1alpha1.SchedulingPolicy
type SchedulingPolicy struct {
	// If specified, the Pod will be dispatched by specified scheduler.
	// If not specified, the Pod will be dispatched by default scheduler.
	//
	// +optional
	SchedulerName string `json:"schedulerName,omitempty"`

	// NodeSelector is a selector which must be true for the Pod to fit on a node.
	// Selector which must match a node's labels for the Pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	//
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NodeName is a request to schedule this Pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this Pod onto that node, assuming that it fits resource
	// requirements.
	//
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Specifies a group of affinity scheduling rules of the Cluster, including NodeAffinity, PodAffinity, and PodAntiAffinity.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Allows Pods to be scheduled onto nodes with matching taints.
	// Each toleration in the array allows the Pod to tolerate node taints based on
	// specified `key`, `value`, `effect`, and `operator`.
	//
	// - The `key`, `value`, and `effect` identify the taint that the toleration matches.
	// - The `operator` determines how the toleration matches the taint.
	//
	// Pods with matching tolerations are allowed to be scheduled on tainted nodes, typically reserved for specific purposes.
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// TopologySpreadConstraints describes how a group of Pods ought to spread across topology
	// domains. Scheduler will schedule Pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	//
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type PodUpdatePolicyType string

const (
	// StrictInPlacePodUpdatePolicyType indicates that only allows in-place upgrades.
	// Any attempt to modify other fields will be rejected.
	StrictInPlacePodUpdatePolicyType PodUpdatePolicyType = "StrictInPlace"

	// PreferInPlacePodUpdatePolicyType indicates that we will first attempt an in-place upgrade of the Pod.
	// If that fails, it will fall back to the ReCreate, where pod will be recreated.
	PreferInPlacePodUpdatePolicyType PodUpdatePolicyType = "PreferInPlace"
)

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

// RoleProbe defines how to observe role
type RoleProbe struct {
	// Defines a custom method for role probing.
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

// RoleUpdateMechanism defines the way how pod role label being updated.
// +enum
type RoleUpdateMechanism string

const (
	ReadinessProbeEventUpdate  RoleUpdateMechanism = "ReadinessProbeEventUpdate"
	DirectAPIServerEventUpdate RoleUpdateMechanism = "DirectAPIServerEventUpdate"
)

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

// MemberUpdateStrategy defines Cluster Component update strategy.
// +enum
type MemberUpdateStrategy string

const (
	SerialUpdateStrategy             MemberUpdateStrategy = "Serial"
	BestEffortParallelUpdateStrategy MemberUpdateStrategy = "BestEffortParallel"
	ParallelUpdateStrategy           MemberUpdateStrategy = "Parallel"
)

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
}

// InstanceTemplateStatus aggregates the status of replicas for each InstanceTemplate
type InstanceTemplateStatus struct {
	// Name, the name of the InstanceTemplate.
	Name string `json:"name"`

	// Replicas is the number of replicas of the InstanceTemplate.
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of Pods that have a Ready Condition.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of Pods that ready for at least minReadySeconds.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// currentReplicas is the number of instances created by the InstanceSet controller from the InstanceSet version
	// indicated by CurrentRevisions.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// UpdatedReplicas is the number of Pods created by the InstanceSet controller from the InstanceSet version
	// indicated by UpdateRevisions.
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
}

type ConditionType string

const (
	// InstanceReady is added in an instance set when at least one of its instances(pods) is in a Ready condition.
	// ConditionStatus will be True if all its instances(pods) are in a Ready condition.
	// Or, a NotReady reason with not ready instances encoded in the Message filed will be set.
	InstanceReady ConditionType = "InstanceReady"

	// InstanceAvailable ConditionStatus will be True if all instances(pods) are in the ready condition
	// and continue for "MinReadySeconds" seconds. Otherwise, it will be set to False.
	InstanceAvailable ConditionType = "InstanceAvailable"

	// InstanceFailure is added in an instance set when at least one of its instances(pods) is in a `Failed` phase.
	InstanceFailure ConditionType = "InstanceFailure"

	// InstanceUpdateRestricted represents a ConditionType that indicates updates to an InstanceSet are blocked(when the
	// PodUpdatePolicy is set to StrictInPlace but the pods cannot be updated in-place).
	InstanceUpdateRestricted ConditionType = "InstanceUpdateRestricted"
)

const (
	// ReasonNotReady is a reason for condition InstanceReady.
	ReasonNotReady = "NotReady"

	// ReasonReady is a reason for condition InstanceReady.
	ReasonReady = "Ready"

	// ReasonNotAvailable is a reason for condition InstanceAvailable.
	ReasonNotAvailable = "NotAvailable"

	// ReasonAvailable is a reason for condition InstanceAvailable.
	ReasonAvailable = "Available"

	// ReasonInstanceFailure is a reason for condition InstanceFailure.
	ReasonInstanceFailure = "InstanceFailure"

	// ReasonInstanceUpdateRestricted is a reason for condition InstanceUpdateRestricted.
	ReasonInstanceUpdateRestricted = "InstanceUpdateRestricted"
)

const defaultInstanceTemplateReplicas = 1

func (t *InstanceTemplate) GetName() string {
	return t.Name
}

func (t *InstanceTemplate) GetReplicas() int32 {
	if t.Replicas != nil {
		return *t.Replicas
	}
	return defaultInstanceTemplateReplicas
}

func (t *InstanceTemplate) GetOrdinals() Ordinals {
	return t.Ordinals
}
