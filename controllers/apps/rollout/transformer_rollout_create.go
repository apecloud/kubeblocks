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
	// TODO: sharding
	return nil
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
	if target < 0 {
		return 0, 0, errors.Errorf("invalid target replicas %d for component %s", target, comp.Name)
	}

	return replicas, target, nil
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
	if comp.Strategy.Create.Promotion == nil || !ptr.Deref(comp.Strategy.Create.Promotion.Auto, false) {
		return nil
	}

	// Find the canary instance template
	prefix := replaceInstanceTemplateNamePrefix(transCtx.Rollout)
	var canaryTpl *appsv1.InstanceTemplate
	for i := range spec.Instances {
		if spec.Instances[i].Name == prefix {
			canaryTpl = &spec.Instances[i]
			break
		}
	}
	if canaryTpl == nil {
		// Canary instance template not found, nothing to promote
		return nil
	}

	// Check if canary replicas have reached target
	// This should be guaranteed by the caller (promote is only called when replicas+targetReplicas <= spec.Replicas)
	// but we check anyway
	if canaryTpl.Replicas == nil || *canaryTpl.Replicas < targetReplicas {
		// Canary replicas not yet reached target, should still be in rolling phase
		return nil
	}

	// Find the component status to check promotion timestamps
	rollout := transCtx.Rollout
	var compStatus *appsv1alpha1.RolloutComponentStatus
	for i := range rollout.Status.Components {
		if rollout.Status.Components[i].Name == comp.Name {
			compStatus = &rollout.Status.Components[i]
			break
		}
	}
	if compStatus == nil {
		// Component status not found, should not happen
		return nil
	}

	// Check promotion delay
	promotion := comp.Strategy.Create.Promotion
	delaySeconds := ptr.Deref(promotion.DelaySeconds, 30)

	// Use LastScaleUpTimestamp as promotion start time
	// When canary replicas first reach target, LastScaleUpTimestamp should be set
	if !compStatus.LastScaleUpTimestamp.IsZero() {
		elapsed := time.Since(compStatus.LastScaleUpTimestamp.Time)
		if elapsed < time.Duration(delaySeconds)*time.Second {
			// Delay not yet passed, requeue
			remaining := time.Duration(delaySeconds)*time.Second - elapsed
			return controllerutil.NewDelayedRequeueError(remaining, fmt.Sprintf("waiting for promotion delay: %v remaining", remaining))
		}
	} else {
		// First time reaching target, set the timestamp
		compStatus.LastScaleUpTimestamp = metav1.Now()
		return controllerutil.NewDelayedRequeueError(time.Second, "setting promotion start time")
	}

	// Check pre-promotion condition if specified
	// if promotion.Condition != nil && promotion.Condition.Prev != nil {
	//	// TODO: implement condition checking
	//	// For now, just log that condition checking is not implemented
	//	// return fmt.Errorf("pre-promotion condition checking not implemented yet")
	// }

	// Execute promotion: mark canary instance template as non-canary
	canaryTpl.Canary = ptr.To(false)

	// Check if we need to scale down old instances
	scaleDownDelaySeconds := ptr.Deref(promotion.ScaleDownDelaySeconds, 30)
	if scaleDownDelaySeconds > 0 {
		// Check if scale down delay has passed since promotion started
		// We use LastScaleUpTimestamp as promotion start time
		elapsedSincePromotion := time.Since(compStatus.LastScaleUpTimestamp.Time)
		if elapsedSincePromotion < time.Duration(scaleDownDelaySeconds)*time.Second {
			// Scale down delay not yet passed
			remaining := time.Duration(scaleDownDelaySeconds)*time.Second - elapsedSincePromotion
			return controllerutil.NewDelayedRequeueError(remaining, fmt.Sprintf("waiting for scale down delay: %v remaining", remaining))
		}
	}

	// Scale down old instances: reduce original replicas
	// The original replicas are stored in 'replicas' parameter
	// We need to find the original instance template(s) and reduce their replicas
	// For simplicity, we assume there's a default instance template (without the prefix)
	// or we need to identify which instances are old

	// For now, we'll reduce the total replicas to match targetReplicas (canary replicas)
	// since canary instances are now promoted to stable
	// This assumes canary instances replace old instances one-to-one
	// spec.Replicas should already include canary replicas, so we need to reduce it by (replicas - targetReplicas)
	// where 'replicas' is the original stable replicas count
	scaleDownCount := replicas - targetReplicas
	if scaleDownCount > 0 {
		// Reduce total replicas
		spec.Replicas -= scaleDownCount

		// Also need to reduce replicas in the original instance template(s)
		// For now, we assume there's a default instance template
		for i := range spec.Instances {
			if spec.Instances[i].Name != prefix && spec.Instances[i].Replicas != nil && *spec.Instances[i].Replicas > 0 {
				// Reduce replicas of this old instance template
				oldReplicas := *spec.Instances[i].Replicas
				reduceBy := scaleDownCount
				if reduceBy > oldReplicas {
					reduceBy = oldReplicas
				}
				spec.Instances[i].Replicas = ptr.To(oldReplicas - reduceBy)
				scaleDownCount -= reduceBy

				// Add to scale down instances in status
				// TODO: track which specific instances are scaled down
				if compStatus != nil {
					// For simplicity, just mark that scaling down happened
					compStatus.LastScaleDownTimestamp = metav1.Now()
				}

				if scaleDownCount <= 0 {
					break
				}
			}
		}
	}

	// Check post-promotion condition if specified
	// if promotion.Condition != nil && promotion.Condition.Post != nil {
	//	// TODO: implement condition checking
	//	// For now, just log that condition checking is not implemented
	//	// return fmt.Errorf("post-promotion condition checking not implemented yet")
	// }

	// Promotion completed
	return nil
}
