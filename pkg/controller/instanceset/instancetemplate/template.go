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

package instancetemplate

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var (
	reader *zstd.Decoder
	writer *zstd.Encoder
)

func init() {
	var err error
	reader, err = zstd.NewReader(nil)
	runtime.Must(err)
	writer, err = zstd.NewWriter(nil)
	runtime.Must(err)
}

// GenerateTemplateName2OrdinalMap returns a map from template name to sorted ordinals
// it rely on the instanceset's status to generate desired pod names
// it may not be updated, but it should converge eventually
//
// template ordianls are assumed to be valid at this time
func GenerateTemplateName2OrdinalMap(its *workloads.InstanceSet) (map[string]sets.Set[int32], error) {
	allOrdinalSet := sets.New[int32]()
	template2OrdinalSetMap := map[string]sets.Set[int32]{}
	ordinalToTemplateMap := map[int32]string{}
	// FIXME: take compressed templates into account
	instanceTemplateList := buildInstanceTemplates(its, nil)

	for _, t := range instanceTemplateList {
		template2OrdinalSetMap[t.Name] = sets.New[int32]()
	}
	for templateName, ordinals := range its.Status.CurrentInstances {
		template2OrdinalSetMap[templateName].Insert(ordinals...)
		for _, ordinal := range ordinals {
			allOrdinalSet.Insert(ordinal)
			ordinalToTemplateMap[ordinal] = templateName
		}
	}

	// 1. handle those who have ordinals specified
	for _, instanceTemplate := range instanceTemplateList {
		currentOrdinalSet := template2OrdinalSetMap[instanceTemplate.Name]
		desiredOrdinalSet := ConvertOrdinalsToSet(instanceTemplate.Ordinals)
		if len(desiredOrdinalSet) == 0 {
			continue
		}
		toDelete := currentOrdinalSet.Difference(desiredOrdinalSet)
		toCreate := desiredOrdinalSet.Difference(currentOrdinalSet)
		for _, ordinal := range toDelete.UnsortedList() {
			allOrdinalSet.Delete(ordinal)
			template2OrdinalSetMap[ordinalToTemplateMap[ordinal]].Delete(ordinal)
		}
		for _, ordinal := range toCreate.UnsortedList() {
			if templateName, ok := ordinalToTemplateMap[ordinal]; ok {
				// if the ordinal is already in the current instance, replace it with the new one
				template2OrdinalSetMap[templateName].Delete(ordinal)
			}
			template2OrdinalSetMap[instanceTemplate.Name].Insert(ordinal)
			allOrdinalSet.Insert(ordinal)
		}
	}

	offlineOrdinals := sets.New[int32]()
	for _, instance := range its.Spec.OfflineInstances {
		ordinal, err := getOrdinal(instance)
		if err != nil {
			return nil, err
		}
		offlineOrdinals.Insert(ordinal)
	}
	// 2. handle those who have decreased replicas
	for _, instanceTemplate := range instanceTemplateList {
		currentOrdinals := template2OrdinalSetMap[instanceTemplate.Name]
		if toOffline := currentOrdinals.Intersection(offlineOrdinals); toOffline.Len() > 0 {
			for ordinal := range toOffline {
				allOrdinalSet.Delete(ordinal)
				template2OrdinalSetMap[instanceTemplate.Name].Delete(ordinal)
			}
		}
		if int(*instanceTemplate.Replicas) < len(currentOrdinals) {
			// delete in the name set from high to low
			l := convertOrdinalSetToSortedList(currentOrdinals)
			for i := len(currentOrdinals) - 1; i >= int(*instanceTemplate.Replicas); i-- {
				allOrdinalSet.Delete(l[i])
				template2OrdinalSetMap[instanceTemplate.Name].Delete(l[i])
			}
		}
	}

	// 3. handle those who have increased replicas
	var cur int32 = 0
	for _, instanceTemplate := range instanceTemplateList {
		currentOrdinals := template2OrdinalSetMap[instanceTemplate.Name]
		if int(*instanceTemplate.Replicas) > len(currentOrdinals) {
			for i := len(currentOrdinals); i < int(*instanceTemplate.Replicas); i++ {
				// find the next available ordinal
				for {
					if !allOrdinalSet.Has(cur) && !offlineOrdinals.Has(cur) {
						allOrdinalSet.Insert(cur)
						template2OrdinalSetMap[instanceTemplate.Name].Insert(cur)
						break
					}
					cur++
				}
			}
		}
	}

	return template2OrdinalSetMap, nil
}

func GenerateAllInstanceNames(its *workloads.InstanceSet) ([]string, error) {
	template2OrdinalSetMap, err := GenerateTemplateName2OrdinalMap(its)
	if err != nil {
		return nil, err
	}
	allOrdinalSet := sets.New[int32]()
	for _, ordinalSet := range template2OrdinalSetMap {
		allOrdinalSet = allOrdinalSet.Union(ordinalSet)
	}
	instanceNames := make([]string, 0, len(allOrdinalSet))
	allOrdinalList := convertOrdinalSetToSortedList(allOrdinalSet)
	for _, ordinal := range allOrdinalList {
		instanceNames = append(instanceNames, fmt.Sprintf("%v-%v", its.Name, ordinal))
	}
	return instanceNames, nil
}

// build a complate instance template list.
// That is to append a pseudo template (which equals to `.spec.template`)
// to the end of the list, to fill up the replica count.
// And also if there is any compressed template, add them too.
func buildInstanceTemplates(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	var instanceTemplateList []*workloads.InstanceTemplate
	var replicasInTemplates int32
	instanceTemplates := getInstanceTemplates(its.Spec.Instances, instancesCompressed)
	for i := range instanceTemplates {
		instance := &instanceTemplates[i]
		replicas := int32(1)
		if instance.Replicas != nil {
			replicas = *instance.Replicas
		}
		instanceTemplateList = append(instanceTemplateList, instance)
		replicasInTemplates += replicas
	}
	totalReplicas := *its.Spec.Replicas
	if replicasInTemplates < totalReplicas {
		replicas := totalReplicas - replicasInTemplates
		instance := &workloads.InstanceTemplate{Replicas: &replicas, Ordinals: its.Spec.DefaultTemplateOrdinals}
		// FIXME: is it necessary to let the default template be the first one?
		t := []*workloads.InstanceTemplate{instance}
		instanceTemplateList = append(t, instanceTemplateList...)
	}
	// FIXME: what about replicasInTemplates > totalReplica?

	return instanceTemplateList
}

func buildInstanceSetExt(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*instanceSetExt, error) {
	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return nil, err
	}

	instanceTemplateList := buildInstanceTemplates(its, instancesCompressed)

	return &instanceSetExt{
		its:               its,
		instanceTemplates: instanceTemplateList,
	}, nil
}

// PodsToCurrentInstances returns instanceset's .status.currentInstances
func PodsToCurrentInstances(pods []corev1.Pod, its *workloads.InstanceSet) (workloads.CurrentInstances, error) {
	currentInstances := make(workloads.CurrentInstances)
	for _, pod := range pods {
		templateName, ok := pod.Labels[TemplateNameLabelKey]
		if !ok {
			return nil, fmt.Errorf("unknown pod %v", klog.KObj(&pod))
		}
		ordinal, err := getOrdinal(pod.Name)
		if err != nil {
			return nil, err
		}
		currentInstances[templateName] = append(currentInstances[templateName], ordinal)
	}
	return currentInstances, nil
}
