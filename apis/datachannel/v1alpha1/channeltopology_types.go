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
	appv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelTopologySpec defines the desired state of Channel
type ChannelTopologySpec struct {
	// Channel description.
	// +optional
	Description string `json:"description,omitempty"`

	Channels []ChannelDefine `json:"channels"`

	// +optional
	Hubs []HubDefine `json:"hubs,omitempty"`

	// +optional
	Settings GlobalSettings `json:"settings,omitempty"`
}

// ChannelTopologyStatus defines the observed state of ChannelTopology
type ChannelTopologyStatus struct {
	Phase ChannelTopologyPhase `json:"phase,omitempty"`

	Message string `json:"message,omitempty"`

	// +optional
	ChannelTotal int `json:"channelTotalCount,omitempty"`

	// +optional
	ChannelEstablished int `json:"channelEstablished,omitempty"`

	// +optional
	ChannelWaitForBuilding int `json:"channelWaitForBuilding,omitempty"`
}

type ChannelDefine struct {
	Name string `json:"name,omitempty"`

	From ChannelNodeDefine `json:"from,omitempty"`

	To ChannelNodeDefine `json:"to,omitempty"`

	// +optional
	IncludeObjs []SyncMetaObject `json:"includeObjs,omitempty"`

	// +optional
	ExcludeObjs []SyncMetaObject `json:"excludeObjs,omitempty"`
}

type ChannelNodeDefine struct {
	// +optional
	ClusterRef string `json:"clusterRef,omitempty"`

	// +optional
	ClusterNamespace string `json:"clusterNamespace,omitempty"`

	// +optional
	ChannelDefinitionRef string `json:"channelDefinitionRef,omitempty"`

	// +optional
	HubRef string `json:"hubRef,omitempty"`
}

type HubDefine struct {
	Name string `json:"name,omitempty"`

	Namespace string `json:"namespace,omitempty"`

	ClusterRef string `json:"clusterRef,omitempty"`

	LimitPolicy LimitPolicy `json:"limitPolicy,omitempty"`
}

type LimitPolicy struct {

	// +optional
	BestEffect bool `json:"bestEffect,omitempty"`

	// +optional
	NumLimit int `json:"numLimit,omitempty"`
}

type RelyClusterRef struct {
	Name string `json:"name,omitempty"`

	RelyHub string `json:"clusterRef,omitempty"`

	// +optional
	Namespace string `json:"namespace,omitempty"`

	// +optional
	RelyHubChannelDef string `json:"clusterChannelDef,omitempty"`
}

type GlobalSettings struct {
	// +optional
	Topology TopologySettings `json:"topology,omitempty"`

	// +optional
	Schedule ScheduleSettings `json:"schedule,omitempty"`
}

type TopologySettings struct {
	PrepareTTLMinutes int `json:"prepareTtlMinutes,omitempty"`

	TopologyStruct TopologyStruct `json:"topologyStruct,omitempty"`

	BuildingPolicy BuildingPolicy `json:"buildingPolicy,omitempty"`
}

type ScheduleSettings struct {
	// affinity is a group of affinity scheduling rules.
	// +optional
	Affinity *appv1alpha1.Affinity `json:"affinity,omitempty"`

	// tolerations are attached to tolerate any taint that matches the triple <key,value,effect> using the matching operator <operator>.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

type SyncMetaObject struct {
	Name string `json:"name"`

	// +optional
	MappingName string `json:"mappingName,omitempty"`

	Type SyncMetaType `json:"type,omitempty"`

	// +optional
	IsAll bool `json:"isAll,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Child []SyncMetaObject `json:"child,omitempty"`
}

// IsEmpty check if the SyncMetaObject is empty.
func (so *SyncMetaObject) IsEmpty() bool {
	return so.Name == "" || (!so.IsAll) && len(so.Child) == 0
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},shortName=cht
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status"
// +kubebuilder:printcolumn:name="CHANNEL-TOTAL",type="string",JSONPath=".status.channelTotalCount",description="channelTotalCount"
// +kubebuilder:printcolumn:name="CHANNEL-ESTABLISHED",type="string",JSONPath=".status.channelEstablished",description="channelEstablished"
// +kubebuilder:printcolumn:name="CHANNEL-WAIT-FOR-BUILDING",type="string",JSONPath=".status.channelWaitForBuilding",description="channelWaitForBuilding"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ChannelTopology is the Schema for the channelTopology API
type ChannelTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChannelTopologySpec   `json:"spec,omitempty"`
	Status ChannelTopologyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChannelTopologyList contains a list of ChannelTopology
type ChannelTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChannelTopology `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChannelTopology{}, &ChannelTopologyList{})
}
