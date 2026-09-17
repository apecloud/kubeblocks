/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type verticalScalingHandler struct{}

var _ OpsHandler = verticalScalingHandler{}

func init() {
	vsHandler := verticalScalingHandler{}
	verticalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		OpsHandler:        vsHandler,
		QueueByCluster:    true,
		CancelFunc:        vsHandler.Cancel,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// ActionStartedCondition the started condition when handle the vertical scaling request.
func (vs verticalScalingHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewVerticalScalingCondition(opsRes.OpsRequest), nil
}

// Action applies spec.verticalScaling to the requested component and instance-template resources.
func (vs verticalScalingHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	applyVerticalScaling := func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		verticalScaling := obj.(opsv1alpha1.VerticalScaling)
		if vs.verticalScalingComp(verticalScaling) {
			compSpec.Resources = verticalScaling.ResourceRequirements
		}
		for _, v := range verticalScaling.Instances {
			for i := range compSpec.Instances {
				if compSpec.Instances[i].Name == v.Name {
					compSpec.Instances[i].Resources = &v.ResourceRequirements
					break
				}
			}
		}
		return nil
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.VerticalScalingType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			for _, v := range earlierOps.Spec.VerticalScalingList {
				// abort the earlierOps if exists the same component.
				if _, ok := compOpsSet.componentOpsSet[v.ComponentName]; ok {
					return true, nil
				}
			}
			return false, nil
		}); err != nil {
		return err
	}
	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, applyVerticalScaling); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if rollingActionGenerationPending(opsRes) {
		return opsv1alpha1.OpsRunningPhase, 0, nil
	}
	if !vs.targetsMatch(opsRes) {
		return opsv1alpha1.OpsRunningPhase, time.Second, nil
	}

	opsRequest := opsRes.OpsRequest
	oldOpsRequest := opsRequest.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	helper := newComponentOpsHelper(opsRequest.Spec.VerticalScalingList)
	progress, err := vs.observeProgress(reqCtx, cli, opsRes, helper)
	if err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}

	phase := helper.componentActionPhase(opsRes, appsv1.RunningComponentPhase)
	// Apps and workload observations may arrive in different reconciliations. A
	// successful domain result is published only after the current instance
	// progress contains no failures and has converged to the same target.
	if phase == opsv1alpha1.OpsSucceedPhase &&
		(!progress.observationsComplete || progress.failed > 0 || progress.succeeded != progress.expected) {
		phase = opsv1alpha1.OpsRunningPhase
	}
	if err := patchCurrentProgress(reqCtx, cli, opsRes, oldOpsRequest, progress.details,
		progress.succeeded+progress.failed, progress.expected); err != nil {
		return opsv1alpha1.OpsRunningPhase, 0, err
	}
	if phase == opsv1alpha1.OpsRunningPhase {
		return phase, time.Second, nil
	}
	return phase, 0, nil
}

func (vs verticalScalingHandler) verticalScalingComp(verticalScaling opsv1alpha1.VerticalScaling) bool {
	return len(verticalScaling.Requests) != 0 || len(verticalScaling.Limits) != 0
}

func (vs verticalScalingHandler) targetsMatch(opsRes *OpsResource) bool {
	if opsRes == nil || opsRes.Cluster == nil || opsRes.OpsRequest == nil {
		return false
	}
	for i := range opsRes.OpsRequest.Spec.VerticalScalingList {
		verticalScaling := &opsRes.OpsRequest.Spec.VerticalScalingList[i]
		component := getComponentSpecOrShardingTemplate(opsRes.Cluster, verticalScaling.ComponentName)
		if component == nil || !vs.componentTargetMatches(opsRes.OpsRequest, verticalScaling, component) {
			return false
		}
	}
	return true
}

func (vs verticalScalingHandler) componentTargetMatches(ops *opsv1alpha1.OpsRequest,
	verticalScaling *opsv1alpha1.VerticalScaling, component *appsv1.ClusterComponentSpec) bool {
	if verticalScalingIsCancelling(ops) {
		last, ok := ops.Status.LastConfiguration.Components[verticalScaling.ComponentName]
		if !ok {
			return false
		}
		if vs.verticalScalingComp(*verticalScaling) &&
			!equality.Semantic.DeepEqual(component.Resources, last.ResourceRequirements) {
			return false
		}
		for _, requested := range verticalScaling.Instances {
			var previous *corev1.ResourceRequirements
			for i := range last.InstanceTemplates {
				if last.InstanceTemplates[i].Name == requested.Name {
					previous = last.InstanceTemplates[i].Resources
					break
				}
			}
			current, found := findInstanceTemplate(component, requested.Name)
			if !found || !equality.Semantic.DeepEqual(current.Resources, previous) {
				return false
			}
		}
		return true
	}
	if vs.verticalScalingComp(*verticalScaling) &&
		!equality.Semantic.DeepEqual(component.Resources, verticalScaling.ResourceRequirements) {
		return false
	}
	for i := range verticalScaling.Instances {
		requested := &verticalScaling.Instances[i]
		current, found := findInstanceTemplate(component, requested.Name)
		if !found || current.Resources == nil ||
			!equality.Semantic.DeepEqual(*current.Resources, requested.ResourceRequirements) {
			return false
		}
	}
	return true
}

