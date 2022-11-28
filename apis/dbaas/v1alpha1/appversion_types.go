/*
Copyright ApeCloud Inc.

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

// AppVersionSpec defines the desired state of AppVersion
type AppVersionSpec struct {
	// ref ClusterDefinition.
	// +kubebuilder:validation:Required
	ClusterDefinitionRef string `json:"clusterDefinitionRef"`

	// List of components in current AppVersion. Component will replace the field in ClusterDefinition's component if type is matching typeName.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +patchMergeKey=type
	// +patchStrategy=merge,retainKeys
	Components []AppVersionComponent `json:"components" patchStrategy:"merge,retainKeys" patchMergeKey:"type"`
}

// AppVersionStatus defines the observed state of AppVersion
type AppVersionStatus struct {
	// phase - in list of [Available,Unavailable]
	// +kubebuilder:validation:Enum={Available,Unavailable}
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// A human readable message indicating details about why the appVersion is in this phase.
	// +optional
	Message string `json:"message,omitempty"`

	// generation number
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	ClusterDefinitionStatusGeneration `json:",inline"`
}

// AppVersionComponent is an application version component spec.
type AppVersionComponent struct {
	// Type is a component type in ClusterDefinition.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=12
	Type string `json:"type"`

	// ConfigTemplateRefs defines a configuration extension mechanism to handle configuration differences between versions,
	// the configTemplateRefs field, together with configTemplateRefs in the ClusterDefinition,
	// determines the final configuration file.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	ConfigTemplateRefs []ConfigTemplate `json:"configTemplateRefs,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name"`

	// PodSpec is pod spec, if not nil, will replace ClusterDefinitionSpec.PodSpec in ClusterDefinition.
	// +optional
	PodSpec *corev1.PodSpec `json:"podSpec,omitempty"`

	// Service is service spec, if not nil, will replace ClusterDefinitionSpec.Service in ClusterDefinition spec.
	// +optional
	Service *corev1.ServiceSpec `json:"service,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:categories={dbaas},scope=Cluster
//+kubebuilder:printcolumn:name="PHASE",type="string",JSONPath=".status.phase",description="status phase"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// AppVersion is the Schema for the appversions API
type AppVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppVersionSpec   `json:"spec,omitempty"`
	Status AppVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppVersionList contains a list of AppVersion
type AppVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppVersion{}, &AppVersionList{})
}

// GetTypeMappingComponents return Type name mapping AppVersionComponent.
func (r *AppVersion) GetTypeMappingComponents() map[string]*AppVersionComponent {
	m := map[string]*AppVersionComponent{}
	for i, c := range r.Spec.Components {
		m[c.Type] = &r.Spec.Components[i]
	}
	return m
}
