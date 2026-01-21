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
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	states1, err := t.components(transCtx, rollout)
	if err != nil {
		return err
	}
	states2, err := t.shardings(transCtx, rollout)
	if err != nil {
		return err
	}

	rollout.Status.ObservedGeneration = rollout.Generation
	rollout.Status.State = t.compose(states1, states2)

	// TODO: error message, conditions

	return nil
}

func (t *rolloutStatusTransformer) compose(states1, states2 []appsv1alpha1.RolloutState) appsv1alpha1.RolloutState {
	var (
		hasError   = false
		hasRolling = false
		hasSucceed = false
		hasPending = false
		allSucceed = true
	)
	for _, state := range append(states1, states2...) {
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
		return appsv1alpha1.ErrorRolloutState
	case hasRolling:
		return appsv1alpha1.RollingRolloutState
	case allSucceed:
		return appsv1alpha1.SucceedRolloutState
	case hasSucceed:
		return appsv1alpha1.RollingRolloutState
	case hasPending:
		return appsv1alpha1.PendingRolloutState
	default:
		return ""
	}
}

func (t *rolloutStatusTransformer) components(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout) ([]appsv1alpha1.RolloutState, error) {
	states := make([]appsv1alpha1.RolloutState, 0)
	for _, comp := range rollout.Spec.Components {
		state, err := t.component(transCtx, rollout, comp)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (t *rolloutStatusTransformer) component(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	if comp.Strategy.Inplace != nil {
		return t.compInplace(transCtx, rollout, comp)
	}
	if comp.Strategy.Replace != nil {
		return t.compReplace(transCtx, rollout, comp)
	}
	if comp.Strategy.Create != nil {
		return t.compCreate(transCtx, rollout, comp)
	}
	return "", nil
}

func (t *rolloutStatusTransformer) compInplace(transCtx *rolloutTransformContext,
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

func (t *rolloutStatusTransformer) compReplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	spec := t.compSpec(transCtx, comp.Name)
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	if slices.IndexFunc(spec.Instances, func(tpl appsv1.InstanceTemplate) bool {
		return strings.HasPrefix(tpl.Name, prefix)
	}) < 0 {
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
			return strings.HasPrefix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], prefix)
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

	tpls, _, err := replaceCompInstanceTemplates(rollout, comp, spec)
	if err != nil {
		return "", err
	}
	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if !checkClusterNCompRunning(transCtx, comp.Name) || spec.Replicas != newReplicas {
		return appsv1alpha1.RollingRolloutState, nil
	}
	if allPodCnt != spec.Replicas {
		return appsv1alpha1.RollingRolloutState, nil // scaling down
	}
	return appsv1alpha1.SucceedRolloutState, nil
}

func (t *rolloutStatusTransformer) compCreate(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent) (appsv1alpha1.RolloutState, error) {
	spec := t.compSpec(transCtx, comp.Name)
	prefix := replaceInstanceTemplateNamePrefix(rollout)

	// Check if the instance template exists
	if slices.IndexFunc(spec.Instances, func(tpl appsv1.InstanceTemplate) bool {
		return strings.HasPrefix(tpl.Name, prefix)
	}) < 0 {
		return appsv1alpha1.PendingRolloutState, nil
	}

	// Get pods for the component
	pods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(rollout.Namespace),
		client.MatchingLabels(constant.GetCompLabels(rollout.Spec.ClusterName, comp.Name)),
	}
	if err := transCtx.Client.List(transCtx.Context, pods, listOpts...); err != nil {
		return "", err
	}

	canaryPodCnt := int32(generics.CountFunc(pods.Items, func(pod corev1.Pod) bool {
		if pod.Labels != nil {
			return strings.HasPrefix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], prefix)
		}
		return false
	}))

	// Update status for the component
	for i, status := range rollout.Status.Components {
		if status.Name == comp.Name {
			if checkClusterNCompRunning(transCtx, comp.Name) {
				// Update timestamps when canary replicas change
				if status.CanaryReplicas < canaryPodCnt {
					rollout.Status.Components[i].LastScaleUpTimestamp = metav1.Now()
				}

				rollout.Status.Components[i].CanaryReplicas = canaryPodCnt
				rollout.Status.Components[i].NewReplicas = canaryPodCnt
				// For create strategy, rolled out replicas equals canary replicas
				rollout.Status.Components[i].RolledOutReplicas = canaryPodCnt
			}
			break
		}
	}

	// Determine state
	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if we have reached the target replicas from instance template
	var templateTargetReplicas int32 = 0
	for _, tpl := range spec.Instances {
		if strings.HasPrefix(tpl.Name, prefix) && tpl.Replicas != nil {
			templateTargetReplicas = *tpl.Replicas
			break
		}
	}

	if canaryPodCnt < templateTargetReplicas {
		return appsv1alpha1.RollingRolloutState, nil
	}

	// All canary pods are ready and reached target count
	// Check promotion strategy
	if comp.Strategy.Create == nil || comp.Strategy.Create.Promotion == nil {
		// No promotion configured, stay in rolling state until manual promotion
		return appsv1alpha1.RollingRolloutState, nil
	}

	promotion := comp.Strategy.Create.Promotion

	// Find component status
	var compStatus *appsv1alpha1.RolloutComponentStatus
	for i := range rollout.Status.Components {
		if rollout.Status.Components[i].Name == comp.Name {
			compStatus = &rollout.Status.Components[i]
			break
		}
	}
	if compStatus == nil {
		// Component status not found, should not happen
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if auto promotion is enabled
	if !ptr.Deref(promotion.Auto, false) {
		// Auto promotion not enabled, stay in rolling state
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Auto promotion enabled
	// Check promotion delay
	delaySeconds := ptr.Deref(promotion.DelaySeconds, 30)
	if !compStatus.LastScaleUpTimestamp.IsZero() {
		elapsed := time.Since(compStatus.LastScaleUpTimestamp.Time)
		if elapsed < time.Duration(delaySeconds)*time.Second {
			// Promotion delay not yet passed
			return appsv1alpha1.RollingRolloutState, nil
		}
	} else {
		// LastScaleUpTimestamp not set yet, promotion hasn't started
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check pre-promotion condition if specified
	if promotion.Condition != nil && promotion.Condition.Prev != nil {
		// TODO: implement condition checking
		// For now, assume condition is not met if specified
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if canary instance template is still marked as canary
	// Find the canary instance template
	var canaryTpl *appsv1.InstanceTemplate
	for i := range spec.Instances {
		if strings.HasPrefix(spec.Instances[i].Name, prefix) {
			canaryTpl = &spec.Instances[i]
			break
		}
	}
	if canaryTpl != nil && ptr.Deref(canaryTpl.Canary, false) {
		// Canary instance template is still marked as canary
		// Promotion hasn't been executed yet
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check scale down delay
	scaleDownDelaySeconds := ptr.Deref(promotion.ScaleDownDelaySeconds, 30)
	if scaleDownDelaySeconds > 0 {
		// Check if scale down delay has passed since promotion started
		// We use LastScaleUpTimestamp as promotion start time
		elapsedSincePromotion := time.Since(compStatus.LastScaleUpTimestamp.Time)
		if elapsedSincePromotion < time.Duration(scaleDownDelaySeconds)*time.Second {
			// Scale down delay not yet passed
			return appsv1alpha1.RollingRolloutState, nil
		}
	}

	// Check if old instances have been scaled down
	// Count total replicas from all instance templates
	totalReplicasFromTemplates := int32(0)
	for _, tpl := range spec.Instances {
		if tpl.Replicas != nil {
			totalReplicasFromTemplates += *tpl.Replicas
		}
	}

	// Check if total replicas match the target (canary replicas count)
	// After promotion, total replicas should equal canaryPodCnt (promoted replicas)
	if totalReplicasFromTemplates != canaryPodCnt {
		// Old instances not fully scaled down yet
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check post-promotion condition if specified
	if promotion.Condition != nil && promotion.Condition.Post != nil {
		// TODO: implement condition checking
		// For now, assume condition is not met if specified
		return appsv1alpha1.RollingRolloutState, nil
	}

	// All promotion steps completed
	return appsv1alpha1.SucceedRolloutState, nil
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

func (t *rolloutStatusTransformer) shardings(transCtx *rolloutTransformContext, rollout *appsv1alpha1.Rollout) ([]appsv1alpha1.RolloutState, error) {
	states := make([]appsv1alpha1.RolloutState, 0)
	for _, sharding := range rollout.Spec.Shardings {
		state, err := t.sharding(transCtx, rollout, sharding)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, nil
}

func (t *rolloutStatusTransformer) sharding(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) (appsv1alpha1.RolloutState, error) {
	if sharding.Strategy.Inplace != nil {
		return t.shardingInplace(transCtx, rollout, sharding)
	}
	if sharding.Strategy.Replace != nil {
		return t.shardingReplace(transCtx, rollout, sharding)
	}
	if sharding.Strategy.Create != nil {
		return t.shardingCreate(transCtx, rollout, sharding)
	}
	return "", nil
}

func (t *rolloutStatusTransformer) shardingInplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) (appsv1alpha1.RolloutState, error) {
	spec := t.shardingSpec(transCtx, sharding.Name)
	shardingDef, serviceVersion, compDef := shardingDefNServiceVersionNCompDef(rollout, sharding, spec)
	if shardingDef == spec.ShardingDef && serviceVersion == spec.Template.ServiceVersion && compDef == spec.Template.ComponentDef {
		return appsv1alpha1.PendingRolloutState, nil
	}
	if checkClusterNShardingRunning(transCtx, sharding.Name) {
		return appsv1alpha1.SucceedRolloutState, nil
	}
	return appsv1alpha1.RollingRolloutState, nil
}

func (t *rolloutStatusTransformer) shardingReplace(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) (appsv1alpha1.RolloutState, error) {
	spec := t.shardingSpec(transCtx, sharding.Name)
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	if slices.IndexFunc(spec.Template.Instances, func(tpl appsv1.InstanceTemplate) bool {
		return strings.HasPrefix(tpl.Name, prefix)
	}) < 0 {
		return appsv1alpha1.PendingRolloutState, nil
	}

	pods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(rollout.Namespace),
		client.MatchingLabels(constant.GetClusterLabels(rollout.Spec.ClusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: sharding.Name,
		})),
	}
	if err := transCtx.Client.List(transCtx.Context, pods, listOpts...); err != nil {
		return "", err
	}

	allPodCnt := int32(len(pods.Items))
	newPodCnt := int32(generics.CountFunc(pods.Items, func(pod corev1.Pod) bool {
		if pod.Labels != nil {
			return strings.HasPrefix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], prefix)
		}
		return false
	}))
	for i, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			if checkClusterNShardingRunning(transCtx, sharding.Name) {
				newReplicas, rolledOutReplicas := newPodCnt, newPodCnt-(allPodCnt-status.Replicas)
				if rolledOutReplicas == newReplicas {
					if status.RolledOutReplicas < rolledOutReplicas {
						rollout.Status.Shardings[i].LastScaleDownTimestamp = metav1.Now()
					}
				} else {
					if status.NewReplicas < newReplicas {
						rollout.Status.Shardings[i].LastScaleUpTimestamp = metav1.Now()
					}
				}
				rollout.Status.Shardings[i].NewReplicas = newReplicas
				rollout.Status.Shardings[i].RolledOutReplicas = rolledOutReplicas
			}
			break
		}
	}

	tpls, _, err := replaceShardingInstanceTemplates(rollout, sharding, spec)
	if err != nil {
		return "", err
	}
	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if !checkClusterNShardingRunning(transCtx, sharding.Name) || spec.Template.Replicas != newReplicas {
		return appsv1alpha1.RollingRolloutState, nil
	}
	if allPodCnt != spec.Template.Replicas*spec.Shards {
		return appsv1alpha1.RollingRolloutState, nil // scaling down
	}
	return appsv1alpha1.SucceedRolloutState, nil
}

