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

// AddonSpecSpec defines the desired state of AddonSpec
type AddonSpecSpec struct {
	// Addon description.
	// +optional
	Description string `json:"description,omitempty"`

	// Addon type, valid value is helm.
	// +kubebuilder:validation:Required
	Type AddonType `json:"type"`

	// Helm installation spec., it's only being processed if type=helm.
	// +optional
	Helm *HelmInstallSpec `json:"helm,omitempty"`

	// Default installation parameters.
	// +optional
	DefaultInstallSpec []AddonDefaultInstallSpecItem `json:"defaultInstall,omitempty"`

	// Installation parameters.
	// +optional
	InstallSpec *AddonInstallSpec `json:"install,omitempty"`

	// Addon installable spec., provide selector and auto-install settings.
	// +optional
	Installable *InstallableSpec `json:"installable,omitempty"`
}

// AddonSpecStatus defines the observed state of AddonSpec
type AddonSpecStatus struct {
	// Addon installation phases. Value values are Disabled, Enabled, Failed, Enabling, Disabling.
	Phase AddonPhase `json:"addonPhase,omitempty"`

	// Describe current state of AddonSpec API installation conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type InstallableSpec struct {
	// Addon installable selectors. If multiple selectors are provided
	// that all selectors must evaluate to true.
	// +optional
	Selector []SelectorRequirement `json:"selectors,omitempty"`

	// autoInstall defines an addon should auto installed
	// +kubebuilder:default=false
	AutoInstall bool `json:"autoInstall"`
}

// SelectorRequirement is the installation selector requirement.
type SelectorRequirement struct {
	// The selector key, valid values are kubeGitVersion, kubeVersion.
	// "kubeVersion" the semver expression of Kubernetes versions, i.e., v1.24.
	// "kubeGitVersion" may contain distro. info., i.e., v1.24.4+eks.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Represents a key's relationship to a set of values.
	// Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.
	//
	// Possible enum values:
	// `"Contains"` line contains string (symbol: "|="）
	// `"DoesNotContain"` line does not contain string (symbol: "!=")
	// `"MatchRegex"` line contains a match to the regular expression (symbol: "|~"）
	// `"DoesNoteMatchRegex"` line does not contain a match to the regular expression (symbol: "!~")
	// +kubebuilder:validation:Required
	Operator LineSelectorOperator `json:"operator"`

	// An array of string values. Server as "OR" expression to the operator.
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

// HelmInstallSpec defines a Helm release installation spec.
type HelmInstallSpec struct {

	// A Helm Chart repository URL.
	// +kubebuilder:validation:Required
	ChartRepoURL string `json:"chartRepoURL"`

	// installOptions defines Helm release install options.
	// +optional
	InstallOptions HelmInstallOptions `json:"installOptions,omitempty"`

	// HelmInstallValues defines Helm release install set values.
	// +optional
	InstallValues *HelmInstallValues `json:"installValues,omitempty"`

	// valuesMapping defines addon normalized resources parameters mapped to Helm values' keys.
	// +optional
	ValuesMapping *HelmValuesMapping `json:"valuesMapping,omitempty"`
}

type HelmInstallOptions map[string]string

type HelmInstallValues struct {
	// +optional
	URLs []string `json:"urls,omitempty"`

	// Selects a key of a ConfigMap item list, the value of ConfigMap can be
	// a JSON or YAML string content, use key name with ".json" or ".yaml" or ".yml"
	// extension name to specify content type.
	// +optional
	ConfigMapRefs []DataObjectKeySelector `json:"configMapRefs,omitempty"`

	// Selects a key of a Secrets item list, the value of Secrets can be
	// a JSON or YAML string content, use key name with ".json" or ".yaml" or ".yml"
	// extension name to specify content type.
	// +optional
	SecretRefs []DataObjectKeySelector `json:"secretsRefs,omitempty"`

	// Helm install set values, can specify multiple or separate values with commas(key1=val1,key2=val2).
	// +optional
	SetValues []string `json:"setValues,omitempty"`

	// Helm install set JSON values, can specify multiple or separate values with commas(key1=jsonval1,key2=jsonval2).
	// +optional
	SetJSONValues []string `json:"setJSONValues,omitempty"`
}

type HelmValuesMapping struct {
	HelmValuesMappingItem `json:",inline"`

	// Helm value mapping items for extra items.
	// +optional
	ExtraItems []HelmValuesMappingExtraItem `json:"extras,omitempty"`
}

type HelmValuesMappingExtraItem struct {
	HelmValuesMappingItem `json:",inline"`

	// Name of the item.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type HelmValueMap map[KeyHelmValueKey]string

type HelmValuesMappingItem struct {
	// valueMap define the "key" mapping values, valid keys are ReplicaCount,
	// PVEnabled, and StorageClass. Enum values explained:
	// `"ReplicaCount"` sets replicaCount value mapping key
	// `"PVEnabled"` sets persistent volume enabled mapping key
	// `"StorageClass"` sets storageClass mapping key
	// +optional
	HelmValueMap HelmValueMap `json:"valueMap,omitempty"`

	// jsonMap define the "key" mapping values, valid keys are Toleration.
	// Enum values explained:
	// `"Toleration"` sets toleration mapping key
	// +optional
	HelmJSONMap HelmValueMap `json:"jsonMap,omitempty"`

	// resources sets resources related mapping keys.
	// +optional
	ResourcesMapping *ResourceMappingItem `json:"resources,omitempty"`
}

type ResourceMappingItem struct {

	// storage sets storage size value mapping key.
	// +optional
	Storage string `json:"storage,omitempty"`

	// cpu sets CPU requests and limits mapping keys.
	// +optional
	CPU *ResourceReqLimItem `json:"cpu,omitempty"`

	// memory sets Memory requests and limits mapping keys.
	// +optional
	Memory *ResourceReqLimItem `json:"memory,omitempty"`
}

type ResourceReqLimItem struct {
	// Requests value mapping key.
	// +optional
	Requests string `json:"requests,omitempty"`

	// Limits value mapping key.
	// +optional
	Limits string `json:"limits,omitempty"`
}

type DataObjectKeySelector struct {
	// Object name of the referent.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the referent.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// The key to select.
	// +kubebuilder:validation:Required
	Key string `json:"key" protobuf:"bytes,2,opt,name=key"`
}

type AddonDefaultInstallSpecItem struct {
	AddonInstallSpec `json:",inline"`

	// Addon default install parameters selectors. If multiple selectors are provided
	// that all selectors must evaluate to true.
	// +optional
	Selector []SelectorRequirement `json:"selectors,omitempty"`
}

type AddonInstallSpec struct {
	AddonInstallSpecItem `json:",inline"`

	// Install spec. for extra items.
	// +optional
	ExtraItems []AddonInstallExtraItem `json:"extras,omitempty"`
}

type AddonInstallExtraItem struct {
	AddonInstallSpecItem `json:",inline"`

	// Name of the item.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type AddonInstallSpecItem struct {
	// Installation version, for type=helm, this value is corresponding to Helm chart version (SemVer).
	// +optional
	Version string `json:"version,omitempty"`

	// Replicas value.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Resource requirements.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AddonSpec is the Schema for the addonspecs API
type AddonSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpecSpec   `json:"spec,omitempty"`
	Status AddonSpecStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AddonSpecList contains a list of AddonSpec
type AddonSpecList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AddonSpec `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AddonSpec{}, &AddonSpecList{})
}