func findInstanceTemplate(component *appsv1.ClusterComponentSpec, name string) (*appsv1.InstanceTemplate, bool) {
	for i := range component.Instances {
		if component.Instances[i].Name == name {
			return &component.Instances[i], true
		}
	}
	return nil, false
}

type verticalScalingProgress struct {
	expected             int32
	succeeded            int32
	failed               int32
	observationsComplete bool
	details              map[string][]opsv1alpha1.ProgressStatusDetail
}

func (vs verticalScalingHandler) observeProgress(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, helper componentOpsHelper) (verticalScalingProgress, error) {
	result := verticalScalingProgress{
		observationsComplete: true,
		details:              map[string][]opsv1alpha1.ProgressStatusDetail{},
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		component := &opsRes.Cluster.Spec.ComponentSpecs[i]
		compOps, ok := helper.getComponentOps(component.Name)
		if !ok {
			continue
		}
		verticalScaling := compOps.(opsv1alpha1.VerticalScaling)
		result.details[component.Name] = nil
		if err := vs.observeInstanceSet(reqCtx, cli, opsRes, component.Name, component.Name,
			verticalScaling, vs.affectedReplicas(component, verticalScaling), &result); err != nil {
			return result, err
		}
	}
	for i := range opsRes.Cluster.Spec.Shardings {
		shardingSpec := &opsRes.Cluster.Spec.Shardings[i]
		compOps, ok := helper.getComponentOps(shardingSpec.Name)
		if !ok {
			continue
		}
		verticalScaling := compOps.(opsv1alpha1.VerticalScaling)
		result.details[shardingSpec.Name] = nil
		fallback := vs.affectedReplicas(&shardingSpec.Template, verticalScaling)
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, shardingSpec.Name)
		if err != nil {
			return result, err
		}
		for j := range components {
			fullName := components[j].Labels[constant.KBAppComponentLabelKey]
			if fullName == "" {
				fullName = components[j].Name
			}
			physicalTarget := appsv1.ClusterComponentSpec{
				Replicas: components[j].Spec.Replicas, Instances: components[j].Spec.Instances,
			}
			if err := vs.observeInstanceSet(reqCtx, cli, opsRes, shardingSpec.Name, fullName,
				verticalScaling, vs.affectedReplicas(&physicalTarget, verticalScaling), &result); err != nil {
				return result, err
			}
		}
		if int32(len(components)) != shardingSpec.Shards {
			result.observationsComplete = false
			if missing := shardingSpec.Shards - int32(len(components)); missing > 0 {
				result.expected += fallback * missing
			}
		}
	}
	return result, nil
}

func (vs verticalScalingHandler) affectedReplicas(component *appsv1.ClusterComponentSpec,
	verticalScaling opsv1alpha1.VerticalScaling) int32 {
	explicit := make(map[string]struct{}, len(verticalScaling.Instances))
	for i := range verticalScaling.Instances {
		explicit[verticalScaling.Instances[i].Name] = struct{}{}
	}
	var selected, templateReplicas int32
	for i := range component.Instances {
		template := &component.Instances[i]
		replicas := template.GetReplicas()
		templateReplicas += replicas
		_, explicitlySelected := explicit[template.Name]
		if explicitlySelected || vs.verticalScalingComp(verticalScaling) && template.Resources == nil {
			selected += replicas
		}
	}
	if vs.verticalScalingComp(verticalScaling) && component.Replicas > templateReplicas {
		selected += component.Replicas - templateReplicas
	}
	return selected
}

