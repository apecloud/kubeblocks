/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

// Package v1alpha1 contains API Schema definitions for the apps v1alpha1 API group
package v1alpha1

const (
	APIVersion = "apps.kubeblocks.io/v1alpha1"
)

type ComponentTemplateSpec struct {
	// Specifies the name of the configuration template.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +kubebuilder:default="default"
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Refers to the volume name of PodTemplate. The configuration file produced through the configuration
	// template will be mounted to the corresponding volume. Must be a DNS_LABEL name.
	// The volume name must be defined in podSpec.containers[*].volumeMounts.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	VolumeName string `json:"volumeName"`

	// The operator attempts to set default file permissions for scripts (0555) and configurations (0444).
	// However, certain database engines may require different file permissions.
	// You can specify the desired file permissions here.
	//
	// Must be specified as an octal value between 0000 and 0777 (inclusive),
	// or as a decimal value between 0 and 511 (inclusive).
	// YAML supports both octal and decimal values for file permissions.
	//
	// Please note that this setting only affects the permissions of the files themselves.
	// Directories within the specified path are not impacted by this setting.
	// It's important to be aware that this setting might conflict with other options
	// that influence the file mode, such as fsGroup.
	// In such cases, the resulting file mode may have additional bits set.
	// Refers to documents of k8s.ConfigMapVolumeSource.defaultMode for more information.
	//
	// +optional
	DefaultMode *int32 `json:"defaultMode,omitempty" protobuf:"varint,3,opt,name=defaultMode"`
}

type ConfigTemplateExtension struct {
	// Specifies the name of the referenced configuration template ConfigMap object.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	TemplateRef string `json:"templateRef"`

	// Specifies the namespace of the referenced configuration template ConfigMap object.
	// An empty namespace is equivalent to the "default" namespace.
	//
	// +kubebuilder:default="default"
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Defines the strategy for merging externally imported templates into component templates.
	//
	// +kubebuilder:default="none"
	// +optional
	Policy MergedPolicy `json:"policy,omitempty"`
}

// LegacyRenderedTemplateSpec describes the configuration extension for the lazy rendered template.
//
// Deprecated: LegacyRenderedTemplateSpec has been deprecated since 0.9.0 and will be removed in 0.10.0
type LegacyRenderedTemplateSpec struct {
	// Extends the configuration template.
	ConfigTemplateExtension `json:",inline"`
}

type ComponentConfigSpec struct {
	ComponentTemplateSpec `json:",inline"`

	// Specifies the configuration files within the ConfigMap that support dynamic updates.
	//
	// A configuration template (provided in the form of a ConfigMap) may contain templates for multiple
	// configuration files.
	// Each configuration file corresponds to a key in the ConfigMap.
	// Some of these configuration files may support dynamic modification and reloading without requiring
	// a pod restart.
	//
	// If empty or omitted, all configuration files in the ConfigMap are assumed to support dynamic updates,
	// and ConfigConstraint applies to all keys.
	//
	// +listType=set
	// +optional
	Keys []string `json:"keys,omitempty"`

	// Specifies the secondary rendered config spec for pod-specific customization.
	//
	// The template is rendered inside the pod (by the "config-manager" sidecar container) and merged with the main
	// template's render result to generate the final configuration file.
	//
	// This field is intended to handle scenarios where different pods within the same Component have
	// varying configurations. It allows for pod-specific customization of the configuration.
	//
	// Note: This field will be deprecated in future versions, and the functionality will be moved to
	// `cluster.spec.componentSpecs[*].instances[*]`.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +optional
	LegacyRenderedConfigSpec *LegacyRenderedTemplateSpec `json:"legacyRenderedConfigSpec,omitempty"`

	// Specifies the name of the referenced configuration constraints object.
	//
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern:=`^[a-z0-9]([a-z0-9\.\-]*[a-z0-9])?$`
	// +optional
	ConfigConstraintRef string `json:"constraintRef,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	// Deprecated: `asEnvFrom` has been deprecated since 0.9.0 and will be removed in 0.10.0.
	// Use `injectEnvTo` instead.
	//
	// +kubebuilder:deprecatedversion:warning="This field has been deprecated since 0.9.0 and will be removed in 0.10.0"
	// +listType=set
	// +optional
	AsEnvFrom []string `json:"asEnvFrom,omitempty"`

	// Specifies the containers to inject the ConfigMap parameters as environment variables.
	//
	// This is useful when application images accept parameters through environment variables and
	// generate the final configuration file in the startup script based on these variables.
	//
	// This field allows users to specify a list of container names, and KubeBlocks will inject the environment
	// variables converted from the ConfigMap into these designated containers. This provides a flexible way to
	// pass the configuration items from the ConfigMap to the container without modifying the image.
	//
	//
	// +listType=set
	// +optional
	InjectEnvTo []string `json:"injectEnvTo,omitempty"`

	// Specifies whether the configuration needs to be re-rendered after v-scale or h-scale operations to reflect changes.
	//
	// In some scenarios, the configuration may need to be updated to reflect the changes in resource allocation
	// or cluster topology. Examples:
	//
	// - Redis: adjust maxmemory after v-scale operation.
	// - MySQL: increase max connections after v-scale operation.
	// - Zookeeper: update zoo.cfg with new node addresses after h-scale operation.
	//
	// +listType=set
	// +optional
	ReRenderResourceTypes []RerenderResourceType `json:"reRenderResourceTypes,omitempty"`

	// Whether to store the final rendered parameters as a secret.
	//
	// +optional
	AsSecret *bool `json:"asSecret,omitempty"`
}

// RerenderResourceType defines the resource requirements for a component.
// +enum
// +kubebuilder:validation:Enum={vscale,hscale,tls,shardingHScale}
type RerenderResourceType string

const (
	ComponentVScaleType         RerenderResourceType = "vscale"
	ComponentHScaleType         RerenderResourceType = "hscale"
	ShardingComponentHScaleType RerenderResourceType = "shardingHScale"
)

// MergedPolicy defines how to merge external imported templates into component templates.
// +enum
// +kubebuilder:validation:Enum={patch,replace,none}
type MergedPolicy string

const (
	PatchPolicy     MergedPolicy = "patch"
	ReplacePolicy   MergedPolicy = "replace"
	OnlyAddPolicy   MergedPolicy = "add"
	NoneMergePolicy MergedPolicy = "none"
)
