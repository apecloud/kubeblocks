/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks}

// Rollout is the Schema for the rollouts API
type Rollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RolloutSpec   `json:"spec,omitempty"`
	Status RolloutStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RolloutList contains a list of Rollout
type RolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rollout{}, &RolloutList{})
}

// RolloutSpec defines the desired state of Rollout
type RolloutSpec struct {
	// Specifies the target cluster of the Rollout.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	ClusterName string `json:"clusterName"`

	// Specifies the target components to be rolled out.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Components []RolloutComponent `json:"components,omitempty"`
}

// RolloutStatus defines the observed state of Rollout
type RolloutStatus struct {
	// The most recent generation number of the Rollout object that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The current phase of the Rollout.
	//
	// +optional
	Phase RolloutPhase `json:"phase,omitempty"`

	// Provides additional information about the phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// Represents a list of detailed status of the Rollout object.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Records the status information of all components within the Rollout.
	//
	// +optional
	Components []RolloutComponentStatus `json:"components,omitempty"`
}

type RolloutComponent struct {
	// Specifies the name of the component.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the target ServiceVersion of the component.
	//
	// +kubebuilder:validation:MaxLength=32
	// +optional
	ServiceVersion string `json:"serviceVersion,omitempty"`

	// Specifies the target ComponentDefinition of the component.
	//
	// +kubebuilder:validation:MaxLength=64
	// +optional
	CompDef string `json:"compDef"`

	// Specifies the rollout strategy for the component.
	//
	// +kubebuilder:validation:Required
	Strategy RolloutStrategy `json:"strategy"`

	// Specifies the number of instances to be rolled out.
	//
	// +optional
	Replicas *intstr.IntOrString `json:"replicas,omitempty"`

	// Additional metadata for the instances.
	//
	// +optional
	Metadata *RolloutMetadata `json:"metadata,omitempty"`

	// Specifies the promotion strategy for the component.
	//
	// +optional
	Promotion *RolloutPromotion `json:"promotion,omitempty"`
}

type RolloutStrategy struct {
	// In-place rollout strategy.
	//
	// If specified, the rollout will be performed in-place (delete and then create).
	//
	// +optional
	Inplace *RolloutStrategyInplace `json:"inplace,omitempty"`

	// Replace rollout strategy.
	//
	// If specified, the rollout will be performed by replacing the old instances with new instances (create and then delete).
	//
	// +optional
	Replace *RolloutStrategyReplace `json:"replace,omitempty"`

	// Create rollout strategy.
	//
	// If specified, the rollout will be performed by creating new instances.
	//
	// +optional
	Create *RolloutStrategyCreate `json:"create,omitempty"`
}

type RolloutStrategyInplace struct {
	// The selector to select the instances to be rolled out in-place.
	//
	// +optional
	Selector *RolloutPodSelector `json:"selector,omitempty"`
}

type RolloutStrategyReplace struct {
	// The selector to select the instances to be rolled out by replacing.
	//
	// +optional
	Selector *RolloutPodSelector `json:"selector,omitempty"`

	// Specifies the affinity for the new instances.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

type RolloutStrategyCreate struct {
	// Specifies the affinity for the new instances.
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

type RolloutPodSelector struct {
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// TODO: role, ordinal, or other selectors
}

type RolloutPromotion struct {
	// Specifies whether to automatically promote the new instances.
	//
	// +optional
	Auto *bool `json:"auto,omitempty"`

	// The delay time (in seconds) before promoting the new instances.
	//
	// +kubebuilder:default=30
	// +optional
	DelaySeconds *int32 `json:"delaySeconds,omitempty"`

	// The condition for promoting the new instances.
	//
	// +optional
	Condition *RolloutPromoteCondition `json:"condition,omitempty"`

	// The delay time (in seconds) before scaling down the old instances.
	//
	// +kubebuilder:default=30
	// +optional
	ScaleDownDelaySeconds *int32 `json:"scaleDownDelaySeconds,omitempty"`
}

type RolloutPromoteCondition struct {
	// The condition before promoting the new instances.
	//
	// If specified, the new instances will be promoted only when the condition is met.
	//
	// +optional
	Prev *appsv1.Action `json:"prev,omitempty"`

	// The condition after promoting the new instances successfully.
	//
	// +optional
	Post *appsv1.Action `json:"post,omitempty"`

	// TODO: variables to be used in the conditions.
}

type RolloutMetadata struct {
	// Metadata added to the old instances.
	//
	// +optional
	Stable *Metadata `json:"stable,omitempty"`

	// Metadata added to the new instances.
	//
	// +optional
	Canary *Metadata `json:"canary,omitempty"`
}

type Metadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// RolloutPhase defines the phase of the Rollout within the .status.phase field.
//
// +enum
// +kubebuilder:validation:Enum={Pending,Running,Succeed,Failed}
type RolloutPhase string

const (
	PendingRolloutPhase RolloutPhase = "Pending"
	RunningRolloutPhase RolloutPhase = "Running"
	SucceedRolloutPhase RolloutPhase = "Succeed"
	FailedRolloutPhase  RolloutPhase = "Failed"
)

type RolloutComponentStatus struct {
	// Specifies the name of the component.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Specifies the number of replicas has been rolled out.
	//
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`
}
