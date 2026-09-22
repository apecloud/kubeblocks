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
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type volumeExpansionOpsHandler struct {
}

var _ OpsHandler = volumeExpansionOpsHandler{}

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	// Expansion follows the current target and owner instance observations.
	volumeExpansionBehaviour := OpsBehaviour{
		OpsHandler:  volumeExpansionOpsHandler{},
		QueueBySelf: true,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// ActionStartedCondition the started condition when handle the volume expansion request.
func (ve volumeExpansionOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewVolumeExpandingCondition(opsRes.OpsRequest), nil
}

// Action modifies the selected component and instance-template VCTs in Cluster.spec.
func (ve volumeExpansionOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	applyVolumeExpansion := func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		setStorage := func(requested resource.Quantity, requests *corev1.ResourceList) {
			if *requests == nil {
				*requests = corev1.ResourceList{}
			}
			(*requests)[corev1.ResourceStorage] = requested
		}
		volumeExpansion := obj.(opsv1alpha1.VolumeExpansion)
		if _, err := opsv1alpha1.NormalizeVolumeExpansion(volumeExpansion, compSpec); err != nil {
			return err
		}
		for _, v := range volumeExpansion.VolumeClaimTemplates {
			for i := range compSpec.VolumeClaimTemplates {
				if compSpec.VolumeClaimTemplates[i].Name == v.Name {
					setStorage(v.Storage, &compSpec.VolumeClaimTemplates[i].Spec.Resources.Requests)
				}
			}
		}
		for _, instanceRequest := range volumeExpansion.Instances {
			for instanceIndex := range compSpec.Instances {
				instance := &compSpec.Instances[instanceIndex]
				if instance.Name != instanceRequest.Name {
					continue
				}
				for _, requested := range instanceRequest.VolumeClaimTemplates {
					found := false
					for vctIndex := range instance.VolumeClaimTemplates {
						if instance.VolumeClaimTemplates[vctIndex].Name != requested.Name {
							continue
						}
						setStorage(requested.Storage, &instance.VolumeClaimTemplates[vctIndex].Spec.Resources.Requests)
						found = true
					}
					if found {
						continue
					}
					for _, componentVCT := range compSpec.VolumeClaimTemplates {
						if componentVCT.Name != requested.Name {
							continue
						}
						copy := *componentVCT.DeepCopy()
						setStorage(requested.Storage, &copy.Spec.Resources.Requests)
						instance.VolumeClaimTemplates = append(instance.VolumeClaimTemplates, copy)
						break
					}
				}
			}
		}
		return nil
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.VolumeExpansionList)
	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, applyVolumeExpansion); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (ve volumeExpansionOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	ops := opsRes.OpsRequest
	oldOps := ops.DeepCopy()
	if ops.Status.Components == nil {
		ve.initComponentStatus(ops)
	}
	helper := newComponentOpsHelper(ops.Spec.VolumeExpansionList)
	current := helper.emptyInstanceProgress(opsRes)
	complete := opsRes.Cluster.Generation >= ops.Status.ClusterGeneration
	var expected, succeeded int32
	observe := func(name, physicalName string, spec *appsv1.ClusterComponentSpec, targets map[string]resource.Quantity, instanceTargets map[string]map[string]resource.Quantity) error {
		its := &workloads.InstanceSet{}
		err := cli.Get(reqCtx.Ctx, client.ObjectKey{Namespace: opsRes.Cluster.Namespace,
			Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, physicalName)}, its)
		if apierrors.IsNotFound(err) {
			complete = false
			return nil
		}
		if err != nil {
			return err
		}
		progress := volumeExpansionProgress(its, spec, targets, instanceTargets)
		current[name] = append(current[name], progress.details...)
		expected += progress.expectedCount
		succeeded += progress.succeededCount
		complete = complete && progress.observationsComplete && progress.succeededCount == progress.expectedCount
		return nil
	}
	for _, request := range ops.Spec.VolumeExpansionList {
		name := request.ComponentName
		current[name] = nil
		spec := opsRes.Cluster.Spec.GetComponentByName(name)
		shardSpec := opsRes.Cluster.Spec.GetShardingByName(name)
		var status appsv1.ClusterComponentStatus
		if shardSpec != nil {
			spec = &shardSpec.Template
			shardStatus := opsRes.Cluster.Status.Shardings[name]
			status = appsv1.ClusterComponentStatus{Phase: shardStatus.Phase, ObservedGeneration: shardStatus.ObservedGeneration, UpToDate: shardStatus.UpToDate}
		} else {
			status = opsRes.Cluster.Status.Components[name]
		}
		componentStatus := ops.Status.Components[name]
		componentStatus.Phase = status.Phase
		ops.Status.Components[name] = componentStatus
		if spec == nil {
			complete = false
			continue
		}
		targets, instanceTargets, targetObserved := volumeExpansionTargets(spec, request)
		complete = complete && targetObserved && status.ObservedGeneration == opsRes.Cluster.Generation && status.UpToDate
		if shardSpec == nil {
			if err := observe(name, name, spec, targets, instanceTargets); err != nil {
				return opsv1alpha1.OpsRunningPhase, time.Minute, err
			}
			continue
		}
		components, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, name)
		if err != nil {
			return opsv1alpha1.OpsRunningPhase, time.Minute, err
		}
		complete = complete && int32(len(components)) == shardSpec.Shards
		eligible := false
		for _, comp := range components {
			if slices.ContainsFunc(shardSpec.ShardTemplates, func(t appsv1.ShardTemplate) bool {
				return t.Name == comp.Labels[constant.KBAppShardTemplateLabelKey] && t.VolumeClaimTemplates != nil
			}) {
				continue
			}
			eligible = true
			physical := &appsv1.ClusterComponentSpec{Replicas: comp.Spec.Replicas, Instances: comp.Spec.Instances,
				Stop: comp.Spec.Stop, OfflineInstances: comp.Spec.OfflineInstances}
			if err := observe(name, comp.Labels[constant.KBAppComponentLabelKey], physical, targets, instanceTargets); err != nil {
				return opsv1alpha1.OpsRunningPhase, time.Minute, err
			}
		}
		if !eligible {
			complete = false
		}
	}
	if err := patchCurrentProgress(reqCtx, cli, opsRes, oldOps, current, succeeded, expected); err != nil {
		return opsv1alpha1.OpsRunningPhase, time.Minute, err
	}
	if complete {
		return opsv1alpha1.OpsSucceedPhase, 0, nil
	}
	if time.Now().After(ops.Status.StartTimestamp.Add(VolumeExpansionTimeOut)) {
		return opsv1alpha1.OpsFailedPhase, time.Minute, errors.New(fmt.Sprintf(
			"Timed out waiting for volume expansion to complete, the timeout value is %g minutes", VolumeExpansionTimeOut.Minutes()))
	}
	return opsv1alpha1.OpsRunningPhase, time.Minute, nil
}

