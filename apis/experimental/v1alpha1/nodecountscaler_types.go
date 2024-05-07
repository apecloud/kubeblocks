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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeCountScalerSpec defines the desired state of NodeCountScaler
type NodeCountScalerSpec struct {
	// Specified the target Cluster name this scaler applies to.
	TargetClusterName string `json:"targetClusterName"`

	// Specified the target Component names this scaler applies to.
	// All Components will be applied if not set.
	//
	// +optional
	TargetComponentNames []string `json:"targetComponentNames,omitempty"`
}

// NodeCountScalerStatus defines the observed state of NodeCountScaler
type NodeCountScalerStatus struct {
	// Records the current status information of all Components specified in the NodeCountScalerSpec.
	//
	// +optional
	ComponentStatuses []ComponentStatus `json:"componentStatuses,omitempty"`

	// Represents the latest available observations of a nodecountscaler's current state.
	// Known .status.conditions.type are: "ScaleReady".
	// ScaleReady - All target components are ready.
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// LastScaleTime is the last time the NodeCountScaler scaled the number of instances.
	//
	// +optional
	LastScaleTime metav1.Time `json:"lastScaleTime,omitempty"`
}

type ComponentStatus struct {
	// Specified the Component name.
	Name string `json:"name"`

	// The current number of instances of this component.
	CurrentReplicas int32 `json:"currentReplicas"`

	// The number of instances of this component with a Ready condition.
	ReadyReplicas int32 `json:"readyReplicas"`

	// The number of instances of this component with a Ready condition for at least MinReadySeconds defined in the instance template.
	AvailableReplicas int32 `json:"availableReplicas"`

	// The desired number of instances of this component.
	// Usually, it should be the number of nodes.
	DesiredReplicas int32 `json:"desiredReplicas"`
}

type ConditionType string

const (
	// ScaleReady is added to a nodecountscaler when all target components are ready.
	ScaleReady ConditionType = "ScaleReady"
)

const (
	// ReasonNotReady is a reason for condition ScaleReady.
	ReasonNotReady = "NotReady"

	// ReasonReady is a reason for condition ScaleReady.
	ReasonReady = "Ready"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks,all},shortName=ncs
// +kubebuilder:printcolumn:name="TARGET-CLUSTER-NAME",type="string",JSONPath=".spec.targetClusterName",description="target cluster name."
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type==\"ScaleReady\")].status",description="scale ready."
// +kubebuilder:printcolumn:name="REASON",type="string",JSONPath=".status.conditions[?(@.type==\"ScaleReady\")].reason",description="reason."
// +kubebuilder:printcolumn:name="MESSAGE",type="string",JSONPath=".status.conditions[?(@.type==\"ScaleReady\")].message",description="message."
// +kubebuilder:printcolumn:name="LAST-SCALE-TIME",type="date",JSONPath=".status.lastScaleTime"

// NodeCountScaler is the Schema for the nodecountscalers API
type NodeCountScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeCountScalerSpec   `json:"spec,omitempty"`
	Status NodeCountScalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeCountScalerList contains a list of NodeCountScaler
type NodeCountScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeCountScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeCountScaler{}, &NodeCountScalerList{})
}
