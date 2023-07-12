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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterFamilySpec defines the desired state of ClusterFamily
type ClusterFamilySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// list of clustertemplates
	ClusterTemplateRefs []ClusterFamilyTemplateRef `json:"clusterTemplateRefs,omitempty"`
}

// ClusterFamilyStatus defines the observed state of ClusterFamily
type ClusterFamilyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cf
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterFamily is the Schema for the clusterfamilies API
type ClusterFamily struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec of clusterfamily which defines what kind of clustertemplate to use in some conditions
	Spec ClusterFamilySpec `json:"spec,omitempty"`
	// status of clusterfamily
	Status ClusterFamilyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterFamilyList contains a list of ClusterFamily
type ClusterFamilyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFamily `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFamily{}, &ClusterFamilyList{})
}

type ClusterFamilyTemplateRef struct {
	Key string `json:"key,omitempty"`

	Expression string `json:"expression,omitempty"`

	Value string `json:"value,omitempty"`

	TemplateRef string `json:"templateRef,omitempty"`

	Selector []ClusterFamilyTemplateRefSelector `json:"selector,omitempty"`
}

type ClusterFamilyTemplateRefSelector struct {
	Value string `json:"value,omitempty"`

	Expression string `json:"expression,omitempty"`

	TemplateRef string `json:"template,omitempty"`
}