// volumeExpansionTargets keeps the current logical target, including a later compatible expansion.
func volumeExpansionTargets(spec *appsv1.ClusterComponentSpec, request opsv1alpha1.VolumeExpansion) (map[string]resource.Quantity, map[string]map[string]resource.Quantity, bool) {
	plan, err := opsv1alpha1.NormalizeVolumeExpansion(request, spec)
	if err != nil {
		return map[string]resource.Quantity{}, map[string]map[string]resource.Quantity{}, false
	}
	targets := map[string]resource.Quantity{}
	instanceTargets := map[string]map[string]resource.Quantity{}
	for key := range plan.Targets {
		if key.InstanceTemplateName == "" {
			for _, vct := range spec.VolumeClaimTemplates {
				if vct.Name == key.VCTName {
					targets[key.VCTName] = vct.Spec.Resources.Requests[corev1.ResourceStorage]
				}
			}
			continue
		}
		if instanceTargets[key.InstanceTemplateName] == nil {
			instanceTargets[key.InstanceTemplateName] = map[string]resource.Quantity{}
		}
		if vct, ok := opsv1alpha1.EffectiveVolumeClaimTemplate(spec, key.InstanceTemplateName, key.VCTName); ok {
			instanceTargets[key.InstanceTemplateName][key.VCTName] = vct.Spec.Resources.Requests[corev1.ResourceStorage]
		}
	}
	return targets, instanceTargets, len(plan.Targets) == len(request.VolumeClaimTemplates)+countInstanceVolumeTargets(request)
}

func countInstanceVolumeTargets(request opsv1alpha1.VolumeExpansion) int {
	count := 0
	for _, instance := range request.Instances {
		count += len(instance.VolumeClaimTemplates)
	}
	return count
}

