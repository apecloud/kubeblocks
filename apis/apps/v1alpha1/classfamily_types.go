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
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/inf.v0"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClassFamilySpec defines the desired state of ClassFamily
type ClassFamilySpec struct {
	// Class family models, generally, a model is a static memory/cpu ratio or a range.
	Models []ClassFamilyModel `json:"models,omitempty"`
}

type ClassFamilyModel struct {
	// The constraint for CPU cores
	// +kubebuilder:validation:Required
	CPU CPUConstraint `json:"cpu,omitempty"`

	// The constraint for memory size
	// +kubebuilder:validation:Required
	Memory MemoryConstraint `json:"memory,omitempty"`
}

type CPUConstraint struct {
	// The maximum count of vcpu cores.
	// +optional
	Max *resource.Quantity `json:"max,omitempty"`

	// The minimum count of vcpu cores.
	// +optional
	Min *resource.Quantity `json:"min,omitempty"`

	// The minimum granularity of vcpu cores.
	// +optional
	Step *resource.Quantity `json:"step,omitempty"`

	// The available vcpu cores,
	// +optional
	Slots []resource.Quantity `json:"slots,omitempty"`
}

type MemoryConstraint struct {
	// The size of memory per vcpu
	// +optional
	SizePerCPU *resource.Quantity `json:"sizePerCPU,omitempty"`

	// The maximum memory per vcpu
	// +optional
	MaxPerCPU *resource.Quantity `json:"maxPerCPU,omitempty"`

	// The minimum memory per vcpu
	// +optional
	MinPerCPU *resource.Quantity `json:"minPerCPU,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories={kubeblocks,all},scope=Cluster,shortName=cf

// ClassFamily is the Schema for the classfamilies API
type ClassFamily struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClassFamilySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ClassFamilyList contains a list of ClassFamily
type ClassFamilyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClassFamily `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClassFamily{}, &ClassFamilyList{})
}

func (m *ClassFamilyModel) ValidateCPU(cpu resource.Quantity) bool {
	if m.CPU.Min != nil && m.CPU.Min.Cmp(cpu) > 0 {
		return false
	}
	if m.CPU.Max != nil && m.CPU.Max.Cmp(cpu) < 0 {
		return false
	}
	if m.CPU.Slots != nil && slices.Index(m.CPU.Slots, cpu) < 0 {
		return false
	}
	return true
}

func (m *ClassFamilyModel) ValidateMemory(cpu *resource.Quantity, memory *resource.Quantity) bool {
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

func (m *ClassFamilyModel) ValidateResourceRequirements(r *corev1.ResourceRequirements) bool {
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

func (c *ClassFamily) FindMatchingModels(r *corev1.ResourceRequirements) []ClassFamilyModel {
	var models []ClassFamilyModel
	for _, model := range c.Spec.Models {
		if model.ValidateResourceRequirements(r) {
			models = append(models, model)
		}
	}
	return models
}

func GetMinCPUAndMemory(model ClassFamilyModel) (*resource.Quantity, *resource.Quantity) {
	var (
		minCPU    resource.Quantity
		minMemory resource.Quantity
	)

	if len(model.CPU.Slots) > 0 {
		minCPU = model.CPU.Slots[0]
	}

	if model.CPU.Min != nil && minCPU.Cmp(*model.CPU.Min) < 0 {
		minCPU = *model.CPU.Min
	}
	var memory *inf.Dec
	if model.Memory.MinPerCPU != nil {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), model.Memory.MinPerCPU.AsDec())
	} else {
		memory = inf.NewDec(1, 0).Mul(minCPU.AsDec(), model.Memory.SizePerCPU.AsDec())
	}
	minMemory = resource.MustParse(memory.String())
	return &minCPU, &minMemory
}

type ClassModelWithFamilyName struct {
	Family string
	Model  ClassFamilyModel
}

type ByModelList []ClassModelWithFamilyName

func (m ByModelList) Len() int {
	return len(m)
}

func (m ByModelList) Less(i, j int) bool {
	cpu1, mem1 := GetMinCPUAndMemory(m[i].Model)
	cpu2, mem2 := GetMinCPUAndMemory(m[j].Model)
	switch cpu1.Cmp(*cpu2) {
	case 1:
		return false
	case -1:
		return true
	}
	switch mem1.Cmp(*mem2) {
	case 1:
		return false
	case -1:
		return true
	}
	return false
}

func (m ByModelList) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

type ComponentClass struct {
	Name    string            `json:"name,omitempty"`
	CPU     resource.Quantity `json:"cpu,omitempty"`
	Memory  resource.Quantity `json:"memory,omitempty"`
	Storage []*Disk           `json:"storage,omitempty"`
	Family  string            `json:"-"`
}

var _ sort.Interface = ByClassCPUAndMemory{}

type ByClassCPUAndMemory []*ComponentClass

func (b ByClassCPUAndMemory) Len() int {
	return len(b)
}

func (b ByClassCPUAndMemory) Less(i, j int) bool {
	if out := b[i].CPU.Cmp(b[j].CPU); out != 0 {
		return out < 0
	}

	if out := b[i].Memory.Cmp(b[j].Memory); out != 0 {
		return out < 0
	}

	return false
}

func (b ByClassCPUAndMemory) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type Filters map[string]resource.Quantity

func (f Filters) String() string {
	var result []string
	for k, v := range f {
		result = append(result, fmt.Sprintf("%s=%v", k, v.Value()))
	}
	return strings.Join(result, ",")
}

type Disk struct {
	Name  string            `json:"name,omitempty"`
	Size  resource.Quantity `json:"size,omitempty"`
	Class string            `json:"class,omitempty"`
}

func (d Disk) String() string {
	return fmt.Sprintf("%s=%s", d.Name, d.Size.String())
}

type ProviderComponentClassDef struct {
	Provider string   `json:"provider,omitempty"`
	Args     []string `json:"args,omitempty"`
}

type DiskDef struct {
	Name  string `json:"name,omitempty"`
	Size  string `json:"size,omitempty"`
	Class string `json:"class,omitempty"`
}

type ComponentClassDef struct {
	Name     string                      `json:"name,omitempty"`
	CPU      string                      `json:"cpu,omitempty"`
	Memory   string                      `json:"memory,omitempty"`
	Storage  []DiskDef                   `json:"storage,omitempty"`
	Args     []string                    `json:"args,omitempty"`
	Variants []ProviderComponentClassDef `json:"variants,omitempty"`
}

type ComponentClassSeriesDef struct {
	Name    string              `json:"name,omitempty"`
	Classes []ComponentClassDef `json:"classes,omitempty"`
}

type ComponentClassFamilyDef struct {
	Family   string                    `json:"family"`
	Template string                    `json:"template,omitempty"`
	Vars     []string                  `json:"vars,omitempty"`
	Series   []ComponentClassSeriesDef `json:"series,omitempty"`
}
