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
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentResourceConstraintSpec defines the desired state of ComponentResourceConstraint
type ComponentResourceConstraintSpec struct {
	// Component resource constraint rules.
	// +patchMergeKey=name
	// +patchStrategy=merge,retainKeys
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Required
	Rules []ResourceConstraintRule `json:"rules"`

	// selector is used to bind the resource constraint to cluster definitions based on ClusterDefinition API.
	// +listType=map
	// +listMapKey=clusterDefRef
	//+kubebuilder:deprecatedversion:warning="This field is deprecated from KB 0.8.0, use ComponentSelector instead."
	// +optional
	Selector []ClusterResourceConstraintSelector `json:"selector,omitempty"`

	// componentSelector is used to bind the resource constraint to components based on ComponentDefinition API.
	// +listType=map
	// +listMapKey=componentDefRef
	// +optional
	ComponentSelector []ComponentResourceConstraintSelector `json:"componentSelector,omitempty"`
}

type ClusterResourceConstraintSelector struct {
	// clusterDefRef is the name of the cluster definition.
	// +kubebuilder:validation:Required
	ClusterDefRef string `json:"clusterDefRef"`

	// selector is used to bind the resource constraint to components.
	// +listType=map
	// +listMapKey=componentDefRef
	// +kubebuilder:validation:Required
	Components []ComponentResourceConstraintSelector `json:"components"`
}

type ComponentResourceConstraintSelector struct {
	// In versions prior to KB 0.8.0, ComponentDefRef is the name of the component definition in the ClusterDefinition.
	// In KB 0.8.0 and later versions, ComponentDefRef is the name of ComponentDefinition.
	// +kubebuilder:validation:Required
	ComponentDefRef string `json:"componentDefRef"`

	// rules are the constraint rules that will be applied to the component.
	// +kubebuilder:validation:Required
	Rules []string `json:"rules"`
}

type ResourceConstraintRule struct {
	// The name of the constraint.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// The constraint for vcpu cores.
	// +kubebuilder:validation:Required
	CPU CPUConstraint `json:"cpu"`

	// The constraint for memory size.
	// +kubebuilder:validation:Required
	Memory MemoryConstraint `json:"memory"`

	// The constraint for storage size.
	// +optional
	Storage StorageConstraint `json:"storage"`
}

type CPUConstraint struct {
	// The maximum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range
	// must be multiple times of Step. It's useful to define a large number of valid values without defining them one by
	// one. Please see the documentation for Step for some examples.
	// If Slots is specified, Max, Min, and Step are ignored
	// +optional
	Max *resource.Quantity `json:"max,omitempty"`

	// The minimum count of vcpu cores, [Min, Max] defines a range for valid vcpu cores, and the value in this range
	// must be multiple times of Step. It's useful to define a large number of valid values without defining them one by
	// one. Please see the documentation for Step for some examples.
	// If Slots is specified, Max, Min, and Step are ignored
	// +optional
	Min *resource.Quantity `json:"min,omitempty"`

	// The minimum granularity of vcpu cores, [Min, Max] defines a range for valid vcpu cores and the value in this range must be
	// multiple times of Step.
	// For example:
	// 1. Min is 2, Max is 8, Step is 2, and the valid vcpu core is {2, 4, 6, 8}.
	// 2. Min is 0.5, Max is 2, Step is 0.5, and the valid vcpu core is {0.5, 1, 1.5, 2}.
	// +optional
	Step *resource.Quantity `json:"step,omitempty"`

	// The valid vcpu cores, it's useful if you want to define valid vcpu cores explicitly.
	// If Slots is specified, Max, Min, and Step are ignored
	// +optional
	Slots []resource.Quantity `json:"slots,omitempty"`
}

type MemoryConstraint struct {
	// The size of memory per vcpu core.
	// For example: 1Gi, 200Mi.
	// If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignore.
	// +optional
	SizePerCPU *resource.Quantity `json:"sizePerCPU,omitempty"`

	// The maximum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.
	// It is useful on GCP as the ratio between the CPU and memory may be a range.
	// If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.
	// Reference: https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types
	// +optional
	MaxPerCPU *resource.Quantity `json:"maxPerCPU,omitempty"`

	// The minimum size of memory per vcpu core, [MinPerCPU, MaxPerCPU] defines a range for valid memory size per vcpu core.
	// It is useful on GCP as the ratio between the CPU and memory may be a range.
	// If SizePerCPU is specified, MinPerCPU and MaxPerCPU are ignored.
	// Reference: https://cloud.google.com/compute/docs/general-purpose-machines#custom_machine_types
	// +optional
	MinPerCPU *resource.Quantity `json:"minPerCPU,omitempty"`
}

