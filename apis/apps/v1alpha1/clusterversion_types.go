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
)

// ClusterVersionSpec defines the desired state of ClusterVersion
type ClusterVersionSpec struct {
	// ref ClusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefinitionRef string `json:"clusterDefinitionRef"`

	// List of components' containers versioning context, i.e., container image ID, container commands, args., and environments.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=componentDefRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefRef
	ComponentVersions []ClusterComponentVersion `json:"componentVersions" patchStrategy:"merge,retainKeys" patchMergeKey:"componentDefRef"`
}

// ClusterVersionStatus defines the observed state of ClusterVersion
type ClusterVersionStatus struct {
	// phase - in list of [Available,Unavailable]
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human readable message indicating details about why the ClusterVersion is in this phase.
	// +optional
	Message string `json:"message,omitempty"`

	// generation number
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// clusterDefGeneration represents the generation number of ClusterDefinition referenced.
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`
}

func (r ClusterVersionStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

// ClusterComponentVersion is an application version component spec.
type ClusterComponentVersion struct {
	// componentDefRef reference one of the cluster component definition names in ClusterDefinition API (spec.componentDefs.name).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// configSpecs defines a configuration extension mechanism to handle configuration differences between versions,
	// the configTemplateRefs field, together with configTemplateRefs in the ClusterDefinition,
	// determines the final configuration file.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// systemAccountSpec define image for the component to connect database or engines.
	// It has a higher priority over ClusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.image.
	// +optional
	SystemAccountSpec *SystemAccountShortSpec `json:"systemAccountSpec,omitempty"`

	// versionContext defines containers images' context for component versions,
	// this value replaces ClusterDefinition.spec.componentDefs.podSpec.[initContainers | containers]
	VersionsCtx VersionsContext `json:"versionsContext"`
}

// SystemAccountShortSpec is a short version of SystemAccountSpec, with only CmdExecutorConfig field.
type SystemAccountShortSpec struct {
	// cmdExecutorConfig configs how to get client SDK and perform statements.
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CommandExecutorEnvItem `json:"cmdExecutorConfig"`
}

type VersionsContext struct {
	// Provide ClusterDefinition.spec.componentDefs.podSpec.initContainers override
	// values, typical scenarios are application container image updates.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Provide ClusterDefinition.spec.componentDefs.podSpec.containers override
	// values, typical scenarios are application container image updates.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	Containers []corev1.Container `json:"containers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster,shortName=cv
// +kubebuilder:printcolumn:name="CLUSTER-DEFINITION",type="string",JSONPath=".spec.clusterDefinitionRef",description="ClusterDefinition referenced by cluster."
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterVersion is the Schema for the ClusterVersions API
type ClusterVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterVersionSpec   `json:"spec,omitempty"`
	Status ClusterVersionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterVersionList contains a list of ClusterVersion
type ClusterVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterVersion{}, &ClusterVersionList{})
}

// GetDefNameMappingComponents returns ComponentDefRef name mapping ClusterComponentVersion.
func (r ClusterVersionSpec) GetDefNameMappingComponents() map[string]*ClusterComponentVersion {
	m := map[string]*ClusterComponentVersion{}
	for i, c := range r.ComponentVersions {
		m[c.ComponentDefRef] = &r.ComponentVersions[i]
	}
	return m
}
