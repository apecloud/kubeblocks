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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReplicatedStateMachineSpec defines the desired state of ReplicatedStateMachine
type ReplicatedStateMachineSpec struct {
	// replicas is the desired number of replicas of the given Template.
	// These are replicas in the sense that they are instantiations of the
	// same Template, but individual replicas also have a consistent identity.
	// If unspecified, defaults to 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// selector is a label query over pods that should match the replica count.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	// serviceName is the name of the service that governs this StatefulSet.
	// This service must exist before the StatefulSet, and is responsible for
	// the network identity of the set. Pods get DNS/hostnames that follow the
	// pattern: pod-specific-string.serviceName.default.svc.cluster.local
	// where "pod-specific-string" is managed by the StatefulSet controller.
	ServiceName string `json:"serviceName"`

	// service defines the behavior of a service spec.
	// provides read-write service
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Service *corev1.Service `json:"service,omitempty"`

	// AlternativeServices defines Alternative Services selector pattern specifier.
	// can be used for creating Readonly service.
	// +optional
	AlternativeServices []corev1.Service `json:"alternativeServices,omitempty"`

	Template corev1.PodTemplateSpec `json:"template"`

	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The ReplicatedStateMachine controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// podManagementPolicy controls how pods are created during initial scale up,
	// when replacing pods on nodes, or when scaling down. The default policy is
	// `OrderedReady`, where pods are created in increasing order (pod-0, then
	// pod-1, etc) and the controller will wait until each pod is ready before
	// continuing. When scaling down, the pods are removed in the opposite order.
	// The alternative policy is `Parallel` which will create pods in parallel
	// to match the desired scale without waiting, and on scale down will delete
	// all pods at once.
	// +optional
	PodManagementPolicy appsv1.PodManagementPolicyType `json:"podManagementPolicy,omitempty"`

	// updateStrategy indicates the StatefulSetUpdateStrategy that will be
	// employed to update Pods in the RSM when a revision is made to
	// Template.
	// UpdateStrategy.Type will be set to appsv1.OnDeleteStatefulSetStrategyType if MemberUpdateStrategy is not nil
	UpdateStrategy appsv1.StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`

	// Roles, a list of roles defined in the system.
	// +optional
	Roles []ReplicaRole `json:"roles,omitempty"`

	// RoleProbe provides method to probe role.
	// +optional
	RoleProbe *RoleProbe `json:"roleProbe,omitempty"`

	// MembershipReconfiguration provides actions to do membership dynamic reconfiguration.
	// +optional
	MembershipReconfiguration *MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// MemberUpdateStrategy, Members(Pods) update strategy.
	// serial: update Members one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Members in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	MemberUpdateStrategy *MemberUpdateStrategy `json:"memberUpdateStrategy,omitempty"`

	// Paused indicates that the rsm is paused, means the reconciliation of this rsm object will be paused.
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Credential used to connect to DB engine
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// ReplicatedStateMachineStatus defines the observed state of ReplicatedStateMachine
type ReplicatedStateMachineStatus struct {
	appsv1.StatefulSetStatus `json:",inline"`

	// InitReplicas is the number of pods(members) when cluster first initialized
	// it's set to spec.Replicas at object creation time and never changes
	InitReplicas int32 `json:"initReplicas"`

	// ReadyInitReplicas is the number of pods(members) already in MembersStatus in the cluster initialization stage
	// will never change once equals to InitReplicas
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// CurrentGeneration, if not empty, indicates the version of the RSM used to generate the underlying workload
	// +optional
	CurrentGeneration int64 `json:"currentGeneration,omitempty"`

	// members' status.
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=rsm
// +kubebuilder:printcolumn:name="LEADER",type="string",JSONPath=".status.membersStatus[?(@.role.isLeader==true)].podName",description="leader pod name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.replicas",description="total replicas."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ReplicatedStateMachine is the Schema for the replicatedstatemachines API.
type ReplicatedStateMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicatedStateMachineSpec   `json:"spec,omitempty"`
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
	// Name, role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// AccessMode, what service this member capable.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode"`

	// CanVote, whether this member has voting rights
	// +kubebuilder:default=true
	// +optional
	CanVote bool `json:"canVote"`

	// IsLeader, whether this member is the leader
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
	// BuiltinHandler specifies the builtin handler name to use to probe the role of the main container.
	// current available handlers: mysql, postgres, mongodb, redis, etcd, kafka.
	// use CustomHandler to define your own role probe function if none of them satisfies the requirement.
	// +optional
	BuiltinHandler *string `json:"builtinHandlerName,omitempty"`

	// CustomHandler defines the custom way to do role probe.
	// if the BuiltinHandler satisfies the requirement, use it instead.
	//
	// how the actions defined here works:
	//
	// Actions will be taken in serial.
	// after all actions done, the final output should be a single string of the role name defined in spec.Roles
	// latest [BusyBox](https://busybox.net/) image will be used if Image not configured
	// Environment variables can be used in Command:
	// - v_KB_RSM_LAST_STDOUT stdout from last action, watch 'v_' prefixed
	// - KB_RSM_USERNAME username part of credential
	// - KB_RSM_PASSWORD password part of credential
	// +optional
	CustomHandler []Action `json:"customHandler,omitempty"`

	// Number of seconds after the container has started before role probe has started.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// How often (in seconds) to perform the probe.
	// Default to 2 seconds. Minimum value is 1.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Minimum value is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// RoleUpdateMechanism specifies the way how pod role label being updated.
	// +kubebuilder:default=ReadinessProbeEventUpdate
	// +kubebuilder:validation:Enum={ReadinessProbeEventUpdate, DirectAPIServerEventUpdate}
	// +optional
	RoleUpdateMechanism RoleUpdateMechanism `json:"roleUpdateMechanism,omitempty"`
}

