/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="addon types"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version",description="addon version"
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".spec.provider",description="addon provider"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Addon is the Schema for the add-ons API.
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddonList contains a list of add-ons.
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// Specifies the description of the add-on.
	//
	// +optional
	Description string `json:"description,omitempty"`

	// Defines the type of the add-on. The only valid value is 'helm'.
	//
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	Type AddonType `json:"type"`

	// Indicates the version of the add-on.
	//
	// +optional
	Version string `json:"version,omitempty"`

	// Specifies the provider of the add-on.
	//
	// +optional
	Provider string `json:"provider,omitempty"`

	// Represents the Helm installation specifications. This is only processed
	// when the type is set to 'helm'.
	//
	// +optional
	Helm *HelmTypeInstallSpec `json:"helm,omitempty"`

	// Specifies the default installation parameters.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DefaultInstallValues []AddonDefaultInstallSpecItem `json:"defaultInstallValues"`

	// Defines the installation parameters.
	//
	// +optional
	InstallSpec *AddonInstallSpec `json:"install,omitempty"`

	// Represents the installable specifications of the add-on. This includes
	// the selector and auto-install settings.
	//
	// +optional
	Installable *InstallableSpec `json:"installable,omitempty"`

	// Specifies the CLI plugin installation specifications.
	//
	// +optional
	CliPlugins []CliPlugin `json:"cliPlugins,omitempty"`
}

