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

// StatefulReplicaSetSpec defines the desired state of StatefulReplicaSet
type StatefulReplicaSetSpec struct {
	// Replicas defines number of Pods
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// service defines the behavior of a service spec.
	// provides read-write service
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Required
	Service corev1.ServiceSpec `json:"service"`

	Template corev1.PodTemplateSpec `json:"template"`

	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The StatefulReplicaSet controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// Roles, a list of roles defined in the system.
	// +kubebuilder:validation:Required
	Roles []ReplicaRole `json:"roles"`

	// RoleObservation provides method to observe role.
	// +kubebuilder:validation:Required
	RoleObservation RoleObservation `json:"roleObservation"`

	// MembershipReconfiguration provides actions to do membership dynamic reconfiguration.
	// +optional
	MembershipReconfiguration *MembershipReconfiguration `json:"membershipReconfiguration,omitempty"`

	// UpdateStrategy, Pods update strategy.
	// serial: update Pods one by one that guarantee minimum component unavailable time.
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time.
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time.
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`

	// Credential used to connect to DB engine
	// +optional
	Credential *Credential `json:"credential,omitempty"`
}

// StatefulReplicaSetStatus defines the observed state of StatefulReplicaSet
type StatefulReplicaSetStatus struct {
	appsv1.StatefulSetStatus `json:",inline"`

	// InitReplicas is the number of pods(members) when cluster first initialized
	// it's set to spec.Replicas at object creation time and never changes
	InitReplicas int32 `json:"initReplicas"`

	// ReadyInitReplicas is the number of pods(members) already in MembersStatus in the cluster initialization stage
	// will never change once equals to InitReplicas
	// +optional
	ReadyInitReplicas int32 `json:"readyInitReplicas,omitempty"`

	// members' status.
	// +optional
	MembersStatus []MemberStatus `json:"membersStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=srs
// +kubebuilder:printcolumn:name="LEADER",type="string",JSONPath=".status.membersStatus[?(@.role.isLeader==true)].podName",description="leader pod name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.replicas",description="total replicas."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// StatefulReplicaSet is the Schema for the statefulreplicasets API
type StatefulReplicaSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StatefulReplicaSetSpec   `json:"spec,omitempty"`
	Status StatefulReplicaSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StatefulReplicaSetList contains a list of StatefulReplicaSet
type StatefulReplicaSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StatefulReplicaSet `json:"items"`
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

// UpdateStrategy defines Cluster Component update strategy.
// +enum
type UpdateStrategy string

const (
	SerialUpdateStrategy             UpdateStrategy = "Serial"
	BestEffortParallelUpdateStrategy UpdateStrategy = "BestEffortParallel"
	ParallelUpdateStrategy           UpdateStrategy = "Parallel"
)

// RoleObservation defines how to observe role
type RoleObservation struct {
	// ObservationActions define Actions to be taken in serial.
	// after all actions done, the final output should be a single string of the role name defined in spec.Roles
	// latest [BusyBox](https://busybox.net/) image will be used if Image not configured
	// Environment variables can be used in Command:
	// - v_KB_SRS_LAST_STDOUT stdout from last action, watch 'v_' prefixed
	// - KB_SRS_USERNAME username part of credential
	// - KB_SRS_PASSWORD password part of credential
	// +kubebuilder:validation:Required
	ObservationActions []Action `json:"observationActions"`

	// Number of seconds after the container has started before role observation has started.
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`

	// Number of seconds after which the observation times out.
	// Defaults to 1 second. Minimum value is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

	// How often (in seconds) to perform the observation.
	// Default to 2 seconds. Minimum value is 1.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`

	// Minimum consecutive successes for the observation to be considered successful after having failed.
	// Minimum consecutive successes for the observation to be considered successful after having failed.
	// Defaults to 1. Minimum value is 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`

	// Minimum consecutive failures for the observation to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

type Credential struct {
	// Username
	// variable name will be KB_SRS_USERNAME
	// +kubebuilder:validation:Required
	Username CredentialVar `json:"username"`

	// Password
	// variable name will be KB_SRS_PASSWORD
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
	// - KB_SRS_USERNAME username part of credential
	// - KB_SRS_PASSWORD password part of credential
	// - KB_SRS_LEADER_HOST leader host
	// - KB_SRS_TARGET_HOST target host
	// - KB_SRS_SERVICE_PORT port

	// SwitchoverAction specifies how to do switchover
	// latest [BusyBox](https://busybox.net/) image will be used if Image not configured
	// +optional
	SwitchoverAction *Action `json:"switchoverAction,omitempty"`

	// MemberJoinAction specifies how to add member
	// previous none-nil action's Image wil be used if not configured
	// +optional
	MemberJoinAction *Action `json:"memberJoinAction,omitempty"`

	// MemberLeaveAction specifies how to remove member
	// previous none-nil action's Image wil be used if not configured
	// +optional
	MemberLeaveAction *Action `json:"memberLeaveAction,omitempty"`

	// LogSyncAction specifies how to trigger the new member to start log syncing
	// previous none-nil action's Image wil be used if not configured
	// +optional
	LogSyncAction *Action `json:"logSyncAction,omitempty"`

	// PromoteAction specifies how to tell the cluster that the new member can join voting now
	// previous none-nil action's Image wil be used if not configured
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
	SchemeBuilder.Register(&StatefulReplicaSet{}, &StatefulReplicaSetList{})
}
