/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"k8s.io/apimachinery/pkg/util/sets"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type MutateFunc func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail)

func MergeComponentParameter(expected *parametersv1alpha1.ComponentParameter, existing *parametersv1alpha1.ComponentParameter, mutate MutateFunc) *parametersv1alpha1.ComponentParameter {
	fromList := func(items []parametersv1alpha1.ConfigTemplateItemDetail) sets.Set[string] {
		itemSet := sets.New[string]()
		for _, item := range items {
			itemSet.Insert(item.Name)
		}
		return itemSet
	}

	// update cluster.spec.shardingSpecs[*].template.componentConfigItems.*
	updateConfigSpec := func(item parametersv1alpha1.ConfigTemplateItemDetail) parametersv1alpha1.ConfigTemplateItemDetail {
		if newItem := GetConfigTemplateItem(&expected.Spec, item.Name); newItem != nil {
			updated := item.DeepCopy()
			mutate(updated, newItem)
			return *updated
		}
		return item
	}

	oldSets := fromList(existing.Spec.ConfigItemDetails)
	newSets := fromList(expected.Spec.ConfigItemDetails)
	addSets := newSets.Difference(oldSets)
	delSets := oldSets.Difference(newSets)

	newConfigItems := make([]parametersv1alpha1.ConfigTemplateItemDetail, 0)
	for _, item := range existing.Spec.ConfigItemDetails {
		if !delSets.Has(item.Name) {
			newConfigItems = append(newConfigItems, updateConfigSpec(item))
		}
	}
	for _, item := range expected.Spec.ConfigItemDetails {
		if addSets.Has(item.Name) {
			newConfigItems = append(newConfigItems, item)
		}
	}

	updated := existing.DeepCopy()
	updated.SetLabels(intctrlutil.MergeMetadataMaps(expected.GetLabels(), updated.GetLabels()))
	if len(expected.GetOwnerReferences()) != 0 {
		updated.SetOwnerReferences(expected.GetOwnerReferences())
	}
	updated.Spec.ConfigItemDetails = newConfigItems
	return updated
}
