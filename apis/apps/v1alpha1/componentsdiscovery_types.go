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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
)

// ComponentsDiscoverySpec defines the desired state of ComponentsDiscovery
type ComponentsDiscoverySpec struct {
	// Discovery type, valid values are Helm, Manifests, ExistingCluster.
	// +kubebuilder:validation:Required
	Type DiscoveryType `json:"type"`

	// Helm discovery spec., it's only being processed if type=Helm.
	// +optional
	Helm *HelmDiscoverySpec `json:"helm,omitempty"`

	// Manifest discovery spec., it's only being processed if type=Manifests.
	// +optional
	Manifests *ManifestsDiscoverySpec `json:"manifests,omitempty"`

	// Cluster discovery spec., it's only being processed if type=Cluster.
	// +optional
	Cluster *ClusterDiscoverySpec `json:"cluster,omitempty"`

	// componentDefSelectors defines clusterdefinition.spec.componentDefs[] override
	// attributes, and workload related resources selectors.
	// +optional
	ComponentDefSelectors []ComponentDefSelector `json:"componentDefSelectors,omitempty"`
}

type ComponentDefSelector struct {
	// Name of component definition.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// workloadSelector defines workload label selector, the selector will attempt to
	// select 1st discovered statefulsets.apps, deployment.apps and set to
	// workloadType=[stateful | stateless], only workload's `.spec.template.spec` is preserved.
	// +optional
	WorkloadSelector map[string]string `json:"workloadSelector,omitempty"`

	// serviceSelector defines primary service label selector.
	// +optional
	ServiceSelector map[string]string `json:"serviceSelector,omitempty"`

	// configsSelector defines primary config file configMap items label selector.
	// +optional
	ConfigsSelector map[string]string `json:"configsSelector,omitempty"`

	// scriptsSelector defines script file configMap items label selector.
	// +optional
	ScriptsSelector map[string]string `json:"scriptsSelector,omitempty"`

	// pdbSelector defines PodDistruptionBudget object selector to derive max maxUnavailable value.
	// +optional
	PDBSelector map[string]string `json:"pdbSelector,omitempty"`

	// componentDefinitionOverride define override value for generating clusterdefinition.spec.componentDefs[] spec.ma
	// +optional
	ComponentDefinitionOverride *ClusterComponentDefinitionSpec `json:"componentDefinitionOverride,omitempty"`
}

type HelmDiscoverySpec struct {
	// A Helm Chart location URL.
	// +kubebuilder:validation:Required
	ChartLocationURL string `json:"chartLocationURL"`

	// installOptions defines Helm release install options.
	// +optional
	InstallOptions extensionsv1alpha1.HelmInstallOptions `json:"installOptions,omitempty"`

	// HelmInstallValues defines Helm release install set values.
	// +optional
	InstallValues HelmInstallValues `json:"installValues,omitempty"`
}

type HelmInstallValues struct {
	// valuesItems correspond to `helm install --values` flag, specify values in
	// a YAML file or a URL (can specify multiple).
	// +optional
	ValuesItems []string `json:"valuesItems,omitempty"`

	// setItems correspond to `helm install --set` flag,  specify set values, can
	// specify multiple or separate values with commas(key1=val1,key2=val2).
	// +optional
	SetValues []string `json:"setItems,omitempty"`

	// setJSONItems correspond to `helm install --set-json` flag, specify set JSON values,
	// can specify multiple or separate values with commas(key1=jsonval1,key2=jsonval2).
	// +optional
	SetJSONValues []string `json:"setJSONItems,omitempty"`
}

type ManifestsDiscoverySpec struct {
	// A location URL, accept 'file://' protocol.
	// +kubebuilder:validation:Required
	LocationURL string `json:"chartLocationURL"`
}

type ClusterDiscoverySpec struct {
	// +kubebuilder:validation:Required
	Namespace string           `json:"namespace"`
	Selectors []ObjectSelector `json:"selectors"`
}

type ObjectSelector struct {
	// gvk specify Group-Version-Kind API target to select, in syntax of <group>/<version>/<Kind>,
	// or for group-less APIs in syntax of <version>/<Kind>.
	// +kubebuilder:validation:Required
	GVK string `json:"gvk"`

	// selector is an object kind selector.
	// Selector which must match objects' labels.
	Selector map[string]string `json:"selector"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ComponentsDiscovery is the Schema for the componentsdiscoveries API
type ComponentsDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentsDiscoverySpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ComponentsDiscoveryList contains a list of ComponentsDiscovery
type ComponentsDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentsDiscovery `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentsDiscovery{}, &ComponentsDiscoveryList{})
}
