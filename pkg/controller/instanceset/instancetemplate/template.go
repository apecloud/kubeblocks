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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func BuildInstanceTemplateExts(itsExt *InstanceSetExt) []*InstanceTemplateExt {
	instanceTemplatesMap := itsExt.InstanceTemplates
	templates := make([]*InstanceTemplateExt, 0, len(instanceTemplatesMap))
	for templateName := range instanceTemplatesMap {
		tpl := instanceTemplatesMap[templateName]
		tplExt := buildInstanceTemplateExt(tpl, itsExt.InstanceSet)
		templates = append(templates, tplExt)
	}

	return templates
}

func buildInstanceTemplatesMap(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) map[string]*workloads.InstanceTemplate {
	rtn := make(map[string]*workloads.InstanceTemplate)
	l := BuildInstanceTemplates(its, instancesCompressed)
	for _, t := range l {
		rtn[t.Name] = t
	}
	return rtn
}

// BuildInstanceTemplates builds a complete instance template list,
// i.e. append a pseudo template (which equals to `.spec.template`)
// to the end of the list, to fill up the replica count.
// And also if there is any compressed template, add them too.
//
// It is not guaranteed that the returned list is sorted.
// It is assumed that its spec is valid, e.g. replicasInTemplates < totalReplica.
func BuildInstanceTemplates(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	var instanceTemplateList []*workloads.InstanceTemplate
	var replicasInTemplates int32
	instanceTemplates := getInstanceTemplates(its.Spec.Instances, instancesCompressed)
	for i := range instanceTemplates {
		instance := &instanceTemplates[i]
		if instance.Replicas == nil {
			instance.Replicas = ptr.To[int32](1)
		}
		instanceTemplateList = append(instanceTemplateList, instance)
		replicasInTemplates += *instance.Replicas
	}
	totalReplicas := *its.Spec.Replicas
	if replicasInTemplates < totalReplicas {
		replicas := totalReplicas - replicasInTemplates
		instance := &workloads.InstanceTemplate{Replicas: &replicas, Ordinals: its.Spec.DefaultTemplateOrdinals}
		instanceTemplateList = append(instanceTemplateList, instance)
	}

	return instanceTemplateList
}

func BuildInstanceSetExt(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*InstanceSetExt, error) {
	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return nil, err
	}

	instanceTemplateMap := buildInstanceTemplatesMap(its, instancesCompressed)

	return &InstanceSetExt{
		InstanceSet:       its,
		InstanceTemplates: instanceTemplateMap,
	}, nil
}
