/*
Copyright ApeCloud, Inc.

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
	apps "k8s.io/api/apps/v1"
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

	// Leader, one single leader.
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader"`

	// Followers, has voting right but not Leader.
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Learner, no voting right.
	// +optional
	Learner *ConsensusMember `json:"learner,omitempty"`

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
	apps.StatefulSetStatus `json:"apps.StatefulSetStatus"`

	// leader status.
	// +optional
	Leader ConsensusMemberStatus `json:"leader"`

	// followers status.
	// +optional
	Followers []ConsensusMemberStatus `json:"followers,omitempty"`

	// learner status.
	// +optional
	Learner *ConsensusMemberStatus `json:"learner,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ConsensusSet is the Schema for the consensussets API
type ConsensusSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsensusSetSpec   `json:"spec,omitempty"`
	Status ConsensusSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConsensusSetList contains a list of ConsensusSet
type ConsensusSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConsensusSet `json:"items"`
}

type ConsensusMember struct {
	// Name, role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=leader
	Name string `json:"name"`

	// AccessMode, what service this member capable.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=ReadWrite
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode"`

	// Replicas, number of Pods of this role.
	// default 1 for Leader
	// default 0 for Learner
	// default Components[*].Replicas - Leader.Replicas - Learner.Replicas for Followers
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
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
	// RoleName role name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	RoleName string `json:"roleName"`

	// accessMode, what service this pod provides.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	// +kubebuilder:default=None
	AccessMode AccessMode `json:"accessMode"`

	// PodName pod name.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=Unknown
	PodName string `json:"podName"`
}

func init() {
	SchemeBuilder.Register(&ConsensusSet{}, &ConsensusSetList{})
}
