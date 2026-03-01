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
	"maps"
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
	transCtx := ctx.(*rolloutTransformContext)
	if model.IsObjectDeleting(transCtx.RolloutOrig) || isRolloutSucceed(transCtx.RolloutOrig) {
		return nil
	}
	return t.rollout(transCtx)
}

func (t *rolloutReplaceTransformer) rollout(transCtx *rolloutTransformContext) error {
	if err := t.components(transCtx); err != nil {
		return err
	}
	return t.shardings(transCtx)
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
	replicas, _, err := replaceCompReplicas(rollout, comp, spec)
	if err != nil {
		return err
	}
	tpls, exist, err := replaceCompInstanceTemplates(rollout, comp, spec)
	if err != nil {
		return err
	}

	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if newReplicas == replicas && spec.Replicas == replicas {
		return nil
	}

	if !checkClusterNCompRunning(transCtx, comp.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the component %s is not ready", comp.Name))
	}

	// update cluster spec after the cluster and component are ready
	if !exist {
		for _, tpl := range tpls {
			spec.Instances = append(spec.Instances, *tpl)
		}
		spec.FlatInstanceOrdinal = true
		// retrieve the templates again after the spec is updated
		tpls = replaceCompInstanceTemplatesFromSpec(rollout, spec)
	}

	instance, instTpl, err := t.pickCompInstanceToRollout(transCtx, spec)
	if err != nil {
		return err
	}
	tpl := tpls[""]
	if instTpl != nil {
		tpl = tpls[replaceInstanceTemplateName(*instTpl)]
	}

	if spec.Replicas == replicas {
		return t.compUp(rollout, comp, spec, newReplicas, tpl)
	} else {
		return t.compDown(rollout, comp, spec, newReplicas, instance, instTpl)
	}
}

func (t *rolloutReplaceTransformer) compUp(rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent,
	spec *appsv1.ClusterComponentSpec, newReplicas int32, tpl *appsv1.InstanceTemplate) error {
	if err := t.checkCompDelaySeconds(rollout, comp, newReplicas, false); err != nil {
		return err
	}
	tpl.Replicas = ptr.To(*tpl.Replicas + 1)
	spec.Replicas += 1
	return nil
}