// volumeExpansionProgress combines volume observations per instance, retaining volume count units.
// UpToDate is the owner's application/capacity judgment; service health is outside expansion.
func volumeExpansionProgress(its *workloads.InstanceSet, spec *appsv1.ClusterComponentSpec, targets map[string]resource.Quantity, instanceTargets map[string]map[string]resource.Quantity) instanceProgress {
	result := instanceProgress{observationsComplete: its.Status.ObservedGeneration == its.Generation &&
		ptr.Deref(its.Spec.Replicas, int32(1)) == spec.Replicas &&
		ptr.Deref(its.Spec.Stop, false) == ptr.Deref(spec.Stop, false)}
	templateReplicas, valid := expectedTemplateReplicas(spec)
	result.observationsComplete = result.observationsComplete && valid
	affected := map[string][]string{}
	forwardedTargets := map[string]map[string]bool{}
	getForwarded := func(templateName, name string, storage resource.Quantity) bool {
		if templateName == "" {
			return slices.ContainsFunc(its.Spec.VolumeClaimTemplates, func(v corev1.PersistentVolumeClaim) bool {
				capacity := v.Spec.Resources.Requests[corev1.ResourceStorage]
				return v.Name == name && capacity.Cmp(storage) >= 0
			})
		}
		return slices.ContainsFunc(its.Spec.Instances, func(instance workloads.InstanceTemplate) bool {
			return instance.Name == templateName && slices.ContainsFunc(instance.VolumeClaimTemplates, func(v corev1.PersistentVolumeClaim) bool {
				capacity := v.Spec.Resources.Requests[corev1.ResourceStorage]
				return v.Name == name && capacity.Cmp(storage) >= 0
			})
		})
	}
	for name, storage := range targets {
		forwarded := getForwarded("", name, storage)
		result.observationsComplete = result.observationsComplete && forwarded
	}
	for templateName, templateTargets := range instanceTargets {
		for name, storage := range templateTargets {
			forwarded := getForwarded(templateName, name, storage)
			result.observationsComplete = result.observationsComplete && forwarded
		}
	}
	for templateName, replicas := range templateReplicas {
		volumeTargets := map[string]resource.Quantity{}
		for name, storage := range targets {
			overridden := slices.ContainsFunc(spec.Instances, func(t appsv1.InstanceTemplate) bool {
				return t.Name == templateName && slices.ContainsFunc(t.VolumeClaimTemplates, func(v appsv1.PersistentVolumeClaimTemplate) bool { return v.Name == name })
			})
			forwardedOverride := false
			for _, t := range its.Spec.Instances {
				if t.Name == templateName {
					forwardedOverride = slices.ContainsFunc(t.VolumeClaimTemplates, func(v corev1.PersistentVolumeClaim) bool { return v.Name == name })
				}
			}
			result.observationsComplete = result.observationsComplete && overridden == forwardedOverride
			if !overridden {
				volumeTargets[name] = storage
				if forwardedTargets[templateName] == nil {
					forwardedTargets[templateName] = map[string]bool{}
				}
				forwardedTargets[templateName][name] = slices.ContainsFunc(its.Spec.VolumeClaimTemplates, func(v corev1.PersistentVolumeClaim) bool {
					capacity := v.Spec.Resources.Requests[corev1.ResourceStorage]
					return v.Name == name && capacity.Cmp(storage) >= 0
				})
				result.observationsComplete = result.observationsComplete && forwardedTargets[templateName][name]
			}
		}
		for name, storage := range instanceTargets[templateName] {
			volumeTargets[name] = storage
			if forwardedTargets[templateName] == nil {
				forwardedTargets[templateName] = map[string]bool{}
			}
			forwardedTargets[templateName][name] = getForwarded(templateName, name, storage)
			result.observationsComplete = result.observationsComplete && forwardedTargets[templateName][name]
		}
		for name := range volumeTargets {
			affected[templateName] = append(affected[templateName], name)
			result.expectedCount += replicas
		}
		slices.Sort(affected[templateName])
	}
	// Zero work follows observed allocation declarations, never an empty online status list.
	itsSpec := &appsv1.ClusterComponentSpec{Replicas: ptr.Deref(its.Spec.Replicas, int32(1))}
	for _, t := range its.Spec.Instances {
		itsSpec.Instances = append(itsSpec.Instances, appsv1.InstanceTemplate{Name: t.Name, Replicas: t.Replicas})
	}
	itsReplicas, itsValid := expectedTemplateReplicas(itsSpec)
	result.observationsComplete = result.observationsComplete && itsValid && reflect.DeepEqual(templateReplicas, itsReplicas)
	if result.expectedCount == 0 {
		state := workloads.InstanceDesiredStateActive
		if ptr.Deref(its.Spec.Stop, false) {
			state = workloads.InstanceDesiredStateOffline
		}
		allocated, err := instanceTemplatesByState(its.Status.InstanceStatus, state, func(s workloads.InstanceStatus) bool {
			return !slices.Contains(spec.OfflineInstances, s.PodName)
		})
		result.observationsComplete = result.observationsComplete && err == nil && assignmentsMatchComponent(allocated, spec)
		return result
	}
	active, err := instanceTemplatesByState(its.Status.InstanceStatus, workloads.InstanceDesiredStateActive, func(s workloads.InstanceStatus) bool {
		return s.TemplateName == nil || len(affected[*s.TemplateName]) > 0
	})
	actualReplicas := map[string]int32{}
	for _, template := range active {
		actualReplicas[template]++
	}
	for template, volumes := range affected {
		if len(volumes) > 0 {
			result.observationsComplete = result.observationsComplete && actualReplicas[template] == templateReplicas[template]
		}
	}
	result.observationsComplete = result.observationsComplete && err == nil &&
		!ptr.Deref(its.Spec.Stop, false) && !ptr.Deref(spec.Stop, false)
	for _, instance := range its.Status.InstanceStatus {
		if instance.EffectiveDesiredState() != workloads.InstanceDesiredStateActive || instance.TemplateName == nil {
			continue
		}
		volumes := affected[*instance.TemplateName]
		if len(volumes) == 0 {
			continue
		}
		detail := opsv1alpha1.ProgressStatusDetail{ObjectKey: getProgressObjectKey(constant.PodKind, instance.PodName)}
		if instance.EffectiveCurrentState() == workloads.InstanceCurrentStatePresent && instance.UpToDate {
			forwarded := 0
			for _, name := range volumes {
				if forwardedTargets[*instance.TemplateName][name] {
					forwarded++
				}
			}
			result.succeededCount += int32(forwarded)
			if forwarded == len(volumes) {
				detail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus, fmt.Sprintf("Volumes %s applied to instance %s", strings.Join(volumes, ", "), instance.PodName))
			}
		}
		if detail.Status == "" {
			detail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus, fmt.Sprintf("Waiting for volumes %s on instance %s", strings.Join(volumes, ", "), instance.PodName))
		}
		result.details = append(result.details, detail)
	}
	return result
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (ve volumeExpansionOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	compOpsHelper := newComponentOpsHelper(opsRequest.Spec.VolumeExpansionList)
	storageMap := ve.getRequestStorageMap(opsRequest)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		getLastVCTs := func(vcts []appsv1.PersistentVolumeClaimTemplate) []appsv1.PersistentVolumeClaimTemplate {
			lastVCTs := make([]appsv1.PersistentVolumeClaimTemplate, 0)
			for _, vct := range vcts {
				key := getComponentVCTKey(comOps.GetComponentName(), vct.Name)
				if _, ok := storageMap[key]; !ok {
					continue
				}
				lastVCTs = append(lastVCTs, vct)
			}
			return lastVCTs
		}
		// save the last vcts of the componnet
		lastVCTS := getLastVCTs(compSpec.VolumeClaimTemplates)
		var convertedLastVCTs []opsv1alpha1.OpsRequestVolumeClaimTemplate
		for _, v := range lastVCTS {
			convertedLastVCTs = append(convertedLastVCTs, opsv1alpha1.OpsRequestVolumeClaimTemplate{
				Name:    v.Name,
				Storage: v.Spec.Resources.Requests[corev1.ResourceStorage],
			})
		}
		return opsv1alpha1.LastComponentConfiguration{
			VolumeClaimTemplates: convertedLastVCTs,
		}
	})
	return nil
}

func (ve volumeExpansionOpsHandler) getRequestStorageMap(opsRequest *opsv1alpha1.OpsRequest) map[string]resource.Quantity {
	storageMap := map[string]resource.Quantity{}
	setStorageMap := func(vct opsv1alpha1.OpsRequestVolumeClaimTemplate, compOps opsv1alpha1.ComponentOps) {
		key := getComponentVCTKey(compOps.GetComponentName(), vct.Name)
		storageMap[key] = vct.Storage
	}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		for _, vct := range v.VolumeClaimTemplates {
			setStorageMap(vct, v.ComponentOps)
		}
	}
	return storageMap
}

// initComponentStatus inits status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansionOpsHandler) initComponentStatus(opsRequest *opsv1alpha1.OpsRequest) {
	opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		opsRequest.Status.Components[v.ComponentName] = opsv1alpha1.OpsRequestComponentStatus{}
	}
}

func getComponentVCTKey(compoName, vctName string) string {
	return fmt.Sprintf("%s.%s", compoName, vctName)
}