type Credential struct {
	// Username
	// variable name will be KB_RSM_USERNAME
	// +kubebuilder:validation:Required
	Username CredentialVar `json:"username"`

	// Password
	// variable name will be KB_RSM_PASSWORD
	// +kubebuilder:validation:Required
	Password CredentialVar `json:"password"`
}

type CredentialVar struct {
	// Optional: no more than one of the following may be specified.

	// Variable references $(VAR_NAME) are expanded
	// using the previously defined environment variables in the container and
	// any service environment variables. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
	// "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	// Escaped references will never be expanded, regardless of whether the variable
	// exists or not.
	// Defaults to "".
	// +optional
	Value string `json:"value,omitempty"`

	// Source for the environment variable's value. Cannot be used if value is not empty.
	// +optional
	ValueFrom *corev1.EnvVarSource `json:"valueFrom,omitempty"`
}

type MembershipReconfiguration struct {
	// Environment variables can be used in all following Actions:
	// - KB_RSM_USERNAME username part of credential
	// - KB_RSM_PASSWORD password part of credential
	// - KB_RSM_LEADER_HOST leader host
	// - KB_RSM_TARGET_HOST target host
	// - KB_RSM_SERVICE_PORT port

	// SwitchoverAction specifies how to do switchover
	// latest [BusyBox](https://busybox.net/) image will be used if Image not configured
	// +optional
	SwitchoverAction *Action `json:"switchoverAction,omitempty"`

	// MemberJoinAction specifies how to add member
	// previous none-nil action's Image will be used if not configured
	// +optional
	MemberJoinAction *Action `json:"memberJoinAction,omitempty"`

	// MemberLeaveAction specifies how to remove member
	// previous none-nil action's Image will be used if not configured
	// +optional
	MemberLeaveAction *Action `json:"memberLeaveAction,omitempty"`

	// LogSyncAction specifies how to trigger the new member to start log syncing
	// previous none-nil action's Image will be used if not configured
	// +optional
	LogSyncAction *Action `json:"logSyncAction,omitempty"`

	// PromoteAction specifies how to tell the cluster that the new member can join voting now
	// previous none-nil action's Image will be used if not configured
	// +optional
	PromoteAction *Action `json:"promoteAction,omitempty"`
}

type Action struct {
	// utility image contains command that can be used to retrieve of process role info
	// +optional
	Image string `json:"image,omitempty"`

	// Command will be executed in Container to retrieve or process role info
	// +kubebuilder:validation:Required
	Command []string `json:"command"`
}

type MemberStatus struct {
	// PodName pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`

	ReplicaRole `json:"role"`
}

func init() {
	SchemeBuilder.Register(&ReplicatedStateMachine{}, &ReplicatedStateMachineList{})
}
