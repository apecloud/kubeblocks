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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// Todo: controller, add label to resource, update status.

// ChannelDefinitionSpec defines the desired state of ChannelDefinition
type ChannelDefinitionSpec struct {
	// +optional
	Type ChannelDefWorkerType `json:"type,omitempty"`

	// +optional
	KubeBlocksSettings KubeBlocksSettings `json:"kubeblocksSetting,omitempty"`

	// +optional
	Source ChannelDefinitionWorker `json:"source"`

	// +optional
	Sink ChannelDefinitionWorker `json:"sink"`

	// +optional
	TopologyStructs []TopologyStruct `json:"topologyStructs,omitempty"`

	// +optional
	IsDefault bool `json:"isDefault,omitempty"`
}

// ChannelDefinitionStatus defines the observed state of ChannelDefinition
type ChannelDefinitionStatus struct {
}

type ChannelDefinitionWorker struct {
	// +optional
	KubeBlocks KubeBlocksWorker `json:"kubeblocks,omitempty"`

	// +optional
	SyncObjEnvExpress []SyncObjEnvExpress `json:"syncObjEnvExpress,omitempty"`
}

type KubeBlocksSettings struct {
	ClusterDefinitionRef string `json:"clusterDefinitionRef,omitempty"`

	// +optional
	Expose ExposeService `json:"expose,omitempty"`
}

type KubeBlocksWorker struct {
	// +optional
	ConfigureRequests *appv1alpha1.Reconfigure `json:"configureRequests,omitempty"`

	// +optional
	OpsRequest []*appv1alpha1.OpsRequestSpec `json:"opsRequest,omitempty"`

	// +optional
	AccountRequests ChannelDefAccount `json:"accountRequests,omitempty"`

	Worker *appv1alpha1.ClusterSpec `json:"cluster"`

	// +optional
	ExtraEnvs map[string]string `json:"extraEnvs,omitempty"`
}

type SyncObjEnvExpress struct {
	Name string `json:"name,omitempty"`

	MetaTypeRequired []SyncMetaType `json:"metaTypeRequired,omitempty"`

	MetaTypeConnectSymbol string `json:"metaTypeConnectSymbol,omitempty"`

	MetaObjConnectSymbol string `json:"metaObjConnectSymbol,omitempty"`

	// +optional
	SelectMode SelectMode `json:"selectMode,omitempty"`

	// +optional
	Prefix string `json:"prefix,omitempty"`

	// +optional
	Suffix string `json:"suffix,omitempty"`
}

// IsEmpty check if the SyncObjEnvExpress is empty.
func (se *SyncObjEnvExpress) IsEmpty() bool {
	return se.Name == "" || len(se.MetaTypeRequired) == 0
}

type ChannelDefAccount struct {
	ComponentName string `json:"componentName,omitempty"`

	AccountName string `json:"accountName,omitempty"`
}

type ExposeService struct {
	ComponentDefRef string `json:"componentDefRef,omitempty"`

	Service corev1.ServicePort `json:"service,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},shortName=chd
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ChannelDefinition is the Schema for the channelDefinition API
type ChannelDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChannelDefinitionSpec   `json:"spec,omitempty"`
	Status ChannelDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChannelDefinitionList contains a list of ChannelDefinition
type ChannelDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChannelDefinition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChannelDefinition{}, &ChannelDefinitionList{})
}
