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
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/version"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddonSpec defines the desired state of Addon
type AddonSpec struct {
	// Addon description.
	// +optional
	Description string `json:"description,omitempty"`

	// Addon type, valid value is helm.
	// +kubebuilder:validation:Required
	Type AddonType `json:"type"`

	// Helm installation spec., it's only being processed if type=helm.
	// +optional
	Helm *HelmTypeInstallSpec `json:"helm,omitempty"`

	// Default installation parameters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DefaultInstallValues []AddonDefaultInstallSpecItem `json:"defaultInstallValues,omitempty"`

	// Installation parameters.
	// +optional
	InstallSpec *AddonInstallSpec `json:"install,omitempty"`

	// Addon installable spec., provide selector and auto-install settings.
	// +optional
	Installable *InstallableSpec `json:"installable,omitempty"`
}

// AddonStatus defines the observed state of Addon
type AddonStatus struct {
	// Addon installation phases. Valid values are Disabled, Enabled, Failed, Enabling, Disabling.
	// +kubebuilder:validation:Enum={Disabled,Enabled,Failed,Enabling,Disabling}
	Phase AddonPhase `json:"phase,omitempty"`

	// Describe current state of Addon API installation conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed for this
	// Addon. It corresponds to the Addon's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type InstallableSpec struct {
	// Addon installable selectors. If multiple selectors are provided
	// that all selectors must evaluate to true.
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`

	// autoInstall defines an addon should auto installed
	// +kubebuilder:default=false
	AutoInstall bool `json:"autoInstall"`
}

// SelectorRequirement is the installation selector requirement.
type SelectorRequirement struct {
	// The selector key, valid values are KubeVersion, KubeGitVersion.
	// "KubeVersion" the semver expression of Kubernetes versions, i.e., v1.24.
	// "KubeGitVersion" may contain distro. info., i.e., v1.24.4+eks.
	// +kubebuilder:validation:Required
	Key AddonSelectorKey `json:"key"`

	// Represents a key's relationship to a set of values.
	// Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.
	//
	// Possible enum values:
	// `"Contains"` line contains string (symbol: "|="）
	// `"DoesNotContain"` line does not contain string (symbol: "!=")
	// `"MatchRegex"` line contains a match to the regular expression (symbol: "|~"）
	// `"DoesNotMatchRegex"` line does not contain a match to the regular expression (symbol: "!~")
	// +kubebuilder:validation:Required
	Operator LineSelectorOperator `json:"operator"`

	// An array of string values. Server as "OR" expression to the operator.
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

// HelmTypeInstallSpec defines a Helm release installation spec.
type HelmTypeInstallSpec struct {

	// A Helm Chart repository URL.
	// +kubebuilder:validation:Required
	ChartRepoURL string `json:"chartRepoURL"`

	// installOptions defines Helm release install options.
	// +optional
	InstallOptions HelmInstallOptions `json:"installOptions,omitempty"`

	// HelmInstallValues defines Helm release install set values.
	// +optional
	InstallValues HelmInstallValues `json:"installValues,omitempty"`

	// valuesMapping defines addon normalized resources parameters mapped to Helm values' keys.
	// +optional
	ValuesMapping HelmValuesMapping `json:"valuesMapping,omitempty"`
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
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraItems []HelmValuesMappingExtraItem `json:"extras,omitempty"`
}

type HelmValuesMappingExtraItem struct {
	HelmValuesMappingItem `json:",inline"`

	// Name of the item.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type HelmValueMapType map[KeyHelmValueKey]string

type HelmValuesMappingItem struct {
	// valueMap define the "key" mapping values, valid keys are ReplicaCount,
	// PVEnabled, and StorageClass. Enum values explained:
	// `"ReplicaCount"` sets replicaCount value mapping key
	// `"PVEnabled"` sets persistent volume enabled mapping key
	// `"StorageClass"` sets storageClass mapping key
	// +optional
	HelmValueMap HelmValueMapType `json:"valueMap,omitempty"`

	// jsonMap define the "key" mapping values, valid keys are Toleration.
	// Enum values explained:
	// `"Toleration"` sets toleration mapping key
	// +optional
	HelmJSONMap HelmValueMapType `json:"jsonMap,omitempty"`

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
	Name string `json:"name"` // need corev1.LocalObjectReference

	// The key to select.
	// +kubebuilder:validation:Required
	Key string `json:"key"`
}

type AddonDefaultInstallSpecItem struct {
	AddonInstallSpec `json:",inline"`

	// Addon default install parameters selectors. If multiple selectors are provided
	// that all selectors must evaluate to true.
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`
}

type AddonInstallSpec struct {
	AddonInstallSpecItem `json:",inline"`

	// Install spec. for extra items.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
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
	// Replicas value.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Persistent Volume Enabled value.
	// +optional
	PVEnabled *bool `json:"persistentVolumeEnabled,omitempty"`

	// Storage class name.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// Tolerations JSON array string value.
	// +optional
	Tolerations string `json:"tolerations,omitempty"`

	// Resource requirements.
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`
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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="addon types"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="status phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Addon is the Schema for the addons API
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AddonList contains a list of Addon
type AddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Addon `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Addon{}, &AddonList{})
}

