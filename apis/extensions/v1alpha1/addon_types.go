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
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"

	"github.com/apecloud/kubeblocks/internal/constant"
)

// AddonSpec defines the desired state of an add-on.
// +kubebuilder:validation:XValidation:rule="has(self.type) && self.type == 'Helm' ?  has(self.helm) : !has(self.helm)",message="spec.helm is required when spec.type is Helm, and forbidden otherwise"
type AddonSpec struct {
	// Addon description.
	// +optional
	Description string `json:"description,omitempty"`

	// Add-on type. The valid value is helm.
	// +unionDiscriminator
	// +kubebuilder:validation:Required
	Type AddonType `json:"type"`

	// Helm installation spec. It's processed only when type=helm.
	// +optional
	Helm *HelmTypeInstallSpec `json:"helm,omitempty"`

	// Default installation parameters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DefaultInstallValues []AddonDefaultInstallSpecItem `json:"defaultInstallValues"`

	// Installation parameters.
	// +optional
	InstallSpec *AddonInstallSpec `json:"install,omitempty"`

	// Addon installable spec. It provides selector and auto-install settings.
	// +optional
	Installable *InstallableSpec `json:"installable,omitempty"`

	// Plugin installation spec.
	// +optional
	CliPlugins []CliPlugin `json:"cliPlugins,omitempty"`
}

