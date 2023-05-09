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

package lifecycle

import (
	"fmt"
	"sort"

	"gopkg.in/inf.v0"
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
	clsMgr, err := class.NewManager(classDefinitionList)
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

	for idx, comp := range cluster.Spec.ComponentSpecs {
		cls, err := clsMgr.ChooseClass(&comp)
		if err != nil {
			return err
		}
		if cls == nil {
			// TODO reconsider handling policy for this case
			continue
		}
		comp.ClassDefRef = &cls.ClassDefRef

		// update tenancy affinity
		affinity := cluster.Spec.Affinity
		if comp.Affinity != nil {
			affinity = comp.Affinity
		}
		policies := cluster.Spec.ResourceAllocationPolicies
		if comp.ResourceAllocationPolicies != nil {
			policies = comp.ResourceAllocationPolicies
		}
		if affinity.Tenancy == appsv1alpha1.DedicatedNode {
			comp.Resources, err = getDedicatedComponentResources(policies, cls)
		} else {
			comp.Resources, err = getSharedComponentResources(policies, cls)
		}
		if err != nil {
			return err
		}

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

func buildVolumeClaimByClass(cls *class.ComponentClassWithRef) []appsv1alpha1.ClusterComponentVolumeClaimTemplate {
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

type ResourceSummary struct {
	CPUTotal resource.Quantity
	CPULimit resource.Quantity
	MemTotal resource.Quantity
	MemLimit resource.Quantity
}

func getDedicatedComponentResources(policies appsv1alpha1.ResourceAllocationPolicies, cls *class.ComponentClassWithRef) (corev1.ResourceRequirements, error) {
	resources := getDedicatedResourceTotalAndLimit(policies, cls)
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cls.CPU,
			corev1.ResourceMemory: cls.Memory,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resources.CPULimit,
			corev1.ResourceMemory: resources.MemLimit,
		},
	}, nil
}

func getDedicatedResourceTotalAndLimit(policies appsv1alpha1.ResourceAllocationPolicies, cls *class.ComponentClassWithRef) ResourceSummary {
	var (
		// total is used to choose vm, limit is used to set the resource limits
		cpuTotal = cls.CPU
		memTotal = cls.Memory
		cpuLimit = cls.CPU
		memLimit = cls.Memory
	)

	if policy, ok := policies[corev1.ResourceCPU]; ok {
		if policy.OverCommitRatio != 0 {
			overSize := inf.NewDec(1, 0).Mul(cls.CPU.AsDec(), inf.NewDec(int64(policy.OverCommitRatio), 2))
			cpuTotal = resource.MustParse(inf.NewDec(1, 0).Add(cls.CPU.AsDec(), overSize).String())
		} else if !policy.OverCommitSize.IsZero() {
			cpuTotal = resource.MustParse(cls.CPU.String())
			cpuTotal.Add(policy.OverCommitSize)
		}

		if policy.DedicatedReservedRatio != nil {
			cpuLimit = resource.MustParse(inf.NewDec(1, 0).Mul(cpuTotal.AsDec(), inf.NewDec(int64(*policy.DedicatedReservedRatio), 2)).String())
		} else if !policy.DedicatedReservedSize.IsZero() {
			cpuLimit = policy.DedicatedReservedSize
		} else {
			cpuLimit = cpuTotal
		}
	}

	if policy, ok := policies[corev1.ResourceMemory]; ok {
		if policy.OverCommitRatio != 0 {
			overSize := inf.NewDec(1, 0).Mul(cls.Memory.AsDec(), inf.NewDec(int64(policy.OverCommitRatio), 2))
			memTotal = resource.MustParse(inf.NewDec(1, 0).Add(cls.Memory.AsDec(), overSize).String())
		} else if !policy.OverCommitSize.IsZero() {
			memTotal = resource.MustParse(cls.Memory.String())
			memTotal.Add(policy.OverCommitSize)
		}

		if policy.DedicatedReservedRatio != nil {
			memLimit = resource.MustParse(inf.NewDec(1, 0).Mul(memTotal.AsDec(), inf.NewDec(int64(*policy.DedicatedReservedRatio), 2)).String())
		} else if !policy.DedicatedReservedSize.IsZero() {
			memLimit = policy.DedicatedReservedSize
		} else {
			memLimit = memTotal
		}
	}
	return ResourceSummary{CPUTotal: cpuTotal, CPULimit: cpuLimit, MemTotal: memTotal, MemLimit: memLimit}
}

func getSharedComponentResources(policies appsv1alpha1.ResourceAllocationPolicies, cls *class.ComponentClassWithRef) (corev1.ResourceRequirements, error) {
	var (
		cpuLimit = cls.CPU
		memLimit = cls.Memory
	)

	if policy, ok := policies[corev1.ResourceCPU]; ok {
		if policy.OverCommitRatio != 0 {
			overSize := inf.NewDec(1, 0).Mul(cls.CPU.AsDec(), inf.NewDec(int64(policy.OverCommitRatio), 2))
			cpuLimit = resource.MustParse(inf.NewDec(1, 0).Add(cls.CPU.AsDec(), overSize).String())
		} else if !policy.OverCommitSize.IsZero() {
			cpuLimit = resource.MustParse(cls.CPU.String())
			cpuLimit.Add(policy.OverCommitSize)
		}
	}

	if policy, ok := policies[corev1.ResourceMemory]; ok {
		if policy.OverCommitRatio != 0 {
			overSize := inf.NewDec(1, 0).Mul(cls.Memory.AsDec(), inf.NewDec(int64(policy.OverCommitRatio), 2))
			memLimit = resource.MustParse(inf.NewDec(1, 0).Add(cls.Memory.AsDec(), overSize).String())
		} else if !policy.OverCommitSize.IsZero() {
			memLimit = resource.MustParse(cls.Memory.String())
			memLimit.Add(policy.OverCommitSize)
		}
	}

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cls.CPU,
			corev1.ResourceMemory: cls.Memory,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memLimit,
		},
	}
	return resources, nil
}

var _ graph.Transformer = &FillClassTransformer{}