func (t *rolloutStatusTransformer) shardingCreate(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) (appsv1alpha1.RolloutState, error) {
	spec := t.shardingSpec(transCtx, sharding.Name)
	prefix := replaceInstanceTemplateNamePrefix(rollout)

	// Check if the instance template exists in sharding template
	if slices.IndexFunc(spec.Template.Instances, func(tpl appsv1.InstanceTemplate) bool {
		return strings.HasPrefix(tpl.Name, prefix)
	}) < 0 {
		return appsv1alpha1.PendingRolloutState, nil
	}

	// Get pods for the sharding (all shards)
	pods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(rollout.Namespace),
		client.MatchingLabels(constant.GetClusterLabels(rollout.Spec.ClusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: sharding.Name,
		})),
	}
	if err := transCtx.Client.List(transCtx.Context, pods, listOpts...); err != nil {
		return "", err
	}

	// Count canary pods across all shards
	canaryPodCnt := int32(generics.CountFunc(pods.Items, func(pod corev1.Pod) bool {
		if pod.Labels != nil {
			return strings.HasPrefix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], prefix)
		}
		return false
	}))

	// Update status for the sharding
	for i, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			if checkClusterNShardingRunning(transCtx, sharding.Name) {
				// Update timestamps when canary replicas change
				if status.CanaryReplicas < canaryPodCnt {
					rollout.Status.Shardings[i].LastScaleUpTimestamp = metav1.Now()
				}

				rollout.Status.Shardings[i].CanaryReplicas = canaryPodCnt
				rollout.Status.Shardings[i].NewReplicas = canaryPodCnt
				// For create strategy, rolled out replicas equals canary replicas
				rollout.Status.Shardings[i].RolledOutReplicas = canaryPodCnt
			}
			break
		}
	}

	// Determine state
	if !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if we have reached the target replicas from instance template
	var templateTargetReplicas int32 = 0
	for _, tpl := range spec.Template.Instances {
		if strings.HasPrefix(tpl.Name, prefix) && tpl.Replicas != nil {
			templateTargetReplicas = *tpl.Replicas
			break
		}
	}

	// For sharding, total target replicas = templateTargetReplicas * number of shards
	totalTargetReplicas := templateTargetReplicas * spec.Shards
	if canaryPodCnt < totalTargetReplicas {
		return appsv1alpha1.RollingRolloutState, nil
	}

	// All canary pods are ready and reached target count
	// Check promotion strategy
	if sharding.Strategy.Create == nil || sharding.Strategy.Create.Promotion == nil {
		// No promotion configured, stay in rolling state until manual promotion
		return appsv1alpha1.RollingRolloutState, nil
	}

	promotion := sharding.Strategy.Create.Promotion

	// Find sharding status
	var shardingStatus *appsv1alpha1.RolloutShardingStatus
	for i := range rollout.Status.Shardings {
		if rollout.Status.Shardings[i].Name == sharding.Name {
			shardingStatus = &rollout.Status.Shardings[i]
			break
		}
	}
	if shardingStatus == nil {
		// Sharding status not found, should not happen
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if auto promotion is enabled
	if !ptr.Deref(promotion.Auto, false) {
		// Auto promotion not enabled, stay in rolling state
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Auto promotion enabled
	// Check promotion delay
	delaySeconds := ptr.Deref(promotion.DelaySeconds, 30)
	if !shardingStatus.LastScaleUpTimestamp.IsZero() {
		elapsed := time.Since(shardingStatus.LastScaleUpTimestamp.Time)
		if elapsed < time.Duration(delaySeconds)*time.Second {
			// Promotion delay not yet passed
			return appsv1alpha1.RollingRolloutState, nil
		}
	} else {
		// LastScaleUpTimestamp not set yet, promotion hasn't started
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check pre-promotion condition if specified
	if promotion.Condition != nil && promotion.Condition.Prev != nil {
		// TODO: implement condition checking
		// For now, assume condition is not met if specified
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check if canary instance template is still marked as canary
	// Find the canary instance template
	var canaryTpl *appsv1.InstanceTemplate
	for i := range spec.Template.Instances {
		if strings.HasPrefix(spec.Template.Instances[i].Name, prefix) {
			canaryTpl = &spec.Template.Instances[i]
			break
		}
	}
	if canaryTpl != nil && ptr.Deref(canaryTpl.Canary, false) {
		// Canary instance template is still marked as canary
		// Promotion hasn't been executed yet
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check scale down delay
	scaleDownDelaySeconds := ptr.Deref(promotion.ScaleDownDelaySeconds, 30)
	if scaleDownDelaySeconds > 0 {
		// Check if scale down delay has passed since promotion started
		// We use LastScaleUpTimestamp as promotion start time
		elapsedSincePromotion := time.Since(shardingStatus.LastScaleUpTimestamp.Time)
		if elapsedSincePromotion < time.Duration(scaleDownDelaySeconds)*time.Second {
			// Scale down delay not yet passed
			return appsv1alpha1.RollingRolloutState, nil
		}
	}

	// Check if old instances have been scaled down
	// Count total replicas from all instance templates
	totalReplicasFromTemplates := int32(0)
	for _, tpl := range spec.Template.Instances {
		if tpl.Replicas != nil {
			totalReplicasFromTemplates += *tpl.Replicas
		}
	}

	// For sharding, total replicas = replicas per shard * number of shards
	totalReplicasFromTemplates *= spec.Shards

	// Check if total replicas match the target (canary replicas count)
	// After promotion, total replicas should equal canaryPodCnt (promoted replicas)
	if totalReplicasFromTemplates != canaryPodCnt {
		// Old instances not fully scaled down yet
		return appsv1alpha1.RollingRolloutState, nil
	}

	// Check post-promotion condition if specified
	if promotion.Condition != nil && promotion.Condition.Post != nil {
		// TODO: implement condition checking
		// For now, assume condition is not met if specified
		return appsv1alpha1.RollingRolloutState, nil
	}

	// All promotion steps completed
	return appsv1alpha1.SucceedRolloutState, nil
}

func (t *rolloutStatusTransformer) shardingSpec(transCtx *rolloutTransformContext, shardingName string) *appsv1.ClusterSharding {
	// use the original cluster spec
	cluster := transCtx.ClusterOrig
	for i, sharding := range cluster.Spec.Shardings {
		if sharding.Name == shardingName {
			return &cluster.Spec.Shardings[i]
		}
	}
	return nil
}

func isRolloutSucceed(rollout *appsv1alpha1.Rollout) bool {
	return rollout.Status.State == appsv1alpha1.SucceedRolloutState
}
