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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// fixClusterLabelsTransformer fill the class related info to cluster
type fillClass struct {
	cc  clusterRefResources
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (r *fillClass) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	cluster, _ := rootVertex.obj.(*appsv1alpha1.Cluster)
	return r.fillClass(r.ctx, cluster, r.cc.cd)
}

func (r *fillClass) fillClass(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster, clusterDefinition appsv1alpha1.ClusterDefinition) error {
	var (
		classDefinitionList appsv1alpha1.ComponentClassDefinitionList
	)

	ml := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefinition.Name},
	}
	if err := r.cli.List(reqCtx.Ctx, &classDefinitionList, ml...); err != nil {
		return err
	}
	compClasses, err := class.GetClasses(classDefinitionList)
	if err != nil {
		return err
	}

	var constraintList appsv1alpha1.ComponentResourceConstraintList
	if err = r.cli.List(reqCtx.Ctx, &constraintList); err != nil {
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
		comp.ClassDefRef.Name = &cls.Name
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
		volume := appsv1alpha1.ClusterComponentVolumeClaimTemplate{
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
		}
		volumes = append(volumes, volume)
	}
	return volumes
}
