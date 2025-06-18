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
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type rolloutCreateTransformer struct{}

var _ graph.Transformer = &rolloutCreateTransformer{}

func (t *rolloutCreateTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}

	return t.rollout(transCtx, dag)
}

func (t *rolloutCreateTransformer) rollout(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	if err := t.components(transCtx, dag); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutCreateTransformer) components(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	var delayedError error
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Create != nil {
			if err := t.component(transCtx, rollout, comp); err != nil {
				if controllerutil.IsDelayedRequeueError(err) {
					if delayedError == nil {
						delayedError = err
					}
					continue
				}
				return err
			}
		}
	}
	return delayedError
}

func (t *rolloutCreateTransformer) component(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	if spec == nil {
		return fmt.Errorf("the component %s is not found in cluster", comp.Name)
	}

	replicas, targetReplicas, err := t.replicas(rollout, comp, spec)
	if err != nil {
		return err
	}

	if (replicas + targetReplicas) > spec.Replicas {
		return t.rolling(transCtx, comp, spec, replicas, targetReplicas)
	}

	return t.promote(transCtx, comp, spec, replicas, targetReplicas)
}

func (t *rolloutCreateTransformer) replicas(rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (int32, int32, error) {
	// the original replicas
	replicas := spec.Replicas
	for _, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			replicas = status.Replicas
			break
		}
	}

	// the target replicas
	target, err := func() (int32, error) {
		if comp.Replicas != nil {
			replicas, err := intstr.GetScaledValueFromIntOrPercent(comp.Replicas, int(spec.Replicas), false)
			if err != nil {
				return 0, errors.Wrapf(err, "failed to get scaled value for replicas of component %s", comp.Name)
			}
			return int32(replicas), nil
		}
		return 0, nil
	}()
	if err != nil {
		return 0, 0, err
	}
	if target < 0 {
		return 0, 0, errors.Errorf("invalid target %d for component %s", target, comp.Name)
	}

	return replicas, target, nil
}

func (t *rolloutCreateTransformer) rolling(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas, targetReplicas int32) error {
	if (replicas + targetReplicas) == spec.Replicas {
		return nil
	}

	if !t.status(transCtx, comp) {
		return controllerutil.NewDelayedRequeueError(notReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	tpl, err := t.instanceTemplate(transCtx, comp, spec)
	if err != nil {
		return err
	}
	spec.Replicas += targetReplicas
	tpl.Replicas = ptr.To(targetReplicas)

	return nil
}

func (t *rolloutCreateTransformer) status(transCtx *rolloutTransformContext, comp appsv1alpha1.RolloutComponent) bool {
	cluster := transCtx.Cluster
	compStatus := cluster.Status.Components[comp.Name]
	if cluster.Generation != cluster.Status.ObservedGeneration || compStatus.Phase != appsv1.RunningComponentPhase {
		return false
	}
	compObj, ok := transCtx.Components[comp.Name]
	if !ok || compObj == nil {
		return false
	}
	return compObj.Generation == compObj.Status.ObservedGeneration && compObj.Status.Phase == appsv1.RunningComponentPhase
}

func (t *rolloutCreateTransformer) instanceTemplate(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (*appsv1.InstanceTemplate, error) {
	name := string(transCtx.Rollout.UID[:8])
	for i, tpl := range spec.Instances {
		if tpl.Name == name {
			return &spec.Instances[i], nil
		}
	}
	if len(spec.Instances) > 0 && !spec.FlatInstanceOrdinal {
		return nil, fmt.Errorf("not support the create strategy with the flatInstanceOrdinal is false")
	}
	spec.Instances = append(spec.Instances, appsv1.InstanceTemplate{
		Name:           name,
		ServiceVersion: comp.ServiceVersion,
		CompDef:        comp.CompDef,
		Canary:         ptr.To(true),
		Replicas:       ptr.To[int32](0),
	})
	spec.FlatInstanceOrdinal = true
	return &spec.Instances[len(spec.Instances)-1], nil
}

func (t *rolloutCreateTransformer) promote(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas, targetReplicas int32) error {
	if comp.Promotion == nil || !ptr.Deref(comp.Promotion.Auto, false) {
		return nil
	}

	// TODO: promote

	return nil
}
