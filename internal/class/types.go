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

package class

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func GetMinCPUAndMemory(model appsv1alpha1.ResourceConstraint) (*resource.Quantity, *resource.Quantity) {
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

type ConstraintWithName struct {
	Name       string
	Constraint appsv1alpha1.ResourceConstraint
}

var _ sort.Interface = ByConstraintList{}

type ByConstraintList []ConstraintWithName

func (m ByConstraintList) Len() int {
	return len(m)
}

func (m ByConstraintList) Less(i, j int) bool {
	cpu1, mem1 := GetMinCPUAndMemory(m[i].Constraint)
	cpu2, mem2 := GetMinCPUAndMemory(m[j].Constraint)
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

func (m ByConstraintList) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

var _ sort.Interface = ByClassCPUAndMemory{}

type ByClassCPUAndMemory []*appsv1alpha1.ComponentClassInstance

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
