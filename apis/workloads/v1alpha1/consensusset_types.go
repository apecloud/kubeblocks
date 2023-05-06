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

// ConsensusSetSpec defines the desired state of ConsensusSet
type ConsensusSetSpec struct {
	// Replicas defines number of Pods
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// service defines the behavior of a service spec.
	// provides read-write service
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Service *corev1.ServiceSpec `json:"service,omitempty"`

	Template corev1.PodTemplateSpec `json:"template"`

	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The ConsensusSet controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template. A claim in this list takes precedence over
	// any volumes in the template, with the same name.
	// +optional
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`

	// Roles, a list of roles defined in this consensus system.
	// +kubebuilder:validation:Required
	Roles []ConsensusRole `json:"roles"`

	// RoleObservation provides method to observe role.
	RoleObservation RoleObservation `json:"roleObservation"`

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
}

// ConsensusSetStatus defines the observed state of ConsensusSet
type ConsensusSetStatus struct {
	// observedGeneration is the most recent generation observed for this StatefulSet. It corresponds to the
	// StatefulSet's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	// replicas is the number of Pods created by the StatefulSet controller.
	Replicas int32 `json:"replicas" protobuf:"varint,2,opt,name=replicas"`

	// readyReplicas is the number of pods created for this StatefulSet with a Ready Condition.
	ReadyReplicas int32 `json:"readyReplicas,omitempty" protobuf:"varint,3,opt,name=readyReplicas"`

	// currentReplicas is the number of Pods created by the StatefulSet controller from the StatefulSet version
	// indicated by currentRevision.
	CurrentReplicas int32 `json:"currentReplicas,omitempty" protobuf:"varint,4,opt,name=currentReplicas"`

	// updatedReplicas is the number of Pods created by the StatefulSet controller from the StatefulSet version
	// indicated by updateRevision.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty" protobuf:"varint,5,opt,name=updatedReplicas"`

	// currentRevision, if not empty, indicates the version of the StatefulSet used to generate Pods in the
	// sequence [0,currentReplicas).
	CurrentRevision string `json:"currentRevision,omitempty" protobuf:"bytes,6,opt,name=currentRevision"`

	// updateRevision, if not empty, indicates the version of the StatefulSet used to generate Pods in the sequence
	// [replicas-updatedReplicas,replicas)
	UpdateRevision string `json:"updateRevision,omitempty" protobuf:"bytes,7,opt,name=updateRevision"`

	// collisionCount is the count of hash collisions for the StatefulSet. The StatefulSet controller
	// uses this field as a collision avoidance mechanism when it needs to create the name for the
	// newest ControllerRevision.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty" protobuf:"varint,9,opt,name=collisionCount"`

	// Represents the latest available observations of a statefulset's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []appsv1.StatefulSetCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,10,rep,name=conditions"`

	// Total number of available pods (ready for at least minReadySeconds) targeted by this statefulset.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas" protobuf:"varint,11,opt,name=availableReplicas"`

	// members' status.
	// +optional
	MembersStatus []ConsensusMemberStatus `json:"membersStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=csset
// +kubebuilder:printcolumn:name="LEADER",type="string",JSONPath=".status.membersStatus[0].podName",description="leader pod name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.readyReplicas",description="ready replicas."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.replicas",description="total replicas."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ConsensusSet is the Schema for the consensussets API
type ConsensusSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsensusSetSpec   `json:"spec,omitempty"`
	Status ConsensusSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConsensusSetList contains a list of ConsensusSet
type ConsensusSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConsensusSet `json:"items"`
}

type ConsensusRole struct {
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

// AccessMode define SVC access mode enums.
// +enum
type AccessMode string

const (
	ReadWriteMode AccessMode = "ReadWrite"
	ReadonlyMode  AccessMode = "Readonly"
	NoneMode      AccessMode = "None"
)

// UpdateStrategy define Cluster Component update strategy.
// +enum
type UpdateStrategy string

const (
	SerialUpdateStrategy             UpdateStrategy = "Serial"
	BestEffortParallelUpdateStrategy UpdateStrategy = "BestEffortParallel"
	ParallelUpdateStrategy           UpdateStrategy = "Parallel"
)

// RoleObservation defines method to observe role
type RoleObservation struct {
	Kind string `json:"kind"`

	// Container will be run as a sidecar container
	// +optional
	Container *corev1.Container `json:"container,omitempty"`

	// RoleProbe will be executed against Container to retrieve role info
	RoleProbe corev1.Probe `json:"roleProbe,omitempty"`

	// PostProcessCommand, processes role info returned from RoleProbe,
	// to get a single string of the role name.
	// all commands are from Container, or [BusyBox](https://busybox.net/) if Container not configured
	// +optional
	PostProcessCommand []string `json:"postProcessCommand,omitempty"`
}

type ConsensusMemberStatus struct {
	// PodName pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`

	ConsensusRole `json:"role"`
}

func init() {
	SchemeBuilder.Register(&ConsensusSet{}, &ConsensusSetList{})
}
