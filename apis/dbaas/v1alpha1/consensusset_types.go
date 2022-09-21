/*
Copyright 2022.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConsensusSetSpec defines the desired state of ConsensusSet
type ConsensusSetSpec struct {
	// Replicas, number of pods in this ConsensusSet
	// +kubebuilder:validation:Required
	// +kubebuilder:default=1
	Replicas int `json:"replicas,omitempty"`

	// Leader, one single leader
	// +kubebuilder:validation:Required
	Leader ConsensusMember `json:"leader,omitempty"`

	// Followers, has voting right but not Leader
	// +optional
	Followers []ConsensusMember `json:"followers,omitempty"`

	// Learner, no voting right
	// +optional
	Learner ConsensusMember `json:"learner,omitempty"`

	// UpdateStrategy, Pods update strategy
	// options: serial, bestEffortParallel, parallel
	// serial: update Pods one by one that guarantee minimum component unavailable time
	// 		Learner -> Follower(with AccessMode=none) -> Follower(with AccessMode=readonly) -> Follower(with AccessMode=readWrite) -> Leader
	// bestEffortParallel: update Pods in parallel that guarantee minimum component un-writable time
	//		Learner, Follower(minority) in parallel -> Follower(majority) -> Leader, keep majority online all the time
	// parallel: force parallel
	// +kubebuilder:default=Serial
	// +kubebuilder:validation:Enum={Serial,BestEffortParallel,Parallel}
	// +optional
	UpdateStrategy UpdateStrategy `json:"updateStrategy,omitempty"`
}

type ConsensusMember struct {
	// Name, role name
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// AccessMode, what service this member capable for
	// +kubebuilder:validation:Required
	// +kubebuilder:default=None
	// +kubebuilder:validation:Enum={None, Readonly, ReadWrite}
	AccessMode AccessMode `json:"accessMode,omitempty"`

	// Replicas, number of Pods of this role
	// default 1 for Leader
	// default 0 for Learner
	// default Components[*].Replicas - Leader.Replicas - Learner.Replicas for Followers
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`
}

type AccessMode string

const (
	ReadWrite AccessMode = "ReadWrite"
	Readonly  AccessMode = "Readonly"
	None      AccessMode = "None"
)

type UpdateStrategy string

const (
	Serial             UpdateStrategy = "Serial"
	BestEffortParallel UpdateStrategy = "BestEffortParallel"
	Parallel           UpdateStrategy = "Parallel"
)

// ConsensusSetStatus defines the observed state of ConsensusSet
type ConsensusSetStatus struct {

	// Replicas is the number of Pods created by the controller
	// +kubebuilder:validation:Required
	// +kubebuilder:default=0
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of pods created for this ConsensusSet with a Ready Condition.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=0
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// ReadyLeader, ready leader pod, 0 or 1
	// +kubebuilder:validation:Required
	// +kubebuilder:default=0
	// +kubebuilder:minimum=0
	// +kubebuilder:maximum=1
	ReadyLeader int32 `json:"readyLeader,omitempty"`

	// ReadyFollowers, ready follower pods
	// +kubebuilder:validation:Required
	// +kubebuilder:default=0
	ReadyFollowers int32 `json:"readyFollowers,omitempty"`

	// ReadyLearners, ready learner pods
	// +kubebuilder:validation:Required
	// +kubebuilder:default=0
	ReadyLearners int32 `json:"readyLearners,omitempty"`

	// IsReadWriteServiceReady, indicates readWrite service ready status
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	IsReadWriteServiceReady bool `json:"isReadWriteServiceReady,omitempty"`

	// IsReadonlyServiceReady, indicates readonly service ready status
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	IsReadonlyServiceReady bool `json:"isReadonlyServiceReady,omitempty"`

	// ConsensusSetCondition
	// +optional
	ConsensusSetConditions []ConsensusSetCondition `json:"consensusSetConditions,omitempty"`

	// ObservedGeneration is the most recent generation observed for this ConsensusSet.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type ConsensusSetConditionType string

type ConsensusSetCondition struct {
	// Type of consensusset condition.
	Type ConsensusSetConditionType `json:"type,omitempty"`

	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
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

func init() {
	SchemeBuilder.Register(&ConsensusSet{}, &ConsensusSetList{})
}
