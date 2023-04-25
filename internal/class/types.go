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

package class

import (
	"sort"

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

var _ sort.Interface = ByClassResource{}

type ByClassResource []*ComponentClassWithRef

func (b ByClassResource) Len() int {
	return len(b)
}

func (b ByClassResource) Less(i, j int) bool {
	if out := b[i].CPU.Cmp(b[j].CPU); out != 0 {
		return out < 0
	}

	if out := b[i].Memory.Cmp(b[j].Memory); out != 0 {
		return out < 0
	}

	return false
}

func (b ByClassResource) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

type ComponentClassWithRef struct {
	appsv1alpha1.ComponentClassInstance

	ClassDefRef appsv1alpha1.ClassDefRef
}
