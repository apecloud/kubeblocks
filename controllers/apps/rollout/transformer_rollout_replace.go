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
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type rolloutReplaceTransformer struct {
}

var _ graph.Transformer = &rolloutReplaceTransformer{}

func (t *rolloutReplaceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) {
		return nil
	}

	return t.rollout(transCtx, dag)
}

func (t *rolloutReplaceTransformer) rollout(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	if err := t.components(transCtx, dag); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutReplaceTransformer) components(transCtx *rolloutTransformContext, dag *graph.DAG) error {
	var delayedError error
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Replace != nil {
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

func (t *rolloutReplaceTransformer) component(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	if spec == nil {
		return fmt.Errorf("the component %s is not found in cluster", comp.Name)
	}

	replicas, _, err := t.replicas(rollout, comp, spec)
	if err != nil {
		return err
	}

	return t.rolling(transCtx, comp, spec, replicas)
}

func (t *rolloutReplaceTransformer) replicas(rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (int32, int32, error) {
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
	if target < 0 || target > replicas {
		return 0, 0, errors.Errorf("the target replicas %d is out-of-range, component %s, replicas: %d", target, comp.Name, replicas)
	}
	if target > 0 && target < spec.Replicas {
		return 0, 0, fmt.Errorf("partially rollout with the replace strategy not supported, component: %s", comp.Name)
	}

	return replicas, target, nil
}

func (t *rolloutReplaceTransformer) rolling(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas int32) error {
	tpl := t.instanceTemplate(transCtx, comp, spec)
	if *tpl.Replicas == replicas && spec.Replicas == replicas {
		return nil
	}

	if !t.status(transCtx, comp) {
		return controllerutil.NewDelayedRequeueError(time.Second, fmt.Sprintf("component %s is not ready", comp.Name))
	}

	if spec.Replicas == replicas {
		t.up(spec, tpl)
	} else {
		t.down(spec, tpl)
	}
	return nil
}

func (t *rolloutReplaceTransformer) status(transCtx *rolloutTransformContext, comp appsv1alpha1.RolloutComponent) bool {
	status := transCtx.Cluster.Status.Components[comp.Name]
	return status.Phase == appsv1.RunningComponentPhase
}

func (t *rolloutReplaceTransformer) instanceTemplate(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) *appsv1.InstanceTemplate {
	name := string(transCtx.Rollout.UID[:8])
	for i, tpl := range spec.Instances {
		if tpl.Name == name {
			return &spec.Instances[i]
		}
	}
	spec.Instances = append(spec.Instances, appsv1.InstanceTemplate{
		Name:           name,
		ServiceVersion: comp.ServiceVersion,
		CompDef:        comp.CompDef,
		Replicas:       ptr.To[int32](0),
	})
	return &spec.Instances[len(spec.Instances)-1]
}

func (t *rolloutReplaceTransformer) up(spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) {
	tpl.Replicas = ptr.To(*tpl.Replicas + 1)
	spec.Replicas += 1
}

func (t *rolloutReplaceTransformer) down(spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) {
	spec.Replicas -= 1
	if spec.OfflineInstances == nil {
		spec.OfflineInstances = make([]string, 0)
	}
	spec.OfflineInstances = append(spec.OfflineInstances, t.pickInstanceToScaleDown(spec, tpl))
}

func (t *rolloutReplaceTransformer) pickInstanceToScaleDown(spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) string {
	return "" // TODO
}