type StorageConstraint struct {
	// The minimum size of storage.
	// +kubebuilder:default="20Gi"
	// +optional
	Min *resource.Quantity `json:"min,omitempty"`

	// The maximum size of storage.
	// +kubebuilder:default="10Ti"
	// +optional
	Max *resource.Quantity `json:"max,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={kubeblocks,all},scope=Cluster,shortName=crc
// +kubebuilder:deprecatedversion:warning="Due to the lack of practical use cases, this API is deprecated from KB 0.9.0."

// ComponentResourceConstraint is the Schema for the componentresourceconstraints API
type ComponentResourceConstraint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ComponentResourceConstraintSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentResourceConstraintList contains a list of ComponentResourceConstraint
type ComponentResourceConstraintList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComponentResourceConstraint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComponentResourceConstraint{}, &ComponentResourceConstraintList{})
}

// ValidateCPU validates if the CPU meets the constraint
func (m *ResourceConstraintRule) ValidateCPU(cpu *resource.Quantity) bool {
	if cpu.IsZero() {
		return true
	}
	if m.CPU.Min != nil && m.CPU.Min.Cmp(*cpu) > 0 {
		return false
	}
	if m.CPU.Max != nil && m.CPU.Max.Cmp(*cpu) < 0 {
		return false
	}
	if m.CPU.Step != nil {
		result := inf.NewDec(1, 0).QuoExact(cpu.AsDec(), m.CPU.Step.AsDec())
		if result == nil {
			return false
		}
		// the quotient must be an integer
		if strings.Contains(strings.TrimRight(result.String(), ".0"), ".") {
			return false
		}
	}
	if m.CPU.Slots != nil && slices.Index(m.CPU.Slots, *cpu) < 0 {
		return false
	}
	return true
}

// ValidateMemory validates if the memory meets the constraint
func (m *ResourceConstraintRule) ValidateMemory(cpu *resource.Quantity, memory *resource.Quantity) bool {
	if memory.IsZero() {
		return true
	}

	var slots []resource.Quantity
	switch {
	case cpu != nil && !cpu.IsZero():
		slots = append(slots, *cpu)
	case len(m.CPU.Slots) > 0:
		slots = m.CPU.Slots
	default:
		slot := *m.CPU.Min
		for slot.Cmp(*m.CPU.Max) <= 0 {
			slots = append(slots, slot)
			slot = resource.MustParse(inf.NewDec(1, 0).Add(slot.AsDec(), m.CPU.Step.AsDec()).String())
		}
	}

	for _, slot := range slots {
		if m.Memory.SizePerCPU != nil && !m.Memory.SizePerCPU.IsZero() {
			match := inf.NewDec(1, 0).Mul(slot.AsDec(), m.Memory.SizePerCPU.AsDec()).Cmp(memory.AsDec()) == 0
			if match {
				return true
			}
		} else {
			maxMemory := inf.NewDec(1, 0).Mul(slot.AsDec(), m.Memory.MaxPerCPU.AsDec())
			minMemory := inf.NewDec(1, 0).Mul(slot.AsDec(), m.Memory.MinPerCPU.AsDec())
			if maxMemory.Cmp(memory.AsDec()) >= 0 && minMemory.Cmp(memory.AsDec()) <= 0 {
				return true
			}
		}
	}
	return false
}

// ValidateStorage validates if the storage meets the constraint
func (m *ResourceConstraintRule) ValidateStorage(storage *resource.Quantity) bool {
	if storage.IsZero() {
		return true
	}

	if m.Storage.Min != nil && m.Storage.Min.Cmp(*storage) > 0 {
		return false
	}
	if m.Storage.Max != nil && m.Storage.Max.Cmp(*storage) < 0 {
		return false
	}
	return true
}

// ValidateResources validates if the resources meets the constraint
func (m *ResourceConstraintRule) ValidateResources(r corev1.ResourceList) bool {
	if !m.ValidateCPU(r.Cpu()) {
		return false
	}

	if !m.ValidateMemory(r.Cpu(), r.Memory()) {
		return false
	}

	if !m.ValidateStorage(r.Storage()) {
		return false
	}

	return true
}

func (m *ResourceConstraintRule) CompleteResources(r corev1.ResourceList) corev1.ResourceList {
	if r.Cpu().IsZero() || !r.Memory().IsZero() {
		return corev1.ResourceList{corev1.ResourceCPU: *r.Cpu(), corev1.ResourceMemory: *r.Memory()}
	}

	var memory *inf.Dec
	if m.Memory.SizePerCPU != nil {
		memory = inf.NewDec(1, 0).Mul(r.Cpu().AsDec(), m.Memory.SizePerCPU.AsDec())
	} else {
		memory = inf.NewDec(1, 0).Mul(r.Cpu().AsDec(), m.Memory.MinPerCPU.AsDec())
	}
	return corev1.ResourceList{
		corev1.ResourceCPU:    *r.Cpu(),
		corev1.ResourceMemory: resource.MustParse(memory.String()),
	}
}

// GetMinimalResources gets the minimal resources meets the constraint
func (m *ResourceConstraintRule) GetMinimalResources() corev1.ResourceList {
	var (
		minCPU    resource.Quantity
		minMemory resource.Quantity
	)

	if len(m.CPU.Slots) == 0 && (m.CPU.Min == nil || m.CPU.Min.IsZero()) {
		return corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}
	}

	if len(m.CPU.Slots) > 0 {
		minCPU = m.CPU.Slots[0]
	}

	if minCPU.IsZero() || (m.CPU.Min != nil && minCPU.Cmp(*m.CPU.Min) > 0) {
		minCPU = *m.CPU.Min
	}

	var memory *inf.Dec
	if m.Memory.MinPerCPU != nil {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), m.Memory.MinPerCPU.AsDec())
	} else {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), m.Memory.SizePerCPU.AsDec())
	}
	minMemory = resource.MustParse(memory.String())
	return corev1.ResourceList{corev1.ResourceCPU: minCPU, corev1.ResourceMemory: minMemory}
}

// FindMatchingRules find all constraint rules that resource satisfies.
func (c *ComponentResourceConstraint) FindMatchingRules(
	clusterDefRef string,
	componentDefRef string,
	resources corev1.ResourceList) []ResourceConstraintRule {

	rules := c.FindRules(clusterDefRef, componentDefRef)
	var result []ResourceConstraintRule
	for _, rule := range rules {
		if rule.ValidateResources(resources) {
			result = append(result, rule)
		}
	}
	return result
}

// MatchClass checks if the class meets the constraint rules.
func (c *ComponentResourceConstraint) MatchClass(clusterDefRef, componentDefRef string, class *ComponentClass) bool {
	request := corev1.ResourceList{
		corev1.ResourceCPU:    class.CPU,
		corev1.ResourceMemory: class.Memory,
	}
	constraints := c.FindMatchingRules(clusterDefRef, componentDefRef, request)
	return len(constraints) > 0
}

// FindRules find all constraint rules that the component should conform to based on ClusterDefinition API.
// TODO(xingran): it will be deprecated in the future, use FindRulesWithCompDef instead.
func (c *ComponentResourceConstraint) FindRules(clusterDefRef, componentDefRef string) []ResourceConstraintRule {
	rules := make(map[string]bool)
	for _, selector := range c.Spec.Selector {
		if selector.ClusterDefRef != clusterDefRef {
			continue
		}
		for _, item := range selector.Components {
			if item.ComponentDefRef != componentDefRef {
				continue
			}
			for _, name := range item.Rules {
				rules[name] = true
			}
		}
	}

	var result []ResourceConstraintRule
	for _, rule := range c.Spec.Rules {
		if _, ok := rules[rule.Name]; !ok {
			continue
		}
		result = append(result, rule)
	}
	return result
}

// FindRulesWithCompDef find all constraint rules that the component should conform to based on ComponentDefinition API.
func (c *ComponentResourceConstraint) FindRulesWithCompDef(compDef string) []ResourceConstraintRule {
	rules := make(map[string]bool)

	for _, selector := range c.Spec.ComponentSelector {
		if selector.ComponentDefRef != compDef {
			continue
		}
		for _, name := range selector.Rules {
			rules[name] = true
		}
	}

	var result []ResourceConstraintRule
	for _, rule := range c.Spec.Rules {
		if _, ok := rules[rule.Name]; !ok {
			continue
		}
		result = append(result, rule)
	}
	return result
}