// AddonStatus defines the observed state of an add-on.
type AddonStatus struct {
	// Represents the most recent generation observed for this add-on. It corresponds
	// to the add-on's generation, which is updated on mutation by the API Server.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Defines the current installation phase of the add-on. It can take one of
	// the following values: `Disabled`, `Enabled`, `Failed`, `Enabling`, `Disabling`.
	//
	// +kubebuilder:validation:Enum={Disabled,Enabled,Failed,Enabling,Disabling}
	Phase AddonPhase `json:"phase,omitempty"`

	// Provides a detailed description of the current state of add-on API installation.
	//
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AddonType defines the addon types.
// +enum
// +kubebuilder:validation:Enum={Helm}
type AddonType string

const (
	HelmType AddonType = "Helm"
)

// HelmTypeInstallSpec defines the Helm installation spec.
// +kubebuilder:validation:XValidation:rule="self.chartLocationURL.startsWith('file://') ? has(self.chartsImage) : true",message="chartsImage is required when chartLocationURL starts with 'file://'"
type HelmTypeInstallSpec struct {
	// Specifies the URL location of the Helm Chart.
	//
	// +kubebuilder:validation:Required
	ChartLocationURL string `json:"chartLocationURL"`

	// Defines the options for Helm release installation.
	//
	// +optional
	InstallOptions HelmInstallOptions `json:"installOptions,omitempty"`

	// Defines the set values for Helm release installation.
	//
	// +optional
	InstallValues HelmInstallValues `json:"installValues,omitempty"`

	// Defines the mapping of add-on normalized resources parameters to Helm values' keys.
	//
	// +optional
	ValuesMapping HelmValuesMapping `json:"valuesMapping,omitempty"`

	// Defines the image of Helm charts.
	//
	// +optional
	ChartsImage string `json:"chartsImage,omitempty"`

	// Defines the path of Helm charts in the image. This path is used to copy
	// Helm charts from the image to the shared volume. The default path is "/charts".
	//
	// +kubeBuilder:default="/charts"
	// +optional
	ChartsPathInImage string `json:"chartsPathInImage,omitempty"`
}

type HelmInstallOptions map[string]string

type HelmInstallValues struct {
	// Specifies the URL location of the values file.
	//
	// +optional
	URLs []string `json:"urls,omitempty"`

	// Selects a key from a ConfigMap item list. The value can be
	// a JSON or YAML string content. Use a key name with ".json", ".yaml", or ".yml"
	// extension to specify a content type.
	//
	// +optional
	ConfigMapRefs []DataObjectKeySelector `json:"configMapRefs,omitempty"`

	// Selects a key from a Secrets item list. The value can be
	// a JSON or YAML string content. Use a key name with ".json", ".yaml", or ".yml"
	// extension to specify a content type.
	//
	// +optional
	SecretRefs []DataObjectKeySelector `json:"secretRefs,omitempty"`

	// Values set during Helm installation. Multiple or separate values can be specified with commas (key1=val1,key2=val2).
	//
	// +optional
	SetValues []string `json:"setValues,omitempty"`

	// JSON values set during Helm installation. Multiple or separate values can be specified with commas (key1=jsonval1,key2=jsonval2).
	//
	// +optional
	SetJSONValues []string `json:"setJSONValues,omitempty"`
}

type DataObjectKeySelector struct {
	// Defines the name of the object being referred to.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"` // need corev1.LocalObjectReference

	// Specifies the key to be selected.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type HelmValuesMapping struct {
	HelmValuesMappingItem `json:",inline"`

	// Helm value mapping items for extra items.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraItems []HelmValuesMappingExtraItem `json:"extras,omitempty"`
}

type HelmValuesMappingItem struct {
	// Defines the "key" mapping values. Valid keys include `replicaCount`,
	// `persistentVolumeEnabled`, and `storageClass`.
	// Enum values explained:
	//
	// - `replicaCount` sets the replicaCount value mapping key.
	// - `persistentVolumeEnabled` sets the persistent volume enabled mapping key.
	// - `storageClass` sets the storageClass mapping key.
	//
	// +optional
	HelmValueMap HelmValueMapType `json:"valueMap,omitempty"`

	// Defines the "key" mapping values. The valid key is tolerations.
	// Enum values explained:
	//
	// - `tolerations` sets the toleration mapping key.
	//
	// +optional
	HelmJSONMap HelmJSONValueMapType `json:"jsonMap,omitempty"`

	// Sets resources related mapping keys.
	//
	// +optional
	ResourcesMapping *ResourceMappingItem `json:"resources,omitempty"`
}

type HelmValueMapType struct {
	// Defines the key for setting the replica count in the Helm values map.
	//
	// +optional
	ReplicaCount string `json:"replicaCount,omitempty"`

	// Indicates whether the persistent volume is enabled in the Helm values map.
	//
	// +optional
	PVEnabled string `json:"persistentVolumeEnabled,omitempty"`

	// Specifies the key for setting the storage class in the Helm values map.
	//
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type ResourceMappingItem struct {
	// Specifies the key used for mapping the storage size value.
	//
	// +optional
	Storage string `json:"storage,omitempty"`

	// Specifies the key used for mapping both CPU requests and limits.
	//
	// +optional
	CPU *ResourceReqLimItem `json:"cpu,omitempty"`

	// Specifies the key used for mapping both Memory requests and limits.
	//
	// +optional
	Memory *ResourceReqLimItem `json:"memory,omitempty"`
}

type ResourceReqLimItem struct {
	// Specifies the mapping key for the request value.
	//
	// +optional
	Requests string `json:"requests,omitempty"`

	// Specifies the mapping key for the limit value.
	//
	// +optional
	Limits string `json:"limits,omitempty"`
}

type HelmJSONValueMapType struct {
	// Specifies the toleration mapping key.
	//
	// +optional
	Tolerations string `json:"tolerations,omitempty"`
}

type HelmValuesMappingExtraItem struct {
	HelmValuesMappingItem `json:",inline"`

	// Name of the item.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type AddonDefaultInstallSpecItem struct {
	AddonInstallSpec `json:",inline"`

	// Indicates the default selectors for add-on installations. If multiple selectors are provided,
	// all selectors must evaluate to true.
	//
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`
}

type AddonInstallSpec struct {
	AddonInstallSpecItem `json:",inline"`

	// Can be set to true if there are no specific installation attributes to be set.
	//
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Specifies the installation specifications for extra items.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraItems []AddonInstallExtraItem `json:"extras,omitempty"`
}

type AddonInstallSpecItem struct {
	// Specifies the number of replicas.
	//
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Indicates whether the Persistent Volume is enabled or not.
	//
	// +optional
	PVEnabled *bool `json:"persistentVolumeEnabled,omitempty"`

	// Specifies the name of the storage class.
	//
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Specifies the tolerations in a JSON array string format.
	//
	// +optional
	Tolerations string `json:"tolerations,omitempty"`

	// Specifies the resource requirements.
	//
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`
}

type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
	//
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified;
	// otherwise, it defaults to an implementation-defined value.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
	//
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

type AddonInstallExtraItem struct {
	AddonInstallSpecItem `json:",inline"`

	// Specifies the name of the item.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type SelectorRequirement struct {
	// The selector key. Valid values are KubeVersion, KubeGitVersion and KubeProvider.
	//
	// - `KubeVersion` the semver expression of Kubernetes versions, i.e., v1.24.
	// - `KubeGitVersion` may contain distro. info., i.e., v1.24.4+eks.
	// - `KubeProvider` the Kubernetes provider, i.e., aws, gcp, azure, huaweiCloud, tencentCloud etc.
	//
	// +kubebuilder:validation:Required
	Key AddonSelectorKey `json:"key"`

	// Represents a key's relationship to a set of values.
	// Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.
	//
	// Possible enum values:
	//
	// - `Contains` line contains a string.
	// - `DoesNotContain` line does not contain a string.
	// - `MatchRegex` line contains a match to the regular expression.
	// - `DoesNotMatchRegex` line does not contain a match to the regular expression.
	//
	// +kubebuilder:validation:Required
	Operator LineSelectorOperator `json:"operator"`

	// Represents an array of string values. This serves as an "OR" expression to the operator.
	//
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

// AddonSelectorKey are selector requirement key types.
// +enum
// +kubebuilder:validation:Enum={KubeGitVersion,KubeVersion,KubeProvider}
type AddonSelectorKey string

const (
	KubeGitVersion AddonSelectorKey = "KubeGitVersion"
	KubeVersion    AddonSelectorKey = "KubeVersion"
	KubeProvider   AddonSelectorKey = "KubeProvider"
)

// LineSelectorOperator defines line selector operators.
// +enum
// +kubebuilder:validation:Enum={Contains,DoesNotContain,MatchRegex,DoesNotMatchRegex}
type LineSelectorOperator string

const (
	Contains          LineSelectorOperator = "Contains"
	DoesNotContain    LineSelectorOperator = "DoesNotContain"
	MatchRegex        LineSelectorOperator = "MatchRegex"
	DoesNotMatchRegex LineSelectorOperator = "DoesNotMatchRegex"
)

type InstallableSpec struct {
	// Specifies the selectors for add-on installation. If multiple selectors are provided,
	// they must all evaluate to true for the add-on to be installed.
	//
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`

	// Indicates whether an add-on should be installed automatically.
	//
	// +kubebuilder:default=false
	AutoInstall bool `json:"autoInstall"`
}

type CliPlugin struct {
	// Specifies the name of the plugin.
	//
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Defines the index repository of the plugin.
	//
	// +kubebuilder:validation:Required
	IndexRepository string `json:"indexRepository"`

	// Provides a brief description of the plugin.
	//
	// +optional
	Description string `json:"description,omitempty"`
}

// AddonPhase defines addon phases.
// +enum
type AddonPhase string

const (
	AddonDisabled  AddonPhase = "Disabled"
	AddonEnabled   AddonPhase = "Enabled"
	AddonFailed    AddonPhase = "Failed"
	AddonEnabling  AddonPhase = "Enabling"
	AddonDisabling AddonPhase = "Disabling"
)
