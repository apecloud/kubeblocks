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
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks}

// Rollout is the Schema for the rollouts API
type Rollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RolloutSpec   `json:"spec,omitempty"`
	Status RolloutStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

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
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=64
	ClusterName string `json:"clusterName"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Components []RolloutComponent `json:"components,omitempty"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=128
	// +optional
	Shardings []RolloutSharding `json:"shardings,omitempty"`
}

// RolloutStatus defines the observed state of Rollout
type RolloutStatus struct {
}

type RolloutComponent struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=22
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
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

	// +kubebuilder:validation:Required
	Strategy RolloutStrategy `json:"strategy"`

	// +optional
	Replicas *intstr.IntOrString `json:"replicas,omitempty"`

	// +optional
	Promotion *RolloutPromotion `json:"promotion,omitempty"`

	// +optional
	Metadata *RolloutMetadata `json:"metadata,omitempty"`

	// +optional
	Services []*corev1.Service `json:"services,omitempty"`
}

type RolloutSharding struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string `json:"name"`
}

type RolloutStrategy struct {
	// +optional
	Inplace *RolloutStrategyInplace `json:"inplace,omitempty"`

	// +optional
	Replace *RolloutStrategyReplace `json:"replace,omitempty"`

	// +optional
	Create *RolloutStrategyCreate `json:"create,omitempty"`
}

type RolloutStrategyInplace struct {
	// +optional
	Selector *RolloutPodSelector `json:"selector,omitempty"`
}

type RolloutStrategyReplace struct {
	// +optional
	Selector *RolloutPodSelector `json:"selector,omitempty"`
}

type RolloutStrategyCreate struct {
	// +optional
	AntiAffinity *corev1.PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

type RolloutPodSelector struct {
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// TODO: role, ordinal, or other selectors
}

type RolloutPromotion struct {
	// +optional
	Auto *bool `json:"auto,omitempty"`

	// +kubebuilder:default=30
	// +optional
	DelaySeconds *int32 `json:"delaySeconds,omitempty"`

	// +optional
	Condition *RolloutPromotionCondition `json:"condition,omitempty"`

	// +kubebuilder:default=30
	// +optional
	ScaleDownDelaySeconds *int32 `json:"scaleDownDelaySeconds,omitempty"`
}

type RolloutPromotionCondition struct {
	// +optional
	Prev *appsv1.Action `json:"prev,omitempty"`

	// +optional
	Post *appsv1.Action `json:"post,omitempty"`
}

type RolloutMetadata struct {
	// +optional
	Stable *Metadata `json:"stable,omitempty"`

	// +optional
	Preview *Metadata `json:"preview,omitempty"`
}

type Metadata struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}
