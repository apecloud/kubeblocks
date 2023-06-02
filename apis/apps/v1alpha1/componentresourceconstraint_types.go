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
	"golang.org/x/exp/slices"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentResourceConstraintSpec defines the desired state of ComponentResourceConstraint
type ComponentResourceConstraintSpec struct {
	// Component resource constraints
	Constraints []ResourceConstraint `json:"constraints,omitempty"`
}

type ResourceConstraint struct {
	// The constraint for vcpu cores.
	// +kubebuilder:validation:Required
	CPU CPUConstraint `json:"cpu"`

	// The constraint for memory size.
	// +kubebuilder:validation:Required
	Memory MemoryConstraint `json:"memory"`
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

// +genclient
// +genclient:nonNamespaced
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={kubeblocks,all},scope=Cluster,shortName=crc

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

// ValidateCPU validate if the CPU matches the resource constraints
func (m ResourceConstraint) ValidateCPU(cpu resource.Quantity) bool {
	if m.CPU.Min != nil && m.CPU.Min.Cmp(cpu) > 0 {
		return false
	}
	if m.CPU.Max != nil && m.CPU.Max.Cmp(cpu) < 0 {
		return false
	}
	if m.CPU.Step != nil && inf.NewDec(1, 0).QuoExact(cpu.AsDec(), m.CPU.Step.AsDec()).Scale() != 0 {
		return false
	}
	if m.CPU.Slots != nil && slices.Index(m.CPU.Slots, cpu) < 0 {
		return false
	}
	return true
}

// ValidateMemory validate if the memory matches the resource constraints
func (m ResourceConstraint) ValidateMemory(cpu *resource.Quantity, memory *resource.Quantity) bool {
	if memory == nil {
		return true
	}

	// fast path if cpu is specified
	if cpu != nil && m.Memory.SizePerCPU != nil {
		return inf.NewDec(1, 0).Mul(cpu.AsDec(), m.Memory.SizePerCPU.AsDec()).Cmp(memory.AsDec()) == 0
	}

	if cpu != nil && m.Memory.MaxPerCPU != nil && m.Memory.MinPerCPU != nil {
		maxMemory := inf.NewDec(1, 0).Mul(cpu.AsDec(), m.Memory.MaxPerCPU.AsDec())
		minMemory := inf.NewDec(1, 0).Mul(cpu.AsDec(), m.Memory.MinPerCPU.AsDec())
		return maxMemory.Cmp(memory.AsDec()) >= 0 && minMemory.Cmp(memory.AsDec()) <= 0
	}

	// TODO slow path if cpu is not specified

	return true
}

// ValidateResourceRequirements validate if the resource matches the resource constraints
func (m ResourceConstraint) ValidateResourceRequirements(r *corev1.ResourceRequirements) bool {
	var (
		cpu    = r.Requests.Cpu()
		memory = r.Requests.Memory()
	)

	if cpu.IsZero() && memory.IsZero() {
		return true
	}

	if !m.ValidateCPU(*cpu) {
		return false
	}

	if !m.ValidateMemory(cpu, memory) {
		return false
	}

	return true
}

// FindMatchingConstraints find all constraints that resource matches
func (c *ComponentResourceConstraint) FindMatchingConstraints(r *corev1.ResourceRequirements) []ResourceConstraint {
	if c == nil {
		return nil
	}
	var constraints []ResourceConstraint
	for _, constraint := range c.Spec.Constraints {
		if constraint.ValidateResourceRequirements(r) {
			constraints = append(constraints, constraint)
		}
	}
	return constraints
}

func (c *ComponentResourceConstraint) MatchClass(class *ComponentClassInstance) bool {
	request := corev1.ResourceList{
		corev1.ResourceCPU:    class.CPU,
		corev1.ResourceMemory: class.Memory,
	}
	resource := &corev1.ResourceRequirements{
		Limits:   request,
		Requests: request,
	}
	constraints := c.FindMatchingConstraints(resource)
	return len(constraints) > 0
}
