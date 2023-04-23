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

package lifecycle

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// FillClassTransformer fill the class related info to cluster
type FillClassTransformer struct{}

func (r *FillClassTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if isClusterDeleting(*cluster) {
		return nil
	}
	return r.fillClass(transCtx)
}

func (r *FillClassTransformer) fillClass(transCtx *ClusterTransformContext) error {
	cluster := transCtx.Cluster
	clusterDefinition := transCtx.ClusterDef

	var (
		classDefinitionList appsv1alpha1.ComponentClassDefinitionList
	)

	ml := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefinition.Name},
	}
	if err := transCtx.Client.List(transCtx.Context, &classDefinitionList, ml...); err != nil {
		return err
	}
	compClasses, err := class.GetClasses(classDefinitionList)
	if err != nil {
		return err
	}

	var constraintList appsv1alpha1.ComponentResourceConstraintList
	if err = transCtx.Client.List(transCtx.Context, &constraintList); err != nil {
		return err
	}

	// TODO use this function to get matched resource constraints if class is not specified and component has no classes
	_ = func(comp appsv1alpha1.ClusterComponentSpec) *appsv1alpha1.ComponentClassInstance {
		var candidates []class.ConstraintWithName
		for _, item := range constraintList.Items {
			constraints := item.FindMatchingConstraints(&comp.Resources)
			for _, constraint := range constraints {
				candidates = append(candidates, class.ConstraintWithName{Name: item.Name, Constraint: constraint})
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		sort.Sort(class.ByConstraintList(candidates))
		candidate := candidates[0]
		cpu, memory := class.GetMinCPUAndMemory(candidate.Constraint)
		name := fmt.Sprintf("%s-%vc%vg", candidate.Name, cpu.AsDec().String(), memory.AsDec().String())
		cls := &appsv1alpha1.ComponentClassInstance{
			ComponentClass: appsv1alpha1.ComponentClass{
				Name:   name,
				CPU:    *cpu,
				Memory: *memory,
			},
		}
		return cls
	}

	matchComponentClass := func(comp appsv1alpha1.ClusterComponentSpec, classes map[string]*appsv1alpha1.ComponentClassInstance) *appsv1alpha1.ComponentClassInstance {
		filters := make(map[corev1.ResourceName]resource.Quantity)
		if !comp.Resources.Requests.Cpu().IsZero() {
			filters[corev1.ResourceCPU] = *comp.Resources.Requests.Cpu()
		}
		if !comp.Resources.Requests.Memory().IsZero() {
			filters[corev1.ResourceMemory] = *comp.Resources.Requests.Memory()
		}
		return class.ChooseComponentClasses(classes, filters)
	}

	for idx, comp := range cluster.Spec.ComponentSpecs {
		classes := compClasses[comp.ComponentDefRef]

		var cls *appsv1alpha1.ComponentClassInstance
		// TODO another case if len(constraintList.Items) > 0, use matchClassFamilies to find matching resource constraint:
		switch {
		case comp.ClassDefRef != nil && comp.ClassDefRef.Class != "":
			cls = classes[comp.ClassDefRef.Class]
			if cls == nil {
				return fmt.Errorf("unknown component class %s", comp.ClassDefRef.Class)
			}
		case classes != nil:
			cls = matchComponentClass(comp, classes)
			if cls == nil {
				return fmt.Errorf("can not find matching class for component %s", comp.Name)
			}
		}
		if cls == nil {
			// TODO reconsider handling policy for this case
			continue
		}
		comp.ClassDefRef = &appsv1alpha1.ClassDefRef{Class: cls.Name}
		requests := corev1.ResourceList{
			corev1.ResourceCPU:    cls.CPU,
			corev1.ResourceMemory: cls.Memory,
		}
		requests.DeepCopyInto(&comp.Resources.Requests)
		requests.DeepCopyInto(&comp.Resources.Limits)
		var volumes []appsv1alpha1.ClusterComponentVolumeClaimTemplate
		if len(comp.VolumeClaimTemplates) > 0 {
			volumes = comp.VolumeClaimTemplates
		} else {
			volumes = buildVolumeClaimByClass(cls)
		}
		comp.VolumeClaimTemplates = volumes
		cluster.Spec.ComponentSpecs[idx] = comp
	}
	return nil
}

func buildVolumeClaimByClass(cls *appsv1alpha1.ComponentClassInstance) []appsv1alpha1.ClusterComponentVolumeClaimTemplate {
	var volumes []appsv1alpha1.ClusterComponentVolumeClaimTemplate
	for _, volume := range cls.Volumes {
		volumes = append(volumes, appsv1alpha1.ClusterComponentVolumeClaimTemplate{
			Name: volume.Name,
			Spec: appsv1alpha1.PersistentVolumeClaimSpec{
				// TODO define access mode in class
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: volume.Size,
					},
				},
			},
		})
	}
	return volumes
}

var _ graph.Transformer = &FillClassTransformer{}
