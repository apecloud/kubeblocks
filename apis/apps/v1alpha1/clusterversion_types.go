/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

// ClusterVersionSpec defines the desired state of ClusterVersion.
//
// Deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type ClusterVersionSpec struct {
	// Specifies a reference to the ClusterDefinition.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ClusterDefinitionRef string `json:"clusterDefinitionRef"`

	// Contains a list of versioning contexts for the components' containers.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=componentDefRef
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=componentDefRef
	ComponentVersions []ClusterComponentVersion `json:"componentVersions" patchStrategy:"merge,retainKeys" patchMergeKey:"componentDefRef"`
}

// ClusterVersionStatus defines the observed state of ClusterVersion.
//
// Deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type ClusterVersionStatus struct {
	// The current phase of the ClusterVersion.
	//
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Provides additional information about the current phase.
	//
	// +optional
	Message string `json:"message,omitempty"`

	// The generation number that has been observed by the controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The generation number of the ClusterDefinition that is currently being referenced.
	//
	// +optional
	ClusterDefGeneration int64 `json:"clusterDefGeneration,omitempty"`
}

func (r ClusterVersionStatus) GetTerminalPhases() []Phase {
	return []Phase{AvailablePhase}
}

// ClusterComponentVersion is an application version component spec.
//
// Deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type ClusterComponentVersion struct {
	// Specifies a reference to one of the cluster component definition names in the ClusterDefinition API (spec.componentDefs.name).
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	ComponentDefRef string `json:"componentDefRef"`

	// Defines a configuration extension mechanism to handle configuration differences between versions.
	// The configTemplateRefs field, in conjunction with the configTemplateRefs in the ClusterDefinition, determines
	// the final configuration file.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ConfigSpecs []ComponentConfigSpec `json:"configSpecs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// Defines the image for the component to connect to databases or engines.
	// This overrides the `image` and `env` attributes defined in clusterDefinition.spec.componentDefs.systemAccountSpec.cmdExecutorConfig.
	// To clear default environment settings, set systemAccountSpec.cmdExecutorConfig.env to an empty list.
	//
	// +optional
	SystemAccountSpec *SystemAccountShortSpec `json:"systemAccountSpec,omitempty"`

	// Defines the context for container images for component versions.
	// This value replaces the values in clusterDefinition.spec.componentDefs.podSpec.[initContainers | containers].
	VersionsCtx VersionsContext `json:"versionsContext"`

	// Defines the images for the component to perform a switchover.
	// This overrides the image and env attributes defined in clusterDefinition.spec.componentDefs.SwitchoverSpec.CommandExecutorEnvItem.
	//
	// +optional
	SwitchoverSpec *SwitchoverShortSpec `json:"switchoverSpec,omitempty"`
}

// SystemAccountShortSpec represents a condensed version of the SystemAccountSpec.
//
// Deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type SystemAccountShortSpec struct {
	// Configures the method for obtaining the client SDK and executing statements.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CommandExecutorEnvItem `json:"cmdExecutorConfig"`
}

// SwitchoverShortSpec represents a condensed version of the SwitchoverSpec.
//
// Deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type SwitchoverShortSpec struct {
	// Represents the configuration for the command executor.
	//
	// +kubebuilder:validation:Required
	CmdExecutorConfig *CommandExecutorEnvItem `json:"cmdExecutorConfig"`
}

// VersionsContext is deprecated since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type VersionsContext struct {
	// Provides override values for ClusterDefinition.spec.componentDefs.podSpec.initContainers.
	// Typically used in scenarios such as updating application container images.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Provides override values for ClusterDefinition.spec.componentDefs.podSpec.containers.
	// Typically used in scenarios such as updating application container images.
	//
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
// +kubebuilder:deprecatedversion:warning="The ClusterVersion CRD has been deprecated since 0.9.0"

// ClusterVersion is the Schema for the ClusterVersions API.
//
// Deprecated: ClusterVersion has been replaced by ComponentVersion since v0.9.
// This struct is maintained for backward compatibility and its use is discouraged.
type ClusterVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterVersionSpec   `json:"spec,omitempty"`
	Status ClusterVersionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterVersionList contains a list of ClusterVersion.
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
