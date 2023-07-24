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

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ sort.Interface = ByResourceList{}

type ByResourceList []corev1.ResourceList

func (b ByResourceList) Len() int {
	return len(b)
}

func (b ByResourceList) Less(i, j int) bool {
	switch b[i].Cpu().Cmp(*b[j].Cpu()) {
	case 1:
		return false
	case -1:
		return true
	}
	switch b[i].Memory().Cmp(*b[j].Memory()) {
	case 1:
		return false
	case -1:
		return true
	}
	return false
}

func (b ByResourceList) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

var _ sort.Interface = ByRuleList{}

type ByRuleList []appsv1alpha1.ResourceConstraintRule

func (m ByRuleList) Len() int {
	return len(m)
}

func (m ByRuleList) Less(i, j int) bool {
	var (
		resource1 = m[i].GetMinimalResources()
		resource2 = m[j].GetMinimalResources()
	)
	switch resource1.Cpu().Cmp(*resource2.Cpu()) {
	case 1:
		return false
	case -1:
		return true
	}
	switch resource1.Memory().Cmp(*resource2.Memory()) {
	case 1:
		return false
	case -1:
		return true
	}
	return false
}

func (m ByRuleList) Swap(i, j int) {
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
	appsv1alpha1.ComponentClass

	ClassDefRef appsv1alpha1.ClassDefRef
}
