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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type MutateFunc func(dest, expected *appsv1alpha1.ConfigurationItemDetail)

func MergeConfiguration(expected *appsv1alpha1.Configuration, existing *appsv1alpha1.Configuration, mutate MutateFunc) *appsv1alpha1.Configuration {
	fromList := func(items []appsv1alpha1.ConfigurationItemDetail) sets.Set[string] {
		sets := sets.New[string]()
		for _, item := range items {
			sets.Insert(item.Name)
		}
		return sets
	}

	// update cluster.spec.shardingSpecs[*].template.componentConfigItems.*
	updateConfigSpec := func(item appsv1alpha1.ConfigurationItemDetail) appsv1alpha1.ConfigurationItemDetail {
		if newItem := expected.Spec.GetConfigurationItem(item.Name); newItem != nil {
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

	newConfigItems := make([]appsv1alpha1.ConfigurationItemDetail, 0)
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
