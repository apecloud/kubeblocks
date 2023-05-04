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
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"

	"github.com/apecloud/kubeblocks/internal/constant"
)

// AddonSpec defines the desired state of Addon
// +kubebuilder:validation:XValidation:rule="has(self.type) && self.type == 'Helm' ?  has(self.helm) : !has(self.helm)",message="spec.helm is required when spec.type is Helm, and forbidden otherwise"
type AddonSpec struct {
	// Addon description.
	// +optional
	Description string `json:"description,omitempty"`

	// Addon type, valid value is helm.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	Type AddonType `json:"type"`

	// Helm installation spec., it's only being processed if type=helm.
	// +optional
	Helm *HelmTypeInstallSpec `json:"helm,omitempty"`

	// Default installation parameters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DefaultInstallValues []AddonDefaultInstallSpecItem `json:"defaultInstallValues"`

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
	// `"Contains"` line contains string
	// `"DoesNotContain"` line does not contain string
	// `"MatchRegex"` line contains a match to the regular expression
	// `"DoesNotMatchRegex"` line does not contain a match to the regular expression
	// +kubebuilder:validation:Required
	Operator LineSelectorOperator `json:"operator"`

	// An array of string values. Server as "OR" expression to the operator.
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

type HelmTypeInstallSpec struct {
	// A Helm Chart location URL.
	// +kubebuilder:validation:Required
	ChartLocationURL string `json:"chartLocationURL"`

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
	SecretRefs []DataObjectKeySelector `json:"secretRefs,omitempty"`

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

type HelmValueMapType struct {
	// replicaCount sets replicaCount value mapping key.
	// +optional
	ReplicaCount string `json:"replicaCount,omitempty"`

	// persistentVolumeEnabled persistent volume enabled mapping key.
	// +optional
	PVEnabled string `json:"persistentVolumeEnabled,omitempty"`

	// storageClass sets storageClass mapping key.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type HelmJSONValueMapType struct {
	// tolerations sets toleration mapping key.
	// +optional
	Tolerations string `json:"tolerations,omitempty"`
}

type HelmValuesMappingItem struct {
	// valueMap define the "key" mapping values, valid keys are replicaCount,
	// persistentVolumeEnabled, and storageClass. Enum values explained:
	// `"replicaCount"` sets replicaCount value mapping key
	// `"persistentVolumeEnabled"` sets persistent volume enabled mapping key
	// `"storageClass"` sets storageClass mapping key
	// +optional
	HelmValueMap HelmValueMapType `json:"valueMap,omitempty"`

	// jsonMap define the "key" mapping values, valid keys are tolerations.
	// Enum values explained:
	// `"tolerations"` sets toleration mapping key
	// +optional
	HelmJSONMap HelmJSONValueMapType `json:"jsonMap,omitempty"`

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

	// enabled can be set if there are no specific installation attributes to be set.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

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

// GetExtraNames exacter extra items' name.
func (r *Addon) GetExtraNames() []string {
	if r == nil {
		return nil
	}
	switch r.Spec.Type {
	case HelmType:
		if r.Spec.Helm == nil {
			return nil
		}
		// r.Spec.DefaultInstallValues has minItem=1 constraint
		names := make([]string, 0, len(r.Spec.Helm.ValuesMapping.ExtraItems))
		for _, i := range r.Spec.Helm.ValuesMapping.ExtraItems {
			names = append(names, i.Name)
		}
		return names
	default:
		return nil
	}

}

func buildSelectorStrings(selectors []SelectorRequirement) []string {
	l := len(selectors)
	if l == 0 {
		return nil
	}
	sl := make([]string, 0, l)
	for _, req := range selectors {
		sl = append(sl, req.String())
	}
	return sl
}

// GetSelectorsStrings extract selectors to string representations.
func (r AddonDefaultInstallSpecItem) GetSelectorsStrings() []string {
	return buildSelectorStrings(r.Selectors)
}

// GetSelectorsStrings extract selectors to string representations.
func (r *InstallableSpec) GetSelectorsStrings() []string {
	if r == nil {
		return nil
	}
	return buildSelectorStrings(r.Selectors)
}

func (r SelectorRequirement) String() string {
	return fmt.Sprintf("{key=%s,op=%s,values=%v}",
		r.Key, r.Operator, r.Values)
}

// MatchesFromConfig matches selector requirement value.
func (r SelectorRequirement) MatchesFromConfig() bool {
	verIf := viper.Get(constant.CfgKeyServerInfo)
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

func (r SelectorRequirement) matchesLine(line string) bool {
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

// GetEnabled provides Enabled property getter.
func (r *AddonInstallSpec) GetEnabled() bool {
	if r == nil {
		return false
	}
	return r.Enabled
}

// BuildMergedValues merge values from a AddonInstallSpec and pre-set values.
func (r *HelmTypeInstallSpec) BuildMergedValues(installSpec *AddonInstallSpec) HelmInstallValues {
	if r == nil {
		return HelmInstallValues{}
	}
	installValues := r.InstallValues
	processor := func(installSpecItem AddonInstallSpecItem, valueMapping HelmValuesMappingItem) {
		if installSpecItem.Replicas != nil && *installSpecItem.Replicas >= 0 {
			if v := valueMapping.HelmValueMap.ReplicaCount; v != "" {
				installValues.SetValues = append(installValues.SetValues,
					fmt.Sprintf("%s=%v", v, *installSpecItem.Replicas))
			}
		}

		if installSpecItem.StorageClass != "" {
			if v := valueMapping.HelmValueMap.StorageClass; v != "" {
				if installSpecItem.StorageClass == "-" {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=null", v))
				} else {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", v, installSpecItem.StorageClass))
				}
			}
		}

		if installSpecItem.PVEnabled != nil {
			if v := valueMapping.HelmValueMap.PVEnabled; v != "" {
				installValues.SetValues = append(installValues.SetValues,
					fmt.Sprintf("%s=%v", v, *installSpecItem.PVEnabled))
			}
		}

		if installSpecItem.Tolerations != "" {
			if v := valueMapping.HelmJSONMap.Tolerations; v != "" {
				installValues.SetJSONValues = append(installValues.SetJSONValues,
					fmt.Sprintf("%s=%s", v, installSpecItem.Tolerations))
			}
		}

		for k, v := range installSpecItem.Resources.Requests {
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

		for k, v := range installSpecItem.Resources.Limits {
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
	processor(installSpec.AddonInstallSpecItem, r.ValuesMapping.HelmValuesMappingItem)
	for _, ei := range installSpec.ExtraItems {
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
func (r AddonSpec) GetSortedDefaultInstallValues() []AddonDefaultInstallSpecItem {
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

// NewAddonInstallSpecItem creates an initialized AddonInstallSpecItem object
func NewAddonInstallSpecItem() AddonInstallSpecItem {
	return AddonInstallSpecItem{
		Resources: ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		},
	}
}
