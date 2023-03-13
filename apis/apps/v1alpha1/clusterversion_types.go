/*
Copyright ApeCloud, Inc.

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
	// +kubebuilder:validation:Enum={Available,Unavailable}
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

// ClusterComponentVersion is an application version component spec.
type ClusterComponentVersion struct {
	// componentDefRef reference one of the cluster component definition names in ClusterDefinition API (spec.componentDefs.name).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// ConfigTemplateRefs defines a configuration extension mechanism to handle configuration differences between versions,
	// the configTemplateRefs field, together with configTemplateRefs in the ClusterDefinition,
	// determines the final configuration file.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// versionContext defines containers images' context for component versions,
	// this value replaces ClusterDefinition.spec.componentDefs.podSpec.[initContainers | containers]
	VersionsCtx VersionsContext `json:"versionsContext"`
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

// Ready checks whether the cluster version object is ready for use.
func (r *ClusterVersion) Ready() bool {
	return r.Status.Phase == AvailablePhase
}

// GetDefNameMappingComponents returns ComponentDefRef name mapping ClusterComponentVersion.
func (r *ClusterVersion) GetDefNameMappingComponents() map[string]*ClusterComponentVersion {
	m := map[string]*ClusterComponentVersion{}
	for i, c := range r.Spec.ComponentVersions {
		m[c.ComponentDefRef] = &r.Spec.ComponentVersions[i]
	}
	return m
}
