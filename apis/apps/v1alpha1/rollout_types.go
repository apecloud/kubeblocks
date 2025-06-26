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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks}
// +kubebuilder:printcolumn:name="CLUSTER",type="string",JSONPath=".spec.clusterName",description="The target cluster to be rolled out."
// +kubebuilder:printcolumn:name="REPLICAS",type="string",JSONPath=".status.components[0].replicas",description="The replicas before rollout."
// +kubebuilder:printcolumn:name="ROLLED-OUT",type="string",JSONPath=".status.components[0].rolledOutReplicas",description="The rolled out replicas."
// +kubebuilder:printcolumn:name="CANARY",type="string",JSONPath=".status.components[0].canaryReplicas",description="The canary replicas."
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state",description="The rollout state."
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

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

	// TODO: auto-reclaim the successful rollouts.
}

// RolloutStatus defines the observed state of Rollout
type RolloutStatus struct {
	// The most recent generation number of the Rollout object that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The current state of the Rollout.
	//
	// +optional
	State RolloutState `json:"state,omitempty"`

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
	ServiceVersion *string `json:"serviceVersion,omitempty"`

	// Specifies the target ComponentDefinition of the component.
	//
	// +kubebuilder:validation:MaxLength=64
	// +optional
	CompDef *string `json:"compDef,omitempty"`

	// Specifies the rollout strategy for the component.
	//
	// +kubebuilder:validation:Required
	Strategy RolloutStrategy `json:"strategy"`

	// Specifies the number of instances to be rolled out.
	//
	// +optional
	Replicas *intstr.IntOrString `json:"replicas,omitempty"`

	// Additional meta for the instances.
	//
	// +optional
	InstanceMeta *RolloutInstanceMeta `json:"instanceMeta,omitempty"`
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
	// If specified, the rollout will be performed by replacing the old instances with new instances one by one (create and then delete).
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

type RolloutStrategyInplace struct{}

type RolloutStrategyReplace struct {
	// Specifies the scheduling policy for the new instance.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// The number of seconds to wait between rolling out two instances.
	//
	// +optional
	PerInstanceIntervalSeconds *int32 `json:"perInstanceIntervalSeconds,omitempty"`

	// The number of seconds to wait before scaling down an old instance, after the new instance becomes ready.
	//
	// +optional
	ScaleDownDelaySeconds *int32 `json:"scaleDownDelaySeconds,omitempty"`

	// TODO: policy to scale-down the old instances and retain the PVCs.
}

type RolloutStrategyCreate struct {
	// Whether to decorate the new instances as canary instances.
	//
	// +optional
	Canary *bool `json:"canary,omitempty"`

	// Specifies the scheduling policy for the new instance.
	//
	// +optional
	SchedulingPolicy *SchedulingPolicy `json:"schedulingPolicy,omitempty"`

	// Specifies the promotion strategy for the component.
	//
	// +optional
	Promotion *RolloutPromotion `json:"promotion,omitempty"`
}

type RolloutPromotion struct {
	// Specifies whether to automatically promote the new instances.
	//
	// +optional
	Auto *bool `json:"auto,omitempty"`

	// The delay seconds before promoting the new instances.
	//
	// +kubebuilder:default=30
	// +optional
	DelaySeconds *int32 `json:"delaySeconds,omitempty"`

	// The condition for promoting the new instances.
	//
	// +optional
	Condition *RolloutPromoteCondition `json:"condition,omitempty"`

	// The delay seconds before scaling down the old instances.
	//
	// +kubebuilder:default=30
	// +optional
	ScaleDownDelaySeconds *int32 `json:"scaleDownDelaySeconds,omitempty"`

	// TODO: policy to retain the PVCs of old instances.
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

	// TODO: variables can be used in the conditions.
}

type RolloutInstanceMeta struct {
	//// Meta added to the old instances.
	////
	//// +optional
	// Stable *InstanceMeta `json:"stable,omitempty"`

	// Meta added to the new instances.
	//
	// +optional
	Canary *InstanceMeta `json:"canary,omitempty"`
}

type InstanceMeta struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// RolloutState defines the state of the Rollout within the .status.state field.
//
// +enum
// +kubebuilder:validation:Enum={Pending,Rolling,Succeed,Error}
type RolloutState string

const (
	PendingRolloutState RolloutState = "Pending"
	RollingRolloutState RolloutState = "Rolling"
	SucceedRolloutState RolloutState = "Succeed"
	ErrorRolloutState   RolloutState = "Error"
)

type RolloutComponentStatus struct {
	// The name of the component.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The ServiceVersion of the component before the rollout.
	//
	// +kubebuilder:validation:Required
	ServiceVersion string `json:"serviceVersion"`

	// The ComponentDefinition of the component before the rollout.
	//
	// +kubebuilder:validation:Required
	CompDef string `json:"compDef"`

	// The replicas the component has before the rollout.
	//
	// +kubebuilder:validation:Required
	Replicas int32 `json:"replicas"`

	// The new replicas the component has been created successfully.
	//
	// +optional
	NewReplicas int32 `json:"newReplicas"`

	// The replicas the component has been rolled out successfully.
	//
	// +optional
	RolledOutReplicas int32 `json:"rolledOutReplicas"`

	// The number of canary replicas the component has.
	//
	// +optional
	CanaryReplicas int32 `json:"canaryReplicas"`

	// The instances that are scaled down.
	//
	// +optional
	ScaleDownInstances []string `json:"scaleDownInstances,omitempty"`

	// The last time a component replica was scaled up successfully.
	//
	// +optional
	LastScaleUpTimestamp metav1.Time `json:"lastScaleUpTimestamp,omitempty"`

	// The last time a component replica was scaled down successfully.
	//
	// +optional
	LastScaleDownTimestamp metav1.Time `json:"lastScaleDownTimestamp,omitempty"`
}
