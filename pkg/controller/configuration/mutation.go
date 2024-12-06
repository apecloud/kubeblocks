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

package configuration

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
		if newItem := intctrlutil.GetConfigTemplateItem(&expected.Spec, item.Name); newItem != nil {
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
