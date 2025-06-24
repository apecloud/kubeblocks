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
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type rolloutReplaceTransformer struct{}

var _ graph.Transformer = &rolloutReplaceTransformer{}

func (t *rolloutReplaceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}
	return t.rollout(transCtx)
}

func (t *rolloutReplaceTransformer) rollout(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	// TODO: sharding
	return nil
}

func (t *rolloutReplaceTransformer) components(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Replace != nil {
			if err := t.component(transCtx, rollout, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutReplaceTransformer) component(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	replicas, _, err := t.replicas(rollout, comp, spec)
	if err != nil {
		return err
	}
	return t.rolling(transCtx, rollout, comp, spec, replicas)
}

func (t *rolloutReplaceTransformer) replicas(rollout *appsv1alpha1.Rollout,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (int32, int32, error) {
	return replaceReplicas(rollout, comp, spec)
}

func (t *rolloutReplaceTransformer) rolling(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas int32) error {
	tpl, exist, err := replaceInstanceTemplate(transCtx, comp, spec)
	if err != nil {
		return err
	}
	if *tpl.Replicas == replicas && spec.Replicas == replicas {
		return nil
	}

	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	// update cluster spec after the cluster and component are ready
	if !exist {
		spec.Instances = append(spec.Instances, *tpl)
		spec.FlatInstanceOrdinal = true
		tpl = &spec.Instances[len(spec.Instances)-1]
	}

	if spec.Replicas == replicas {
		return t.up(transCtx, rollout, comp, spec, tpl)
	} else {
		return t.down(transCtx, rollout, comp, spec, tpl)
	}
}

func (t *rolloutReplaceTransformer) up(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) error {
	if err := t.checkDelaySeconds(rollout, comp, *tpl.Replicas, false); err != nil {
		return err
	}
	tpl.Replicas = ptr.To(*tpl.Replicas + 1)
	spec.Replicas += 1
	return nil
}

func (t *rolloutReplaceTransformer) down(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) error {
	if err := t.checkDelaySeconds(rollout, comp, *tpl.Replicas, true); err != nil {
		return err
	}

	instance, instTpl, err := t.pickInstanceToScaleDown(transCtx, spec, tpl)
	if err != nil {
		return err
	}

	spec.Replicas -= 1
	if instTpl != nil {
		if instTpl.Replicas == nil || *instTpl.Replicas == 0 {
			return fmt.Errorf("the instance template %s still has running instances, but its replicas is already 0", instTpl.Name)
		}
		instTpl.Replicas = ptr.To(*instTpl.Replicas - 1)
	}
	if len(instance) > 0 {
		spec.OfflineInstances = append(spec.OfflineInstances, instance)
	}

	// add the scale down instance to the rollout status
	if len(instance) > 0 {
		for i, status := range rollout.Status.Components {
			if status.Name == spec.Name {
				rollout.Status.Components[i].ScaleDownInstances = append(rollout.Status.Components[i].ScaleDownInstances, instance)
				break
			}
		}
	}

	return nil
}

func (t *rolloutReplaceTransformer) checkDelaySeconds(rollout *appsv1alpha1.Rollout,
	comp appsv1alpha1.RolloutComponent, newReplicas int32, scaleDown bool) error {
	delaySeconds := comp.Strategy.Replace.PerInstanceIntervalSeconds
	if scaleDown {
		delaySeconds = comp.Strategy.Replace.ScaleDownDelaySeconds
	}
	if delaySeconds == nil || *delaySeconds == 0 {
		return nil
	}
	if *delaySeconds < 0 {
		return controllerutil.NewDelayedRequeueError(infiniteDelayRequeueDuration, "infinite delay")
	}

	var lastSucceedTimestamp metav1.Time
	for _, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			if scaleDown {
				if status.NewReplicas == newReplicas {
					lastSucceedTimestamp = status.LastScaleUpTimestamp
				} else {
					return controllerutil.NewDelayedRequeueError(time.Second, "stale up status")
				}
			} else {
				if status.RolledOutReplicas == newReplicas {
					lastSucceedTimestamp = status.LastScaleDownTimestamp
				} else {
					return controllerutil.NewDelayedRequeueError(time.Second, "stale down status")
				}
			}
			break
		}
	}
	if lastSucceedTimestamp.IsZero() {
		return nil
	}

	diff := time.Until(lastSucceedTimestamp.Add(time.Duration(*delaySeconds) * time.Second))
	if diff > 0 {
		if scaleDown {
			return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("delay to scale down for %s seconds", diff.String()))
		}
		return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("delay to rollout next instance for %s seconds", diff.String()))
	}
	return nil
}

