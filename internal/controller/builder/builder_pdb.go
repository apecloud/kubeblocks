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

package builder

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PDBBuilder struct {
	BaseBuilder[policyv1.PodDisruptionBudget, *policyv1.PodDisruptionBudget, PDBBuilder]
}

func NewPDBBuilder(namespace, name string) *PDBBuilder {
	builder := &PDBBuilder{}
	builder.init(namespace, name, &policyv1.PodDisruptionBudget{}, builder)
	return builder
}

func (builder *PDBBuilder) SetMinAvailable(minAvailable intstr.IntOrString) *PDBBuilder {
	builder.get().Spec.MinAvailable = &minAvailable
	return builder
}

func (builder *PDBBuilder) AddSelector(key, value string) *PDBBuilder {
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{},
		}
	}
	selector.MatchLabels[key] = value
	builder.get().Spec.Selector = selector
	return builder
}

func (builder *PDBBuilder) AddSelectors(keyValues ...string) *PDBBuilder {
	return builder.AddSelectorsInMap(WithMap(keyValues...))
}

func (builder *PDBBuilder) AddSelectorsInMap(keyValues map[string]string) *PDBBuilder {
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{},
		}
	}
	for k, v := range keyValues {
		selector.MatchLabels[k] = v
	}
	builder.get().Spec.Selector = selector
	return builder
}
