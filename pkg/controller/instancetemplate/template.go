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
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func BuildInstanceSetExt(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (*InstanceSetExt, error) {
	instancesCompressed, err := findTemplateObject(its, tree)
	if err != nil {
		return nil, err
	}

	ext := &InstanceSetExt{
		InstanceSet:       its,
		InstanceTemplates: make(map[string]*workloads.InstanceTemplate),
	}
	templates := buildInstanceTemplates(its, instancesCompressed)
	for i, tpl := range templates {
		ext.InstanceTemplates[tpl.Name] = templates[i]
	}
	return ext, nil
}

func BuildInstanceTemplateExt(itsExt *InstanceSetExt) []*InstanceTemplateExt {
	instanceTemplatesMap := itsExt.InstanceTemplates
	templates := make([]*InstanceTemplateExt, 0, len(instanceTemplatesMap))
	for templateName := range instanceTemplatesMap {
		tpl := instanceTemplatesMap[templateName]
		tplExt := buildInstanceTemplateExt(tpl, itsExt.InstanceSet)
		templates = append(templates, tplExt)
	}

	return templates
}

// buildInstanceTemplates builds a complete instance template list,
// i.e. append a pseudo template (which equals to `.spec.template`)
// to the end of the list, to fill up the replica count.
// And also if there is any compressed template, add them too.
//
// It is not guaranteed that the returned list is sorted.
// It is assumed that its spec is valid, e.g. replicasInTemplates < totalReplica.
func buildInstanceTemplates(its *workloads.InstanceSet, instancesCompressed *corev1.ConfigMap) []*workloads.InstanceTemplate {
	var instanceTemplateList []*workloads.InstanceTemplate
	var replicasInTemplates int32
	instanceTemplates := getInstanceTemplates(its.Spec.Instances, instancesCompressed)
	for i := range instanceTemplates {
		tpl := &instanceTemplates[i]
		if tpl.Replicas == nil {
			tpl.Replicas = ptr.To[int32](1)
		}
		instanceTemplateList = append(instanceTemplateList, tpl)
		replicasInTemplates += *tpl.Replicas
	}
	totalReplicas := *its.Spec.Replicas
	if replicasInTemplates < totalReplicas {
		replicas := totalReplicas - replicasInTemplates
		instance := &workloads.InstanceTemplate{Replicas: &replicas, Ordinals: its.Spec.Ordinals}
		instanceTemplateList = append(instanceTemplateList, instance)
	}

	return instanceTemplateList
}

func buildInstanceTemplateExt(template *workloads.InstanceTemplate, its *workloads.InstanceSet) *InstanceTemplateExt {
	var claims []corev1.PersistentVolumeClaim
	for _, t := range its.Spec.VolumeClaimTemplates {
		claims = append(claims, *t.DeepCopy())
	}
	templateExt := &InstanceTemplateExt{
		Name:                 template.Name,
		PodTemplateSpec:      *its.Spec.Template.DeepCopy(),
		VolumeClaimTemplates: claims,
	}

	replicas := int32(1)
	if template.Replicas != nil {
		replicas = *template.Replicas
	}
	templateExt.Replicas = replicas

	if template.SchedulingPolicy != nil && template.SchedulingPolicy.NodeName != "" {
		templateExt.Spec.NodeName = template.SchedulingPolicy.NodeName
	}
	mergeMap(&template.Annotations, &templateExt.Annotations)
	mergeMap(&template.Labels, &templateExt.Labels)
	if template.SchedulingPolicy != nil {
		mergeMap(&template.SchedulingPolicy.NodeSelector, &templateExt.Spec.NodeSelector)
	}
	if len(templateExt.Spec.Containers) > 0 {
		if template.Resources != nil {
			src := template.Resources
			dst := &templateExt.Spec.Containers[0].Resources
			mergeCPUNMemory(&src.Limits, &dst.Limits)
			mergeCPUNMemory(&src.Requests, &dst.Requests)
		}
		if template.Env != nil {
			intctrlutil.MergeList(&template.Env, &templateExt.Spec.Containers[0].Env,
				func(item corev1.EnvVar) func(corev1.EnvVar) bool {
					return func(env corev1.EnvVar) bool {
						return env.Name == item.Name
					}
				})
		}
	}

	updateImage := func(containers []corev1.Container, images map[string]string) {
		for i, c := range containers {
			if image, ok := images[c.Name]; ok {
				containers[i].Image = image
			}
		}
	}
	updateImage(templateExt.Spec.InitContainers, template.Images)
	updateImage(templateExt.Spec.Containers, template.Images)

	scheduling.ApplySchedulingPolicyToPodSpec(&templateExt.Spec, template.SchedulingPolicy)

	// override by instance template
	for _, tpl1 := range template.VolumeClaimTemplates {
		found := false
		for i, tpl2 := range templateExt.VolumeClaimTemplates {
			if tpl1.Name == tpl2.Name {
				templateExt.VolumeClaimTemplates[i] = *tpl1.DeepCopy()
				found = true
				break
			}
		}
		if !found {
			templateExt.VolumeClaimTemplates = append(templateExt.VolumeClaimTemplates, *tpl1.DeepCopy())
		}
	}

	return templateExt
}

func mergeCPUNMemory(s, d *corev1.ResourceList) {
	if s == nil || *s == nil || d == nil {
		return
	}
	for _, k := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if v, ok := (*s)[k]; ok {
			if *d == nil {
				*d = make(corev1.ResourceList)
			}
			(*d)[k] = v
		}
	}
}

func mergeMap[K comparable, V any](src, dst *map[K]V) {
	if len(*src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[K]V)
	}
	for k, v := range *src {
		(*dst)[k] = v
	}
}