func (r *SelectorRequirement) String() string {
	return fmt.Sprintf("{key=%s,op=%s,values=%v}",
		r.Key, r.Operator, r.Values)
}

func (r *SelectorRequirement) MatchesFromConfig() bool {
	if r == nil {
		return false
	}
	verIf := viper.Get("_KUBE_SERVER_INFO")
	ver, ok := verIf.(version.Info)
	if !ok {
		return false
	}
	var l string
	switch r.Key {
	case KubeGitVersion:
		l = ver.GitVersion
	case KubeVersion:
		l = fmt.Sprintf("%s.%s", ver.Major, ver.Minor)
	}
	return r.matchesLine(l)
}

func (r *SelectorRequirement) matchesLine(line string) bool {
	processor := func(op bool, predicate func(string) bool) bool {
		if len(r.Values) == 0 {
			return !op
		}
		for _, v := range r.Values {
			m := predicate(v)
			if op && m {
				return true
			} else if !op {
				if m {
					return false
				}
				continue
			}
		}
		return !op
	}

	containsProcessor := func(op bool) bool {
		return processor(op, func(v string) bool {
			return strings.Contains(line, v)
		})
	}

	matchRegexProcessor := func(op bool) bool {
		return processor(op, func(v string) bool {
			regex, err := regexp.Compile(v)
			if err != nil {
				return false
			}
			return regex.Match([]byte(line))
		})
	}

	switch r.Operator {
	case Contains:
		return containsProcessor(true)
	case DoesNotContain:
		return containsProcessor(false)
	case MatchRegex:
		return matchRegexProcessor(true)
	case DoesNotMatchRegex:
		return matchRegexProcessor(false)
	default:
		return false
	}
}

// BuildMergedValues merge values from a AddonInstallSpec and pre-set values.
func (r *HelmTypeInstallSpec) BuildMergedValues(spec *AddonInstallSpec) HelmInstallValues {
	installValues := r.InstallValues
	processor := func(specItem AddonInstallSpecItem, valueMapping HelmValuesMappingItem) {
		if specItem.Replicas != nil && *specItem.Replicas >= 0 {
			if v, ok := valueMapping.HelmValueMap[ReplicaCount]; ok {
				installValues.SetValues = append(installValues.SetValues,
					fmt.Sprintf("%s=%v", v, *spec.Replicas))
			}
		}

		if specItem.StorageClass != "" {
			if v, ok := valueMapping.HelmValueMap[StorageClass]; ok {
				if specItem.StorageClass == "-" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=null", v))
				} else {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", v, spec.StorageClass))
				}
			}
		}

		if specItem.PVEnabled != nil {
			if v, ok := valueMapping.HelmValueMap[PVEnabled]; ok {
				installValues.SetValues = append(installValues.SetValues,
					fmt.Sprintf("%s=%v", v, *specItem.PVEnabled))
			}
		}

		if specItem.Tolerations != "" {
			if v, ok := valueMapping.HelmJSONMap[Tolerations]; ok {
				installValues.SetJSONValues = append(installValues.SetJSONValues,
					fmt.Sprintf("%s=%s", v, specItem.Tolerations))
			}
		}

		for k, v := range specItem.Resources.Requests {
			switch k {
			case corev1.ResourceStorage:
				if valueMapping.ResourcesMapping.Storage != "" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.Storage, v.ToUnstructured()))
				}
			case corev1.ResourceCPU:
				if valueMapping.ResourcesMapping.CPU.Requests != "" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.CPU.Requests, v.ToUnstructured()))
				}
			case corev1.ResourceMemory:
				if valueMapping.ResourcesMapping.Memory.Requests != "" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.Memory.Requests, v.ToUnstructured()))
				}
			}
		}

		for k, v := range specItem.Resources.Limits {
			switch k {
			case corev1.ResourceCPU:
				if valueMapping.ResourcesMapping.CPU.Limits != "" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.CPU.Limits, v.ToUnstructured()))
				}
			case corev1.ResourceMemory:
				if valueMapping.ResourcesMapping.Memory.Limits != "" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.Memory.Limits, v.ToUnstructured()))
				}
			}
		}
	}
	processor(spec.AddonInstallSpecItem, r.ValuesMapping.HelmValuesMappingItem)
	for _, ei := range spec.ExtraItems {
		for _, mei := range r.ValuesMapping.ExtraItems {
			if ei.Name != mei.Name {
				continue
			}
			processor(ei.AddonInstallSpecItem, mei.HelmValuesMappingItem)
			break
		}
	}
	return installValues
}

// GetSortedDefaultInstallValues return DefaultInstallValues items with items that has
// provided selector first.
func (r *AddonSpec) GetSortedDefaultInstallValues() []AddonDefaultInstallSpecItem {
	values := make([]AddonDefaultInstallSpecItem, 0, len(r.DefaultInstallValues))
	nvalues := make([]AddonDefaultInstallSpecItem, 0, len(r.DefaultInstallValues))
	for _, i := range r.DefaultInstallValues {
		if len(i.Selectors) > 0 {
			values = append(values, i)
		} else {
			nvalues = append(nvalues, i)
		}
	}
	if len(nvalues) > 0 {
		values = append(values, nvalues...)
	}
	return values
}