func (t *rolloutReplaceTransformer) pickInstanceToScaleDown(transCtx *rolloutTransformContext,
	spec *appsv1.ClusterComponentSpec, tpl *appsv1.InstanceTemplate) (string, *appsv1.InstanceTemplate, error) {
	matchingLabels := constant.GetCompLabels(transCtx.Cluster.Name, spec.Name)
	matchingLabels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
	pods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(transCtx.Cluster.Namespace),
		client.MatchingLabels(matchingLabels),
	}
	if err := transCtx.Client.List(transCtx.Context, pods, listOpts...); err != nil {
		return "", nil, err
	}

	slices.SortFunc(pods.Items, func(a, b corev1.Pod) int {
		return strings.Compare(a.Name, b.Name) * -1
	})
	var targetPod *corev1.Pod
	for i, pod := range pods.Items {
		if pod.DeletionTimestamp == nil && pod.Labels[constant.KBAppInstanceTemplateLabelKey] != tpl.Name {
			targetPod = &pods.Items[i]
			break
		}
	}

	if targetPod == nil {
		return "", nil, nil
	}
	tplName, ok := targetPod.Labels[constant.KBAppInstanceTemplateLabelKey]
	if !ok || len(tplName) == 0 {
		return targetPod.Name, nil, nil
	}
	for i, tpl := range spec.Instances {
		if tpl.Name == tplName {
			return targetPod.Name, &spec.Instances[i], nil
		}
	}
	return "", nil, fmt.Errorf("the instance template %s has not been found", tplName)
}

func replaceReplicas(rollout *appsv1alpha1.Rollout,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (int32, int32, error) {
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
			replicas, err := intstr.GetScaledValueFromIntOrPercent(comp.Replicas, int(replicas), false)
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
	if target > 0 && target < replicas {
		return 0, 0, fmt.Errorf("partially rollout with the replace strategy not supported, component: %s", comp.Name)
	}

	return replicas, target, nil
}

func replaceInstanceTemplate(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (*appsv1.InstanceTemplate, bool, error) {
	name := string(transCtx.Rollout.UID[:8])
	for i, tpl := range spec.Instances {
		if tpl.Name == name {
			return &spec.Instances[i], true, nil
		}
	}
	if len(spec.Instances) > 0 && !spec.FlatInstanceOrdinal {
		return nil, false, fmt.Errorf("not support the replace strategy with the flatInstanceOrdinal is false")
	}
	tpl := &appsv1.InstanceTemplate{
		Name:           name,
		ServiceVersion: comp.ServiceVersion,
		CompDef:        comp.CompDef,
		Replicas:       ptr.To[int32](0),
	}
	if comp.Strategy.Replace.SchedulingPolicy != nil {
		policy := comp.Strategy.Replace.SchedulingPolicy
		tpl.SchedulingPolicy = &appsv1.SchedulingPolicy{
			SchedulerName:             policy.SchedulerName,
			NodeSelector:              policy.NodeSelector,
			NodeName:                  policy.NodeName,
			Affinity:                  policy.Affinity,
			Tolerations:               policy.Tolerations,
			TopologySpreadConstraints: policy.TopologySpreadConstraints,
		}
	}
	if comp.InstanceMeta != nil && comp.InstanceMeta.Canary != nil {
		tpl.Labels = comp.InstanceMeta.Canary.Labels
		tpl.Annotations = comp.InstanceMeta.Canary.Annotations
	}
	return tpl, false, nil
}