func (t *rolloutReplaceTransformer) compDown(rollout *appsv1alpha1.Rollout, comp appsv1alpha1.RolloutComponent,
	spec *appsv1.ClusterComponentSpec, newReplicas int32, instance string, instTpl *appsv1.InstanceTemplate) error {
	if err := t.checkCompDelaySeconds(rollout, comp, newReplicas, true); err != nil {
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

func (t *rolloutReplaceTransformer) checkCompDelaySeconds(rollout *appsv1alpha1.Rollout,
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

func (t *rolloutReplaceTransformer) pickCompInstanceToRollout(transCtx *rolloutTransformContext,
	spec *appsv1.ClusterComponentSpec) (string, *appsv1.InstanceTemplate, error) {
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
	suffix := replaceInstanceTemplateNameSuffix(transCtx.Rollout)
	for i, pod := range pods.Items {
		if pod.DeletionTimestamp == nil && !strings.HasSuffix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], suffix) {
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

func (t *rolloutReplaceTransformer) shardings(transCtx *rolloutTransformContext) error {
	rollout := transCtx.Rollout
	for _, sharding := range rollout.Spec.Shardings {
		if sharding.Strategy.Replace != nil {
			if err := t.sharding(transCtx, rollout, sharding); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *rolloutReplaceTransformer) sharding(transCtx *rolloutTransformContext,
	rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding) error {
	spec := transCtx.ClusterShardings[sharding.Name]
	replicas := replaceShardingReplicas(rollout, sharding, spec)
	tpls, exist, err := replaceShardingInstanceTemplates(rollout, sharding, spec)
	if err != nil {
		return err
	}

	newReplicas := replaceInstanceTemplateReplicas(tpls)
	if newReplicas == replicas && spec.Template.Replicas == replicas {
		return nil
	}

	if !checkClusterNShardingRunning(transCtx, sharding.Name) {
		return controllerutil.NewDelayedRequeueError(componentNotReadyRequeueDuration, fmt.Sprintf("the sharding %s is not ready", sharding.Name))
	}

	// update cluster spec after the cluster and sharding are ready
	if !exist {
		for _, tpl := range tpls {
			spec.Template.Instances = append(spec.Template.Instances, *tpl)
		}
		spec.Template.FlatInstanceOrdinal = true
		// retrieve the templates again after the spec is updated
		tpls = replaceShardingInstanceTemplatesFromSpec(rollout, spec)
	}

	instance, instTpl, err := t.pickShardingInstancesToRollout(transCtx, spec)
	if err != nil {
		return err
	}
	tpl := tpls[""]
	if instTpl != nil {
		tpl = tpls[replaceInstanceTemplateName(*instTpl)]
	}

	totalReplicas := newReplicas * spec.Shards
	if spec.Template.Replicas == replicas {
		return t.shardingUp(rollout, sharding, spec, totalReplicas, tpl)
	} else {
		return t.shardingDown(rollout, sharding, spec, totalReplicas, instance, instTpl)
	}
}

func (t *rolloutReplaceTransformer) shardingUp(rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding,
	spec *appsv1.ClusterSharding, newReplicas int32, tpl *appsv1.InstanceTemplate) error {
	if err := t.checkShardingDelaySeconds(rollout, sharding, newReplicas, false); err != nil {
		return err
	}
	tpl.Replicas = ptr.To(*tpl.Replicas + 1)
	spec.Template.Replicas += 1
	return nil
}

func (t *rolloutReplaceTransformer) shardingDown(rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding,
	spec *appsv1.ClusterSharding, newReplicas int32, instance string, instTpl *appsv1.InstanceTemplate) error {
	if err := t.checkShardingDelaySeconds(rollout, sharding, newReplicas, true); err != nil {
		return err
	}

	spec.Template.Replicas -= 1
	if instTpl != nil {
		if instTpl.Replicas == nil || *instTpl.Replicas == 0 {
			return fmt.Errorf("the instance template %s still has running instances, but its replicas is already 0", instTpl.Name)
		}
		instTpl.Replicas = ptr.To(*instTpl.Replicas - 1)
	}
	if len(instance) > 0 {
		spec.Template.OfflineInstances = append(spec.Template.OfflineInstances, instance)
	}

	// add the scale down instances to the rollout status
	if len(instance) > 0 {
		for i, status := range rollout.Status.Shardings {
			if status.Name == spec.Name {
				rollout.Status.Shardings[i].ScaleDownInstances = append(rollout.Status.Shardings[i].ScaleDownInstances, instance)
				break
			}
		}
	}

	return nil
}

func (t *rolloutReplaceTransformer) checkShardingDelaySeconds(rollout *appsv1alpha1.Rollout,
	sharding appsv1alpha1.RolloutSharding, newReplicas int32, scaleDown bool) error {
	delaySeconds := sharding.Strategy.Replace.PerInstanceIntervalSeconds
	if scaleDown {
		delaySeconds = sharding.Strategy.Replace.ScaleDownDelaySeconds
	}
	if delaySeconds == nil || *delaySeconds == 0 {
		return nil
	}
	if *delaySeconds < 0 {
		return controllerutil.NewDelayedRequeueError(infiniteDelayRequeueDuration, "infinite delay")
	}

	var lastSucceedTimestamp metav1.Time
	for _, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
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

func (t *rolloutReplaceTransformer) pickShardingInstancesToRollout(transCtx *rolloutTransformContext,
	spec *appsv1.ClusterSharding) (string, *appsv1.InstanceTemplate, error) {
	matchingLabels := constant.GetClusterLabels(transCtx.Cluster.Name, map[string]string{
		constant.KBAppShardingNameLabelKey: spec.Name,
	})
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
	suffix := replaceInstanceTemplateNameSuffix(transCtx.Rollout)
	for i, pod := range pods.Items {
		if pod.DeletionTimestamp == nil && !strings.HasSuffix(pod.Labels[constant.KBAppInstanceTemplateLabelKey], suffix) {
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
	for i, tpl := range spec.Template.Instances {
		if tpl.Name == tplName {
			return targetPod.Name, &spec.Template.Instances[i], nil
		}
	}
	return "", nil, fmt.Errorf("the instance template %s has not been found", tplName)
}

func replaceCompReplicas(rollout *appsv1alpha1.Rollout,
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

func replaceShardingReplicas(rollout *appsv1alpha1.Rollout, sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding) int32 {
	// the original replicas
	replicas := spec.Template.Replicas
	for _, status := range rollout.Status.Shardings {
		if status.Name == sharding.Name {
			if spec.Shards == 0 {
				replicas = 0
			} else {
				replicas = status.Replicas / spec.Shards
			}
			break
		}
	}
	return replicas
}

func replaceCompInstanceTemplates(rollout *appsv1alpha1.Rollout,
	comp appsv1alpha1.RolloutComponent, spec *appsv1.ClusterComponentSpec) (map[string]*appsv1.InstanceTemplate, bool, error) {
	tpls := replaceCompInstanceTemplatesFromSpec(rollout, spec)
	if len(tpls) > 0 {
		return tpls, true, nil
	}

	if len(spec.Instances) > 0 && !spec.FlatInstanceOrdinal {
		return nil, false, fmt.Errorf("not support the replace strategy with the flatInstanceOrdinal is false")
	}

	suffix := replaceInstanceTemplateNameSuffix(rollout)
	tpls[""] = replaceCompInstanceTemplate(comp, suffix, nil)
	for _, tpl := range spec.Instances {
		if ptr.Deref(tpl.Replicas, 0) > 0 {
			tpls[tpl.Name] = replaceCompInstanceTemplate(comp, replaceInstanceTemplateName2(tpl, suffix), &tpl)
		}
	}
	return tpls, false, nil
}

func replaceCompInstanceTemplatesFromSpec(rollout *appsv1alpha1.Rollout, spec *appsv1.ClusterComponentSpec) map[string]*appsv1.InstanceTemplate {
	suffix := replaceInstanceTemplateNameSuffix(rollout)
	tpls := make(map[string]*appsv1.InstanceTemplate)
	for i, tpl := range spec.Instances {
		if strings.HasSuffix(tpl.Name, suffix) {
			if tpl.Name == suffix {
				tpls[""] = &spec.Instances[i]
			} else {
				tpls[strings.TrimSuffix(tpl.Name, fmt.Sprintf("-%s", suffix))] = &spec.Instances[i]
			}
		}
	}
	return tpls
}

func replaceCompInstanceTemplate(comp appsv1alpha1.RolloutComponent, newTplName string, tpl *appsv1.InstanceTemplate) *appsv1.InstanceTemplate {
	newTpl := &appsv1.InstanceTemplate{}
	if tpl != nil {
		newTpl = tpl.DeepCopy()
	}
	newTpl.Name = newTplName
	newTpl.Replicas = ptr.To[int32](0)
	if comp.ServiceVersion != nil {
		newTpl.ServiceVersion = *comp.ServiceVersion
	}
	if comp.CompDef != nil {
		newTpl.CompDef = *comp.CompDef
	}
	if comp.Strategy.Replace.SchedulingPolicy != nil {
		policy := comp.Strategy.Replace.SchedulingPolicy
		newTpl.SchedulingPolicy = &appsv1.SchedulingPolicy{
			SchedulerName:             policy.SchedulerName,
			NodeSelector:              policy.NodeSelector,
			NodeName:                  policy.NodeName,
			Affinity:                  policy.Affinity,
			Tolerations:               policy.Tolerations,
			TopologySpreadConstraints: policy.TopologySpreadConstraints,
		}
	}
	if comp.InstanceMeta != nil && comp.InstanceMeta.Canary != nil {
		maps.Copy(newTpl.Labels, comp.InstanceMeta.Canary.Labels)
		maps.Copy(newTpl.Annotations, comp.InstanceMeta.Canary.Annotations)
	}
	if newTpl.Annotations == nil {
		newTpl.Annotations = map[string]string{}
	}
	newTpl.Annotations[instanceTemplateCreatedByAnnotationKey] = "yes"
	return newTpl
}

func replaceShardingInstanceTemplates(rollout *appsv1alpha1.Rollout,
	sharding appsv1alpha1.RolloutSharding, spec *appsv1.ClusterSharding) (map[string]*appsv1.InstanceTemplate, bool, error) {
	tpls := replaceShardingInstanceTemplatesFromSpec(rollout, spec)
	if len(tpls) > 0 {
		return tpls, true, nil
	}

	if len(spec.Template.Instances) > 0 && !spec.Template.FlatInstanceOrdinal {
		return nil, false, fmt.Errorf("not support the replace strategy with the flatInstanceOrdinal is false")
	}

	suffix := replaceInstanceTemplateNameSuffix(rollout)
	tpls[""] = replaceShardingInstanceTemplate(sharding, suffix, nil)
	for _, tpl := range spec.Template.Instances {
		if ptr.Deref(tpl.Replicas, 0) > 0 {
			tpls[tpl.Name] = replaceShardingInstanceTemplate(sharding, replaceInstanceTemplateName2(tpl, suffix), &tpl)
		}
	}
	return tpls, false, nil
}

func replaceShardingInstanceTemplatesFromSpec(rollout *appsv1alpha1.Rollout, spec *appsv1.ClusterSharding) map[string]*appsv1.InstanceTemplate {
	suffix := replaceInstanceTemplateNameSuffix(rollout)
	tpls := make(map[string]*appsv1.InstanceTemplate)
	for i, tpl := range spec.Template.Instances {
		if strings.HasSuffix(tpl.Name, suffix) {
			if tpl.Name == suffix {
				tpls[""] = &spec.Template.Instances[i]
			} else {
				tpls[strings.TrimSuffix(tpl.Name, fmt.Sprintf("-%s", suffix))] = &spec.Template.Instances[i]
			}
		}
	}
	return tpls
}

func replaceShardingInstanceTemplate(sharding appsv1alpha1.RolloutSharding, newTplName string, tpl *appsv1.InstanceTemplate) *appsv1.InstanceTemplate {
	newTpl := &appsv1.InstanceTemplate{}
	if tpl != nil {
		newTpl = tpl.DeepCopy()
	}
	newTpl.Name = newTplName
	newTpl.Replicas = ptr.To[int32](0)
	if sharding.ServiceVersion != nil {
		newTpl.ServiceVersion = *sharding.ServiceVersion
	}
	if sharding.CompDef != nil {
		newTpl.CompDef = *sharding.CompDef
	}
	if sharding.Strategy.Replace.SchedulingPolicy != nil {
		policy := sharding.Strategy.Replace.SchedulingPolicy
		newTpl.SchedulingPolicy = &appsv1.SchedulingPolicy{
			SchedulerName:             policy.SchedulerName,
			NodeSelector:              policy.NodeSelector,
			NodeName:                  policy.NodeName,
			Affinity:                  policy.Affinity,
			Tolerations:               policy.Tolerations,
			TopologySpreadConstraints: policy.TopologySpreadConstraints,
		}
	}
	if sharding.InstanceMeta != nil && sharding.InstanceMeta.Canary != nil {
		maps.Copy(newTpl.Labels, sharding.InstanceMeta.Canary.Labels)
		maps.Copy(newTpl.Annotations, sharding.InstanceMeta.Canary.Annotations)
	}
	if newTpl.Annotations == nil {
		newTpl.Annotations = map[string]string{}
	}
	newTpl.Annotations[instanceTemplateCreatedByAnnotationKey] = "yes"
	return newTpl
}

func replaceInstanceTemplateNameSuffix(rollout *appsv1alpha1.Rollout) string {
	return string(rollout.UID[:8])
}

func replaceInstanceTemplateName(tpl appsv1.InstanceTemplate) string {
	_, ok := tpl.Annotations[instanceTemplateCreatedByAnnotationKey]
	if !ok {
		return tpl.Name
	}
	subs := strings.Split(tpl.Name, "-")
	if len(subs) == 1 {
		return tpl.Name // TODO: "" or tpl.Name
	}
	return strings.Join(subs[:len(subs)-1], "-")
}

// TODO: name
func replaceInstanceTemplateName2(tpl appsv1.InstanceTemplate, suffix string) string {
	tplName := replaceInstanceTemplateName(tpl)
	if tplName == "" {
		return suffix
	}
	return fmt.Sprintf("%s-%s", tplName, suffix)
}

func replaceInstanceTemplateReplicas(tpls map[string]*appsv1.InstanceTemplate) int32 {
	cnt := int32(0)
	for _, tpl := range tpls {
		cnt += *tpl.Replicas
	}
	return cnt
}