func (vs verticalScalingHandler) observeInstanceSet(reqCtx intctrlutil.RequestCtx, cli client.Client,
	opsRes *OpsResource, componentName, fullComponentName string, verticalScaling opsv1alpha1.VerticalScaling,
	missingExpected int32, result *verticalScalingProgress) error {
	its := &workloads.InstanceSet{}
	key := client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
		Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, fullComponentName)}
	if err := cli.Get(reqCtx.Ctx, key, its); err != nil {
		if apierrors.IsNotFound(err) {
			result.expected += missingExpected
			if missingExpected > 0 {
				result.observationsComplete = false
			}
			return nil
		}
		return err
	}
	explicit := make(map[string]struct{}, len(verticalScaling.Instances))
	for i := range verticalScaling.Instances {
		explicit[verticalScaling.Instances[i].Name] = struct{}{}
	}
	expected, selectedTemplates := vs.affectedInstanceSetReplicas(its, verticalScaling, explicit)
	if its.Status.ObservedGeneration != its.Generation {
		result.observationsComplete = false
	}
	var observed int32
	for i := range its.Status.InstanceStatus {
		instance := &its.Status.InstanceStatus[i]
		if instance.EffectiveDesiredState() != workloads.InstanceDesiredStateActive || instance.TemplateName == nil {
			continue
		}
		templateName := *instance.TemplateName
		if _, selected := selectedTemplates[templateName]; !selected {
			continue
		}
		observed++
		objectKey := getProgressObjectKey(constant.PodKind, instance.PodName)
		detail := opsv1alpha1.ProgressStatusDetail{Group: fullComponentName, ObjectKey: objectKey}
		messageKey := "vertical scale"
		if verticalScalingIsCancelling(opsRes.OpsRequest) {
			messageKey += " with rollback"
		}
		targetApplied := instance.EffectiveCurrentState() == workloads.InstanceCurrentStatePresent && instance.UpToDate
		switch {
		case targetApplied && instance.Failed:
			detail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus,
				getProgressFailedMessage(messageKey, objectKey, fullComponentName,
					getFailedPodMessage(opsRes.Cluster, componentName, instance.PodName)))
			result.failed++
		case targetApplied && instance.Ready && instance.Available:
			detail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
				getProgressSucceedMessage(messageKey, objectKey, fullComponentName))
			result.succeeded++
		default:
			detail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus,
				getProgressProcessingMessage(messageKey, objectKey, fullComponentName))
		}
		result.details[componentName] = append(result.details[componentName], detail)
	}
	result.expected += max(expected, observed)
	if observed != expected {
		result.observationsComplete = false
	}
	return nil
}

func (vs verticalScalingHandler) affectedInstanceSetReplicas(its *workloads.InstanceSet,
	verticalScaling opsv1alpha1.VerticalScaling, explicit map[string]struct{}) (int32, map[string]struct{}) {
	total := int32(1)
	if its.Spec.Replicas != nil {
		total = *its.Spec.Replicas
	}
	defaultReplicas := total
	var selected int32
	selectedTemplates := map[string]struct{}{}
	for i := range its.Spec.Instances {
		template := &its.Spec.Instances[i]
		replicas := int32(1)
		if template.Replicas != nil {
			replicas = *template.Replicas
		}
		defaultReplicas -= replicas
		_, explicitlySelected := explicit[template.Name]
		if explicitlySelected || vs.verticalScalingComp(verticalScaling) && template.Resources == nil {
			selected += replicas
			selectedTemplates[template.Name] = struct{}{}
		}
	}
	if vs.verticalScalingComp(verticalScaling) && defaultReplicas > 0 {
		selected += defaultReplicas
		selectedTemplates[""] = struct{}{}
	}
	return selected, selectedTemplates
}

func verticalScalingIsCancelling(ops *opsv1alpha1.OpsRequest) bool {
	return ops != nil && (ops.Spec.Cancel || ops.Status.Phase == opsv1alpha1.OpsCancellingPhase)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (vs verticalScalingHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		verticalScaling := comOps.(opsv1alpha1.VerticalScaling)
		var instanceTemplates []appsv1.InstanceTemplate
		for _, vIns := range verticalScaling.Instances {
			for _, compIns := range compSpec.Instances {
				if vIns.Name != compIns.Name {
					continue
				}
				instanceTemplates = append(instanceTemplates, appsv1.InstanceTemplate{
					Name:      compIns.Name,
					Resources: compIns.Resources,
				})
				break
			}
		}
		return opsv1alpha1.LastComponentConfiguration{
			ResourceRequirements: compSpec.Resources,
			InstanceTemplates:    instanceTemplates,
		}
	})
	return nil
}

// Cancel this function defines the cancel verticalScaling action.
func (vs verticalScalingHandler) Cancel(reqCxt intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	return compOpsHelper.cancelComponentOps(reqCxt.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Resources = lastConfig.ResourceRequirements
		for _, lastIns := range lastConfig.InstanceTemplates {
			for i := range comp.Instances {
				if comp.Instances[i].Name != lastIns.Name {
					continue
				}
				comp.Instances[i].Resources = lastIns.Resources
				break
			}
		}
	})
}
