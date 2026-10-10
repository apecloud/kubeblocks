/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// InstanceObjectReference identifies an observed Kubernetes object, independently of its logical instance name.
type InstanceObjectReference struct {
	// Name is the observed object name.
	Name string `json:"name"`
	// Namespace is the observed object namespace.
	Namespace string `json:"namespace"`
	// UID distinguishes replacements with the same name.
	UID types.UID `json:"uid"`
	// Cluster is the known cluster location. An empty string means the local cluster; nil means unknown.
	// +optional
	Cluster *string `json:"cluster,omitempty"`
}

// InstanceVolumeIdentity identifies one actual PVC and its persistent volume binding.
type InstanceVolumeIdentity struct {
	// OwnerUID is the observed controller owner of the PVC. It identifies whose retention policy can require cleanup;
	// it is not part of data identity and does not request deletion.
	// +optional
	OwnerUID types.UID `json:"ownerUID,omitempty"`
	// Name is the volume name used by the Pod or claim template.
	Name string `json:"name"`
	// Claim identifies the actual PVC. Nil means the claim has not been observed.
	// +optional
	Claim *InstanceObjectReference `json:"claim,omitempty"`
	// VolumeName is the observed PVC binding, independent of capacity and readiness.
	// +optional
	VolumeName string `json:"volumeName,omitempty"`
}

// InstanceStorageIdentity describes actual storage independently of Pod health and desired revisions.
type InstanceStorageIdentity struct {
	// Complete means every referenced claim UID, binding and location, and any ephemeral Pod identity, is known.
	// False does not imply empty storage or missing data.
	Complete bool `json:"complete"`
	// Volumes contains actual mounted claims while a Pod exists, and declared or retained claims while absent.
	// When Complete is false, a previously observed owned claim may remain for verifying cleanup; its current presence is unknown.
	// Generic ephemeral claims, unsupported persistent storage and unknown locations leave Complete false;
	// specification and runtime events refresh them. Generic ephemeral claim names are not inferred from Pod names.
	// +optional
	Volumes []InstanceVolumeIdentity `json:"volumes,omitempty"`
	// EphemeralPod binds nonpersistent data to the actual Pod UID. Pod replacement invalidates that data.
	// It remains diagnostic for an unresolved generic ephemeral claim, whose Complete is false.
	// +optional
	EphemeralPod *InstanceObjectReference `json:"ephemeralPod,omitempty"`
}

// InstanceMemberIdentity identifies an actual database member rather than a desired role or Pod name.
type InstanceMemberIdentity struct {
	Group  string `json:"group"`
	Member string `json:"member"`
}

// InstanceMembershipState describes the observation of an actual member binding.
// +kubebuilder:validation:Enum=Unknown;Present;Absent
type InstanceMembershipState string

const (
	InstanceMembershipUnknown InstanceMembershipState = "Unknown"
	InstanceMembershipPresent InstanceMembershipState = "Present"
	InstanceMembershipAbsent  InstanceMembershipState = "Absent"
)

// InstanceMembershipStatus is nil until membership is observed. Pod absence never proves member absence.
type InstanceMembershipStatus struct {
	State InstanceMembershipState `json:"state"`
	// Identity is the last observed member. Unknown preserves an unresolved binding until its absence is confirmed.
	// +optional
	Identity *InstanceMemberIdentity `json:"identity,omitempty"`
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Reason string `json:"reason,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}

// InstanceDataIdentity identifies the storage and optional engine dataset marker covered by a data observation.
type InstanceDataIdentity struct {
	Storage InstanceStorageIdentity `json:"storage"`
	// Dataset is an actual engine dataset marker, when supplied by its observer.
	// +optional
	Dataset string `json:"dataset,omitempty"`
}

// InstanceDataState describes an actual data observation. Storage binding and runtime health do not establish initialization.
// +kubebuilder:validation:Enum=Unknown;Empty;Partial;Initializing;Initialized
type InstanceDataState string

const (
	InstanceDataUnknown      InstanceDataState = "Unknown"
	InstanceDataEmpty        InstanceDataState = "Empty"
	InstanceDataPartial      InstanceDataState = "Partial"
	InstanceDataInitializing InstanceDataState = "Initializing"
	InstanceDataInitialized  InstanceDataState = "Initialized"
)

// InstanceDataStatus is nil until data is observed. Unknown may retain an old identity for diagnostics, not as usable success.
type InstanceDataStatus struct {
	State InstanceDataState `json:"state"`
	// +optional
	Identity *InstanceDataIdentity `json:"identity,omitempty"`
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Reason string `json:"reason,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}

// InstanceExecutionState describes an observed action execution, not a request or replay token.
// +kubebuilder:validation:Enum=Unknown;Running;Succeeded;Failed
type InstanceExecutionState string

const (
	InstanceExecutionUnknown   InstanceExecutionState = "Unknown"
	InstanceExecutionRunning   InstanceExecutionState = "Running"
	InstanceExecutionSucceeded InstanceExecutionState = "Succeeded"
	InstanceExecutionFailed    InstanceExecutionState = "Failed"
)

// InstanceExecutionObservation is nil until an executor is observed. This contract does not create or recover actions.
type InstanceExecutionObservation struct {
	State InstanceExecutionState `json:"state"`
	// Action identifies the observed action.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Action   string                  `json:"action"`
	Executor InstanceObjectReference `json:"executor"`
	// Member and Data identify actual targets when provided by the action observer.
	// +optional
	Member *InstanceMemberIdentity `json:"member,omitempty"`
	// +optional
	Data *InstanceDataIdentity `json:"data,omitempty"`
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=128
	Reason string `json:"reason,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}
