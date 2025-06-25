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

package rollout

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type rolloutStatusTransformer struct{}

var _ graph.Transformer = &rolloutStatusTransformer{}

func (t *rolloutStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}

	rollout := transCtx.Rollout
	state, err := t.components(transCtx, rollout)
	if err != nil {
		return err
	}
	// TODO: sharding

	rollout.Status.ObservedGeneration = rollout.Generation
	rollout.Status.State = state

	// TODO: error message, conditions

	return nil
}

func (t *rolloutStatusTransformer) components(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout) (appsv1alpha1.RolloutState, error) {
	states := make([]appsv1alpha1.RolloutState, 0)
	for _, comp := range rollout.Spec.Components {
		state, err := t.component(transCtx, rollout, comp)
		if err != nil {
			return "", err
		}
		states = append(states, state)
	}

	var (
		hasError   = false
		hasRolling = false
		hasSucceed = false
		hasPending = false
		allSucceed = true
	)
	for _, state := range states {
		switch state {
		case appsv1alpha1.RollingRolloutState:
			hasRolling = true
			allSucceed = false
		case appsv1alpha1.ErrorRolloutState:
			hasError = true
			allSucceed = false
		case appsv1alpha1.SucceedRolloutState:
			hasSucceed = true
		case appsv1alpha1.PendingRolloutState:
			hasPending = true
			allSucceed = false
		default:
			allSucceed = false
		}
	}
	switch {
	case hasError:
		return appsv1alpha1.ErrorRolloutState, nil
	case hasRolling:
		return appsv1alpha1.RollingRolloutState, nil
	case allSucceed:
		return appsv1alpha1.SucceedRolloutState, nil
	case hasSucceed:
		return appsv1alpha1.RollingRolloutState, nil
	case hasPending:
		return appsv1alpha1.PendingRolloutState, nil
	default:
		return "", nil
	}
}

func (t *rolloutStatusTransformer) component(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	if comp.Strategy.Inplace != nil {
		return t.inplace(transCtx, rollout, comp)
	}
	if comp.Strategy.Replace != nil {
		return t.replace(transCtx, rollout, comp)
	}
	if comp.Strategy.Create != nil {
		return t.create(transCtx, rollout, comp)
	}
	return "", nil
}

func (t *rolloutStatusTransformer) inplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	spec := t.compSpec(transCtx, comp.Name)
	serviceVersion, compDef := serviceVersionNCompDef(rollout, comp, spec)
	if serviceVersion == spec.ServiceVersion && compDef == spec.ComponentDef {
		return appsv1alpha1.PendingRolloutState, nil
	}
	if checkClusterNCompRunning(transCtx, comp.Name) {
		return appsv1alpha1.SucceedRolloutState, nil
	}
	return appsv1alpha1.RollingRolloutState, nil
}

func (t *rolloutStatusTransformer) replace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	var rollingTpl *appsv1.InstanceTemplate
	spec := t.compSpec(transCtx, comp.Name)
	tplName := string(rollout.UID[:8])
	for i, tpl := range spec.Instances {
		if tpl.Name == tplName {
			rollingTpl = &spec.Instances[i]
		}
	}
	if rollingTpl == nil {
		return appsv1alpha1.PendingRolloutState, nil
	}

	pods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(rollout.Namespace),
		client.MatchingLabels(constant.GetCompLabels(rollout.Spec.ClusterName, comp.Name)),
	}
	if err := transCtx.Client.List(transCtx.Context, pods, listOpts...); err != nil {
		return "", err
	}

	allPodCnt := int32(len(pods.Items))
	newPodCnt := int32(generics.CountFunc(pods.Items, func(pod corev1.Pod) bool {
		if pod.Labels != nil {
			return pod.Labels[constant.KBAppInstanceTemplateLabelKey] == tplName
		}
		return false
	}))
	for i, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			if checkClusterNCompRunning(transCtx, comp.Name) {
				newReplicas, rolledOutReplicas := newPodCnt, newPodCnt-(allPodCnt-status.Replicas)
				if rolledOutReplicas == newReplicas {
					if status.RolledOutReplicas < rolledOutReplicas {
						rollout.Status.Components[i].LastScaleDownTimestamp = metav1.Now()
					}
				} else {
					if status.NewReplicas < newReplicas {
						rollout.Status.Components[i].LastScaleUpTimestamp = metav1.Now()
					}
				}
				rollout.Status.Components[i].NewReplicas = newReplicas
				rollout.Status.Components[i].RolledOutReplicas = rolledOutReplicas
			}
			break
		}
	}

	if !checkClusterNCompRunning(transCtx, comp.Name) || spec.Replicas != *rollingTpl.Replicas {
		return appsv1alpha1.RollingRolloutState, nil
	}
	if allPodCnt != spec.Replicas {
		return appsv1alpha1.RollingRolloutState, nil // scaling down
	}
	return appsv1alpha1.SucceedRolloutState, nil
}

func (t *rolloutStatusTransformer) create(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	// TODO: impl
	return "", createStrategyNotSupportedError
}

func (t *rolloutStatusTransformer) compSpec(transCtx *rolloutTransformContext, compName string) *appsv1.ClusterComponentSpec {
	// use the original cluster spec
	cluster := transCtx.ClusterOrig
	for i, comp := range cluster.Spec.ComponentSpecs {
		if comp.Name == compName {
			return &cluster.Spec.ComponentSpecs[i]
		}
	}
	return nil
}

func isRolloutSucceed(rollout *appsv1alpha1.Rollout) bool {
	return rollout.Status.State == appsv1alpha1.SucceedRolloutState
}
