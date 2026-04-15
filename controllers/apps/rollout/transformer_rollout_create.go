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
	target, err := createTargetReplicas(comp.Name, comp.Replicas, replicas, "component")
	if err != nil {
		return 0, 0, err
	}

	return replicas, target, nil
}

func createComponentTargetReplicas(comp appsv1alpha1.RolloutComponent, originalReplicas int32) (int32, error) {
	return createTargetReplicas(comp.Name, comp.Replicas, originalReplicas, "component")
}

func (t *rolloutCreateTransformer) rolling(transCtx *rolloutTransformContext,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec, replicas, targetReplicas int32) error {
	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	tpl, err := createOrUpdateInstanceTemplate(
		string(transCtx.Rollout.UID[:8]),
		&spec.Instances,
		&spec.FlatInstanceOrdinal,
		comp.Strategy.Create.Canary,
		comp.ServiceVersion,
		comp.CompDef,
		comp.Strategy.Create.SchedulingPolicy,
		comp.InstanceMeta,
	)
	if err != nil {
		return err
	}
	applyCreateRolling(&spec.Replicas, tpl, targetReplicas)
	return nil
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
	if err := applyCreatePromotion(promotion, canaryTpl, &compStatus.LastScaleUpTimestamp, &compStatus.LastScaleDownTimestamp); err != nil {
		return err
	}

	scaleDownCount := spec.Replicas - replicas
	if scaleDownCount <= 0 {
		return nil
	}
	spec.Replicas = replicas
	scaleDownInstanceTemplates(spec.Instances, prefix, scaleDownCount)

	return nil
}

func (t *rolloutCreateTransformer) shardingRolling(transCtx *rolloutTransformContext,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding, replicas, targetReplicas int32) error {
	if !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the sharding %s is not ready", sharding.Name))
	}

	tpl, err := createOrUpdateInstanceTemplate(
		string(transCtx.Rollout.UID[:8]),
		&spec.Template.Instances,
		&spec.Template.FlatInstanceOrdinal,
		sharding.Strategy.Create.Canary,
		sharding.ServiceVersion,
		sharding.CompDef,
		sharding.Strategy.Create.SchedulingPolicy,
		sharding.InstanceMeta,
	)
	if err != nil {
		return err
	}
	applyCreateRolling(&spec.Template.Replicas, tpl, targetReplicas)
	return nil
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
	if err := applyCreatePromotion(promotion, canaryTpl, &shardingStatus.LastScaleUpTimestamp, &shardingStatus.LastScaleDownTimestamp); err != nil {
		return err
	}

	scaleDownCount := spec.Template.Replicas - replicas
	if scaleDownCount <= 0 {
		return nil
	}
	spec.Template.Replicas = replicas
	scaleDownInstanceTemplates(spec.Template.Instances, prefix, scaleDownCount)

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
	target, err := createTargetReplicas(sharding.Name, sharding.Replicas, replicas, "sharding")
	if err != nil {
		return 0, 0, err
	}
	return replicas, target, nil
}

func createTargetReplicas(name string, replicasSpec *intstr.IntOrString, originalReplicas int32, subject string) (int32, error) {
	if replicasSpec == nil {
		return 0, nil
	}
	target, err := intstr.GetScaledValueFromIntOrPercent(replicasSpec, int(originalReplicas), false)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get scaled value for replicas of %s %s", subject, name)
	}
	if target < 0 || int32(target) > originalReplicas {
		return 0, errors.Errorf("the target replicas %d is out-of-range, %s %s, replicas: %d", target, subject, name, originalReplicas)
	}
	return int32(target), nil
}

func createOrUpdateInstanceTemplate(name string,
	instances *[]appsv1.InstanceTemplate,
	flatInstanceOrdinal *bool,
	canary *bool,
	serviceVersion *string,
	compDef *string,
	schedulingPolicy *appsv1alpha1.SchedulingPolicy,
	instanceMeta *appsv1alpha1.RolloutInstanceMeta) (*appsv1.InstanceTemplate, error) {
	for i, tpl := range *instances {
		if tpl.Name == name {
			return &(*instances)[i], nil
		}
	}
	if len(*instances) > 0 && !*flatInstanceOrdinal {
		return nil, fmt.Errorf("not support the create strategy with the flatInstanceOrdinal is false")
	}
	tpl := appsv1.InstanceTemplate{
		Name:     name,
		Canary:   canary,
		Replicas: ptr.To[int32](0),
	}
	if serviceVersion != nil {
		tpl.ServiceVersion = *serviceVersion
	}
	if compDef != nil {
		tpl.CompDef = *compDef
	}
	tpl.SchedulingPolicy = rolloutSchedulingPolicy(schedulingPolicy)
	if instanceMeta != nil && instanceMeta.Canary != nil {
		tpl.Labels = instanceMeta.Canary.Labels
		tpl.Annotations = instanceMeta.Canary.Annotations
	}
	*instances = append(*instances, tpl)
	*flatInstanceOrdinal = true
	return &(*instances)[len(*instances)-1], nil
}

func scaleDownInstanceTemplates(instances []appsv1.InstanceTemplate, prefix string, scaleDownCount int32) {
	for i := range instances {
		if instances[i].Name == prefix || instances[i].Replicas == nil || *instances[i].Replicas == 0 {
			continue
		}
		reduceBy := scaleDownCount
		if reduceBy > *instances[i].Replicas {
			reduceBy = *instances[i].Replicas
		}
		instances[i].Replicas = ptr.To(*instances[i].Replicas - reduceBy)
		scaleDownCount -= reduceBy
		if scaleDownCount == 0 {
			return
		}
	}
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

func applyCreateRolling(totalReplicas *int32, tpl *appsv1.InstanceTemplate, targetReplicas int32) {
	*totalReplicas += targetReplicas
	tpl.Replicas = ptr.To(targetReplicas)
}

func applyCreatePromotion(promotion *appsv1alpha1.RolloutPromotion,
	canaryTpl *appsv1.InstanceTemplate,
	lastScaleUpTimestamp, lastScaleDownTimestamp *metav1.Time) error {
	if promotion.Condition != nil && (promotion.Condition.Prev != nil || promotion.Condition.Post != nil) {
		return createStrategyNotSupportedError
	}

	if ptr.Deref(canaryTpl.Canary, false) {
		if lastScaleUpTimestamp.IsZero() {
			return nil
		}
		if diff := createDelayRemaining(*lastScaleUpTimestamp, ptr.Deref(promotion.DelaySeconds, 30)); diff > 0 {
			return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for promotion delay: %v remaining", diff))
		}
		canaryTpl.Canary = ptr.To(false)
		*lastScaleDownTimestamp = metav1.Now()
	}

	if ptr.Deref(promotion.ScaleDownDelaySeconds, 30) > 0 && lastScaleDownTimestamp.IsZero() {
		return nil
	}
	if diff := createDelayRemaining(*lastScaleDownTimestamp, ptr.Deref(promotion.ScaleDownDelaySeconds, 30)); diff > 0 {
		return controllerutil.NewDelayedRequeueError(diff, fmt.Sprintf("waiting for scale down delay: %v remaining", diff))
	}
	return nil
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
