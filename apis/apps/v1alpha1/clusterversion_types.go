/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	// It overrides `image` and `env` attributes defined in ClusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.
	// To clean default envs settings, set `SystemAccountSpec.CmdExecutorConfig.Env` to empty list.
	// +optional
	SystemAccountSpec *SystemAccountShortSpec `json:"systemAccountSpec,omitempty"`

	// versionContext defines containers images' context for component versions,
	// this value replaces ClusterDefinition.spec.componentDefs.podSpec.[initContainers | containers]
	VersionsCtx VersionsContext `json:"versionsContext"`

	// switchoverSpec defines images for the component to do switchover.
	// It overrides `image` and `env` attributes defined in ClusterDefinition.spec.componentDefs.SwitchoverSpec.CommandExecutorEnvItem.
	// +optional
	SwitchoverSpec *SwitchoverShortSpec `json:"switchoverSpec,omitempty"`
}

// SystemAccountShortSpec is a short version of SystemAccountSpec, with only CmdExecutorConfig field.
type SystemAccountShortSpec struct {
	// cmdExecutorConfig configs how to get client SDK and perform statements.
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CommandExecutorEnvItem `json:"cmdExecutorConfig"`
}

// SwitchoverShortSpec is a short version of SwitchoverSpec, with only CommandExecutorEnvItem field.
type SwitchoverShortSpec struct {
	CommandExecutorEnvItem `json:",inline"`
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

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
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