// AddonStatus defines the observed state of an add-on.
type AddonStatus struct {
	// Add-on installation phases. Valid values are Disabled, Enabled, Failed, Enabling, Disabling.
	// +kubebuilder:validation:Enum={Disabled,Enabled,Failed,Enabling,Disabling}
	Phase AddonPhase `json:"phase,omitempty"`

	// Describes the current state of add-on API installation conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recent generation observed for this
	// add-on. It corresponds to the add-on's generation, which is
	// updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type InstallableSpec struct {
	// Add-on installable selectors. If multiple selectors are provided,
	// all selectors must evaluate to true.
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`

	// autoInstall defines an add-on should be installed automatically.
	// +kubebuilder:default=false
	AutoInstall bool `json:"autoInstall"`
}

type SelectorRequirement struct {
	// The selector key. Valid values are KubeVersion, KubeGitVersion.
	// "KubeVersion" the semver expression of Kubernetes versions, i.e., v1.24.
	// "KubeGitVersion" may contain distro. info., i.e., v1.24.4+eks.
	// +kubebuilder:validation:Required
	Key AddonSelectorKey `json:"key"`

	// Represents a key's relationship to a set of values.
	// Valid operators are Contains, NotIn, DoesNotContain, MatchRegex, and DoesNoteMatchRegex.
	//
	// Possible enum values:
	// `"Contains"` line contains a string.
	// `"DoesNotContain"` line does not contain a string.
	// `"MatchRegex"` line contains a match to the regular expression.
	// `"DoesNotMatchRegex"` line does not contain a match to the regular expression.
	// +kubebuilder:validation:Required
	Operator LineSelectorOperator `json:"operator"`

	// An array of string values. It serves as an "OR" expression to the operator.
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

type HelmTypeInstallSpec struct {
	// A Helm Chart location URL.
	// +kubebuilder:validation:Required
	ChartLocationURL string `json:"chartLocationURL"`

	// installOptions defines Helm release installation options.
	// +optional
	InstallOptions HelmInstallOptions `json:"installOptions,omitempty"`

	// HelmInstallValues defines Helm release installation set values.
	// +optional
	InstallValues HelmInstallValues `json:"installValues,omitempty"`

	// valuesMapping defines add-on normalized resources parameters mapped to Helm values' keys.
	// +optional
	ValuesMapping HelmValuesMapping `json:"valuesMapping,omitempty"`
}

type HelmInstallOptions map[string]string

type HelmInstallValues struct {
	// +optional
	URLs []string `json:"urls,omitempty"`

	// Selects a key of a ConfigMap item list. The value of ConfigMap can be
	// a JSON or YAML string content. Use a key name with ".json" or ".yaml" or ".yml"
	// extension name to specify a content type.
	// +optional
	ConfigMapRefs []DataObjectKeySelector `json:"configMapRefs,omitempty"`

	// Selects a key of a Secrets item list. The value of Secrets can be
	// a JSON or YAML string content. Use a key name with ".json" or ".yaml" or ".yml"
	// extension name to specify a content type.
	// +optional
	SecretRefs []DataObjectKeySelector `json:"secretRefs,omitempty"`

	// Helm install set values. It can specify multiple or separate values with commas(key1=val1,key2=val2).
	// +optional
	SetValues []string `json:"setValues,omitempty"`

	// Helm install set JSON values. It can specify multiple or separate values with commas(key1=jsonval1,key2=jsonval2).
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
	// replicaCount sets the replicaCount value mapping key.
	// +optional
	ReplicaCount string `json:"replicaCount,omitempty"`

	// persistentVolumeEnabled sets the persistent volume enabled mapping key.
	// +optional
	PVEnabled string `json:"persistentVolumeEnabled,omitempty"`

	// storageClass sets the storageClass mapping key.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

type HelmJSONValueMapType struct {
	// tolerations sets the toleration mapping key.
	// +optional
	Tolerations string `json:"tolerations,omitempty"`
}

type HelmValuesMappingItem struct {
	// valueMap define the "key" mapping values. Valid keys are replicaCount,
	// persistentVolumeEnabled, and storageClass. Enum values explained:
	// `"replicaCount"` sets the replicaCount value mapping key.
	// `"persistentVolumeEnabled"` sets the persistent volume enabled mapping key.
	// `"storageClass"` sets the storageClass mapping key.
	// +optional
	HelmValueMap HelmValueMapType `json:"valueMap,omitempty"`

	// jsonMap defines the "key" mapping values. The valid key is tolerations.
	// Enum values explained:
	// `"tolerations"` sets the toleration mapping key.
	// +optional
	HelmJSONMap HelmJSONValueMapType `json:"jsonMap,omitempty"`

	// resources sets resources related mapping keys.
	// +optional
	ResourcesMapping *ResourceMappingItem `json:"resources,omitempty"`
}

type ResourceMappingItem struct {

	// storage sets the storage size value mapping key.
	// +optional
	Storage string `json:"storage,omitempty"`

	// pvcSelector defines PVC label selector, if provided, storage expansion
	// will be controlled via PVC resize.
	// +optional
	PVCSelector map[string]string `json:"pvcSelector,omitempty"`

	// cpu sets CPU requests and limits mapping keys.
	// +optional
	CPU *ResourceReqLimItem `json:"cpu,omitempty"`

	// memory sets Memory requests and limits mapping keys.
	// +optional
	Memory *ResourceReqLimItem `json:"memory,omitempty"`
}

type CliPlugin struct {
	// Name of the plugin.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The index repository of the plugin.
	// +kubebuilder:validation:Required
	IndexRepository string `json:"indexRepository"`

	// The description of the plugin.
	// +optional
	Description string `json:"description,omitempty"`
}

func (r *ResourceMappingItem) HasStorageMapping() bool {
	return !(r == nil || r.Storage == "")
}

func (r *ResourceMappingItem) HasCPUReqMapping() bool {
	return !(r == nil || r.CPU == nil || r.CPU.Requests == "")
}

func (r *ResourceMappingItem) HasMemReqMapping() bool {
	return !(r == nil || r.CPU == nil || r.Memory.Requests == "")
}

func (r *ResourceMappingItem) HasCPULimMapping() bool {
	return !(r == nil || r.CPU == nil || r.CPU.Limits == "")
}

func (r *ResourceMappingItem) HasMemLimMapping() bool {
	return !(r == nil || r.CPU == nil || r.Memory.Limits == "")
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

	// Addon installs parameters selectors by default. If multiple selectors are provided,
	// all selectors must evaluate to true.
	// +optional
	Selectors []SelectorRequirement `json:"selectors,omitempty"`
}

type AddonInstallSpec struct {
	AddonInstallSpecItem `json:",inline"`

	// enabled can be set if there are no specific installation attributes to be set.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Installs spec. for extra items.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +optional
	ExtraItems []AddonInstallExtraItem `json:"extras,omitempty"`
}

func (r *AddonInstallSpec) IsDisabled() bool {
	return r == nil || !r.Enabled
}

func (r *AddonInstallSpec) HasSetValues() bool {
	if r == nil {
		return false
	}

	if !r.AddonInstallSpecItem.IsEmpty() {
		return true
	}
	for _, i := range r.ExtraItems {
		if !i.IsEmpty() {
			return true
		}
	}
	return false
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

func (r AddonInstallSpecItem) IsEmpty() bool {
	return r.Replicas == nil &&
		r.PVEnabled == nil &&
		r.StorageClass == "" &&
		r.Tolerations == "" &&
		len(r.Resources.Requests) == 0
}

type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified;
	// otherwise, it defaults to an implementation-defined value.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/.
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories={kubeblocks},scope=Cluster
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type",description="addon types"
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

// GetExtraNames extracts extra items' name.
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

// GetSelectorsStrings extracts selectors to string representations.
func (r AddonDefaultInstallSpecItem) GetSelectorsStrings() []string {
	return buildSelectorStrings(r.Selectors)
}

// GetSelectorsStrings extracts selectors to string representations.
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

// MatchesFromConfig matches the selector requirement value.
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

// GetEnabled provides the Enabled property getter.
func (r *AddonInstallSpec) GetEnabled() bool {
	if r == nil {
		return false
	}
	return r.Enabled
}

// BuildMergedValues merges values from a AddonInstallSpec and pre-set values.
func (r *HelmTypeInstallSpec) BuildMergedValues(installSpec *AddonInstallSpec) HelmInstallValues {
	if r == nil {
		return HelmInstallValues{}
	}
	installValues := r.InstallValues
	processor := func(installSpecItem AddonInstallSpecItem, valueMapping HelmValuesMappingItem) {
		var pvEnabled *bool
		defer func() {
			if v := valueMapping.HelmValueMap.PVEnabled; v != "" && pvEnabled != nil {
				installValues.SetValues = append(installValues.SetValues,
					fmt.Sprintf("%s=%v", v, *pvEnabled))
			}
		}()

		if installSpecItem.PVEnabled != nil {
			b := *installSpecItem.PVEnabled
			pvEnabled = &b
		}

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

		if installSpecItem.Tolerations != "" {
			if v := valueMapping.HelmJSONMap.Tolerations; v != "" {
				installValues.SetJSONValues = append(installValues.SetJSONValues,
					fmt.Sprintf("%s=%s", v, installSpecItem.Tolerations))
			}
		}

		if valueMapping.ResourcesMapping == nil {
			return
		}

		for k, v := range installSpecItem.Resources.Requests {
			switch k {
			case corev1.ResourceStorage:
				if valueMapping.ResourcesMapping.HasStorageMapping() && len(valueMapping.ResourcesMapping.PVCSelector) == 0 {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.Storage, v.ToUnstructured()))
				}
			case corev1.ResourceCPU:
				if valueMapping.ResourcesMapping.HasCPUReqMapping() {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.CPU.Requests, v.ToUnstructured()))
				}
			case corev1.ResourceMemory:
				if valueMapping.ResourcesMapping.HasMemReqMapping() {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.Memory.Requests, v.ToUnstructured()))
				}
			}
		}

		for k, v := range installSpecItem.Resources.Limits {
			switch k {
			case corev1.ResourceCPU:
				if valueMapping.ResourcesMapping.HasCPULimMapping() {
					installValues.SetValues = append(installValues.SetValues,
						fmt.Sprintf("%s=%v", valueMapping.ResourcesMapping.CPU.Limits, v.ToUnstructured()))
				}
			case corev1.ResourceMemory:
				if valueMapping.ResourcesMapping.HasMemLimMapping() {
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

func (r *ResourceMappingItem) HasPVCSelector() bool {
	if r == nil {
		return false
	}
	return len(r.PVCSelector) > 0
}

// BuildContainerArgs derives helm container args.
func (r *HelmTypeInstallSpec) BuildContainerArgs(helmContainer *corev1.Container, installValues HelmInstallValues) error {
	// Add extra helm installation option flags
	for k, v := range r.InstallOptions {
		helmContainer.Args = append(helmContainer.Args, fmt.Sprintf("--%s", k))
		if v != "" {
			helmContainer.Args = append(helmContainer.Args, v)
		}
	}

	// Sets values from URL.
	for _, urlValue := range installValues.URLs {
		helmContainer.Args = append(helmContainer.Args, "--values", urlValue)
	}

	// Sets key1=val1,key2=val2 value.
	if len(installValues.SetValues) > 0 {
		helmContainer.Args = append(helmContainer.Args, "--set",
			strings.Join(installValues.SetValues, ","))
	}

	// Sets key1=jsonval1,key2=jsonval2 JSON value. It can be applied to multiple.
	for _, v := range installValues.SetJSONValues {
		helmContainer.Args = append(helmContainer.Args, "--set-json", v)
	}
	return nil
}

// GetSortedDefaultInstallValues returns DefaultInstallValues items with items that have
// a provided selector first.
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

// NewAddonInstallSpecItem creates an initialized AddonInstallSpecItem object.
func NewAddonInstallSpecItem() AddonInstallSpecItem {
	return AddonInstallSpecItem{
		Resources: ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		},
	}
}
