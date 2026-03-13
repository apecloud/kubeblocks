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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}
	return t.rollout(transCtx)
}

func (t *rolloutCreateTransformer) rollout(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	return t.shardings(transCtx)
}

func (t *rolloutCreateTransformer) components(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, comp := range rollout.Spec.Components {
		if comp.Strategy.Create != nil {
			if err := t.component(transCtx, rollout, comp); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutCreateTransformer) shardings(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, sharding := range rollout.Spec.Shardings {
		if sharding.Strategy.Create != nil {
			if err := t.sharding(transCtx, rollout, sharding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutCreateTransformer) component(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) error {
	spec := transCtx.ClusterComps[comp.Name]
	replicas, targetReplicas, err := t.replicas(rollout, comp, spec)
	if err != nil {
		return err
	}

	if (replicas + targetReplicas) > spec.Replicas {
		return t.rolling(transCtx, comp, spec, replicas, targetReplicas)
	}

	return t.promote(transCtx, comp, spec, replicas, targetReplicas)
}

func (t *rolloutCreateTransformer) sharding(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	spec := transCtx.ClusterShardings[sharding.Name]
	replicas, targetReplicas, err := createShardingReplicas(rollout, sharding, spec)
	if err != nil {
		return err
	}

	if (replicas + targetReplicas) > spec.Template.Replicas {
		return t.shardingRolling(transCtx, sharding, spec, replicas, targetReplicas)
	}

	return t.shardingPromote(transCtx, sharding, spec, replicas, targetReplicas)
}

func (t *rolloutCreateTransformer) replicas(rollout *appsv1alpha1.Rollout,
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
	target, err := createComponentTargetReplicas(comp, replicas)
	if err != nil {
		return 0, 0, err
	}

	return replicas, target, nil
}

func createComponentTargetReplicas(comp appsv1alpha1.RolloutComponent, originalReplicas int32) (int32, error) {
	if comp.Replicas == nil {
		return 0, nil
	}
	target, err := intstr.GetScaledValueFromIntOrPercent(comp.Replicas, int(originalReplicas), false)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get scaled value for replicas of component %s", comp.Name)
	}
	if target < 0 || int32(target) > originalReplicas {
		return 0, errors.Errorf("the target replicas %d is out-of-range, component %s, replicas: %d", target, comp.Name, originalReplicas)
	}
	return int32(target), nil
}

func (t *rolloutCreateTransformer) rolling(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas, targetReplicas int32) error {
	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	tpl, err := t.instanceTemplate(transCtx, comp, spec)
	if err != nil {
		return err
	}
	spec.Replicas += targetReplicas
	tpl.Replicas = ptr.To(targetReplicas)

	return nil
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
	tpl := appsv1.InstanceTemplate{
		Name:     name,
		Canary:   comp.Strategy.Create.Canary,
		Replicas: ptr.To[int32](0),
	}
	if comp.ServiceVersion != nil {
		tpl.ServiceVersion = *comp.ServiceVersion
	}
	if comp.CompDef != nil {
		tpl.CompDef = *comp.CompDef
	}
	tpl.SchedulingPolicy = rolloutSchedulingPolicy(comp.Strategy.Create.SchedulingPolicy)
	if comp.InstanceMeta != nil && comp.InstanceMeta.Canary != nil {
		tpl.Labels = comp.InstanceMeta.Canary.Labels
		tpl.Annotations = comp.InstanceMeta.Canary.Annotations
	}
	spec.Instances = append(spec.Instances, tpl)
	spec.FlatInstanceOrdinal = true
	return &spec.Instances[len(spec.Instances)-1], nil
}

func (t *rolloutCreateTransformer) promote(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas, targetReplicas int32) error {
	promotion := comp.Strategy.Create.Promotion
	if promotion == nil || !ptr.Deref(promotion.Auto, false) || !checkClusterNCompRunning(transCtx, comp.Name) {
		return nil
	}

	prefix := replaceInstanceTemplateNamePrefix(transCtx.Rollout)
	canaryTpl := createInstanceTemplate(spec.Instances, prefix)
	if canaryTpl == nil {
		return nil
	}
	compStatus := createCompStatus(transCtx.Rollout, comp.Name)
	if compStatus == nil || compStatus.CanaryReplicas < targetReplicas {
		return nil
	}
	if promotion.Condition != nil && (promotion.Condition.Prev != nil || promotion.Condition.Post != nil) {
		return createStrategyNotSupportedError
	}

	if ptr.Deref(canaryTpl.Canary, false) {
		if compStatus.LastScaleUpTimestamp.IsZero() {
			return nil
		}
		if diff := createDelayRemaining(compStatus.LastScaleUpTimestamp, ptr.Deref(promotion.DelaySeconds, 30)); diff > 0 {
			return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for promotion delay: %v remaining", diff))
		}
		canaryTpl.Canary = ptr.To(false)
		compStatus.LastScaleDownTimestamp = metav1.Now()
	}

	if ptr.Deref(promotion.ScaleDownDelaySeconds, 30) > 0 && compStatus.LastScaleDownTimestamp.IsZero() {
		return nil
	}
	if diff := createDelayRemaining(compStatus.LastScaleDownTimestamp, ptr.Deref(promotion.ScaleDownDelaySeconds, 30)); diff > 0 {
		return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for scale down delay: %v remaining", diff))
	}

	scaleDownCount := spec.Replicas - replicas
	if scaleDownCount <= 0 {
		return nil
	}
	spec.Replicas = replicas
	for i := range spec.Instances {
		if spec.Instances[i].Name == prefix || spec.Instances[i].Replicas == nil || *spec.Instances[i].Replicas == 0 {
			continue
		}
		reduceBy := scaleDownCount
		if reduceBy > *spec.Instances[i].Replicas {
			reduceBy = *spec.Instances[i].Replicas
		}
		spec.Instances[i].Replicas = ptr.To(*spec.Instances[i].Replicas - reduceBy)
		scaleDownCount -= reduceBy
		if scaleDownCount == 0 {
			break
		}
	}

	return nil
}

func (t *rolloutCreateTransformer) shardingRolling(transCtx *rolloutTransformContext,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding, replicas, targetReplicas int32) error {
	if !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the sharding %s is not ready", sharding.Name))
	}

	tpl, err := t.shardingInstanceTemplate(transCtx, sharding, spec)
	if err != nil {
		return err
	}
	spec.Template.Replicas += targetReplicas
	tpl.Replicas = ptr.To(targetReplicas)
	return nil
}

func (t *rolloutCreateTransformer) shardingInstanceTemplate(transCtx *rolloutTransformContext,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding) (*appsv1.InstanceTemplate, error) {
	name := string(transCtx.Rollout.UID[:8])
	for i, tpl := range spec.Template.Instances {
		if tpl.Name == name {
			return &spec.Template.Instances[i], nil
		}
	}
	if len(spec.Template.Instances) > 0 && !spec.Template.FlatInstanceOrdinal {
		return nil, fmt.Errorf("not support the create strategy with the flatInstanceOrdinal is false")
	}
	tpl := appsv1.InstanceTemplate{
		Name:     name,
		Canary:   sharding.Strategy.Create.Canary,
		Replicas: ptr.To[int32](0),
	}
	if sharding.ServiceVersion != nil {
		tpl.ServiceVersion = *sharding.ServiceVersion
	}
	if sharding.CompDef != nil {
		tpl.CompDef = *sharding.CompDef
	}
	tpl.SchedulingPolicy = rolloutSchedulingPolicy(sharding.Strategy.Create.SchedulingPolicy)
	if sharding.InstanceMeta != nil && sharding.InstanceMeta.Canary != nil {
		tpl.Labels = sharding.InstanceMeta.Canary.Labels
		tpl.Annotations = sharding.InstanceMeta.Canary.Annotations
	}
	spec.Template.Instances = append(spec.Template.Instances, tpl)
	spec.Template.FlatInstanceOrdinal = true
	return &spec.Template.Instances[len(spec.Template.Instances)-1], nil
}

func (t *rolloutCreateTransformer) shardingPromote(transCtx *rolloutTransformContext,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding, replicas, targetReplicas int32) error {
	promotion := sharding.Strategy.Create.Promotion
	if promotion == nil || !ptr.Deref(promotion.Auto, false) || !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return nil
	}

	prefix := replaceInstanceTemplateNamePrefix(transCtx.Rollout)
	canaryTpl := createInstanceTemplate(spec.Template.Instances, prefix)
	if canaryTpl == nil {
		return nil
	}
	shardingStatus := createShardingStatus(transCtx.Rollout, sharding.Name)
	desiredCanaryReplicas := targetReplicas * spec.Shards
	if shardingStatus == nil || shardingStatus.CanaryReplicas < desiredCanaryReplicas {
		return nil
	}
	if promotion.Condition != nil && (promotion.Condition.Prev != nil || promotion.Condition.Post != nil) {
		return createStrategyNotSupportedError
	}

	if ptr.Deref(canaryTpl.Canary, false) {
		if shardingStatus.LastScaleUpTimestamp.IsZero() {
			return nil
		}
		if diff := createDelayRemaining(shardingStatus.LastScaleUpTimestamp, ptr.Deref(promotion.DelaySeconds, 30)); diff > 0 {
			return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for promotion delay: %v remaining", diff))
		}
		canaryTpl.Canary = ptr.To(false)
		shardingStatus.LastScaleDownTimestamp = metav1.Now()
	}

	if ptr.Deref(promotion.ScaleDownDelaySeconds, 30) > 0 && shardingStatus.LastScaleDownTimestamp.IsZero() {
		return nil
	}
	if diff := createDelayRemaining(shardingStatus.LastScaleDownTimestamp, ptr.Deref(promotion.ScaleDownDelaySeconds, 30)); diff > 0 {
		return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for scale down delay: %v remaining", diff))
	}

	scaleDownCount := spec.Template.Replicas - replicas
	if scaleDownCount <= 0 {
		return nil
	}
	spec.Template.Replicas = replicas
	for i := range spec.Template.Instances {
		if spec.Template.Instances[i].Name == prefix || spec.Template.Instances[i].Replicas == nil || *spec.Template.Instances[i].Replicas == 0 {
			continue
		}
		reduceBy := scaleDownCount
		if reduceBy > *spec.Template.Instances[i].Replicas {
			reduceBy = *spec.Template.Instances[i].Replicas
		}
		spec.Template.Instances[i].Replicas = ptr.To(*spec.Template.Instances[i].Replicas - reduceBy)
		scaleDownCount -= reduceBy
		if scaleDownCount == 0 {
			break
		}
	}

	return nil
}

func createInstanceTemplate(instances []appsv1.InstanceTemplate, name string) *appsv1.InstanceTemplate {
	for i := range instances {
		if instances[i].Name == name {
			return &instances[i]
		}
	}
	return nil
}

func createCompStatus(rollout *appsv1alpha1.Rollout, compName string) *appsv1alpha1.RolloutComponentStatus {
	for i := range rollout.Status.Components {
		if rollout.Status.Components[i].Name == compName {
			return &rollout.Status.Components[i]
		}
	}
	return nil
}

func createShardingStatus(rollout *appsv1alpha1.Rollout, shardingName string) *appsv1alpha1.RolloutShardingStatus {
	for i := range rollout.Status.Shardings {
		if rollout.Status.Shardings[i].Name == shardingName {
			return &rollout.Status.Shardings[i]
		}
	}
	return nil
}

func createShardingReplicas(rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding) (int32, int32, error) {
	replicas := spec.Template.Replicas
	for _, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			if spec.Shards == 0 {
				return 0, 0, nil
			}
			replicas = status.Replicas / spec.Shards
			break
		}
	}
	if sharding.Replicas == nil {
		return replicas, 0, nil
	}
	target, err := intstr.GetScaledValueFromIntOrPercent(sharding.Replicas, int(replicas), false)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "failed to get scaled value for replicas of sharding %s", sharding.Name)
	}
	if target < 0 || int32(target) > replicas {
		return 0, 0, errors.Errorf("the target replicas %d is out-of-range, sharding %s, replicas: %d", target, sharding.Name, replicas)
	}
	return replicas, int32(target), nil
}

func createDelayRemaining(lastTimestamp metav1.Time, delaySeconds int32) time.Duration {
	if delaySeconds <= 0 || lastTimestamp.IsZero() {
		return 0
	}
	diff := time.Until(lastTimestamp.Add(time.Duration(delaySeconds) * time.Second))
	if diff < 0 {
		return 0
	}
	return diff
}

func rolloutSchedulingPolicy(policy *appsv1alpha1.SchedulingPolicy) *appsv1.SchedulingPolicy {
	if policy == nil {
		return nil
	}
	return &appsv1.SchedulingPolicy{
		SchedulerName:             policy.SchedulerName,
		NodeSelector:              policy.NodeSelector,
		NodeName:                  policy.NodeName,
		Affinity:                  policy.Affinity,
		Tolerations:               policy.Tolerations,
		TopologySpreadConstraints: policy.TopologySpreadConstraints,
	}
}
