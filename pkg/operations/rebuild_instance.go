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
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

const (
	scalingOutPodPrefixMsg = "Scaling out a new pod"
)

type rebuildInstanceWrapper struct {
	replicas int32
	insNames []string
}

// rebuildInstanceOpsHandler is intentionally not covered by OpsRuntime.
// Rebuild still depends on direct Pod/PVC/PV/InstanceSet actions in the standard path.
type rebuildInstanceOpsHandler struct{}

var _ OpsHandler = rebuildInstanceOpsHandler{}

func init() {
	rebuildInstanceBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1.ClusterPhase{appsv1.AbnormalClusterPhase, appsv1.FailedClusterPhase, appsv1.UpdatingClusterPhase},
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        rebuildInstanceOpsHandler{},
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.RebuildInstanceType, rebuildInstanceBehaviour)
}

// ActionStartedCondition the started condition when handle the rebuild-instance request.
func (r rebuildInstanceOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewInstancesRebuildingCondition(opsRes.OpsRequest), nil
}

func (r rebuildInstanceOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if err := r.validatePlacement(opsRes); err != nil {
		return err
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compPhase := r.getCompStatusFromCluster(opsRes, v.ComponentName)
		if compPhase == nil {
			continue
		}
		// check if the component has matched the `Phase` condition
		if !opsRes.OpsRequest.Spec.Force && !slices.Contains([]appsv1.ComponentPhase{appsv1.FailedComponentPhase, appsv1.UpdatingComponentPhase}, *compPhase) {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the phase of component "%s" can not be %s`, v.ComponentName, *compPhase))
		}
		var (
			synthesizedComp *component.SynthesizedComponent
			err             error
			instanceNames   []string
		)
		for _, ins := range v.Instances {
			runtime, err := opsRes.GetRuntime(v.ComponentName)
			if err != nil {
				return err
			}
			targetInstance, err := runtime.GetInstance(opsRes.Cluster.Namespace, opsRes.Cluster.Name, v.ComponentName, ins.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// neither a Pod nor a retained PVC exists for this name, so
					// retrying the same input can never converge
					return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" not found`, ins.Name))
				}
				return err
			}
			synthesizedComp, err = r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, targetInstance.GetComponentName())
			if err != nil {
				return err
			}
			roleAware := len(synthesizedComp.Roles) > 0
			if !opsRes.OpsRequest.Spec.Force && targetInstance.IsAvailable(synthesizedComp.MinReadySeconds, roleAware) {
				return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" is available, can not rebuild it`, ins.Name))
			}
			instanceNames = append(instanceNames, ins.Name)
		}
		if len(v.Instances) > 0 && !v.InPlace {
			if synthesizedComp.Name != v.ComponentName {
				return intctrlutil.NewFatalError("sharding cluster only supports to rebuild instance in place")
			}
			// validate when rebuilding instance with horizontal scaling
			if err = r.validateRebuildInstanceWithHScale(reqCtx, cli, opsRes, v.ComponentName, synthesizedComp, instanceNames); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r rebuildInstanceOpsHandler) getCompStatusFromCluster(opsRes *OpsResource, compName string) *appsv1.ComponentPhase {
	if compStatus, exist := opsRes.Cluster.Status.Components[compName]; exist {
		return &compStatus.Phase
	}
	if shardStatus, exist := opsRes.Cluster.Status.Shardings[compName]; exist {
		return &shardStatus.Phase
	}
	return nil
}

func (r rebuildInstanceOpsHandler) validateRebuildInstanceWithHScale(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	componentName string,
	synthesizedComp *component.SynthesizedComponent,
	instanceNames []string) error {
	// rebuild instance by horizontal scaling
	runtime, err := opsRes.GetRuntime(componentName)
	if err != nil {
		return err
	}
	instances, err := runtime.ListInstances(opsRes.Cluster.Namespace, opsRes.Cluster.Name, synthesizedComp.Name)
	if err != nil {
		return err
	}
	roleAware := len(synthesizedComp.Roles) > 0
	for _, instance := range instances {
		if slices.Contains(instanceNames, instance.GetName()) {
			continue
		}
		if instance.IsAvailable(synthesizedComp.MinReadySeconds, roleAware) {
			return nil
		}
	}
	return intctrlutil.NewFatalError("Due to insufficient available instances, cannot create a new pod for rebuilding instance. " +
		"may you can rebuild instances in place with backup by set 'inPlace' to 'true'.")
}

func (r rebuildInstanceOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if err := r.validatePlacement(opsRes); err != nil {
		return err
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RebuildFrom)
	getLastComponentInfo := func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		lastCompConfiguration := opsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
		return lastCompConfiguration
	}
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	for _, rebuild := range opsRes.OpsRequest.Spec.RebuildFrom {
		if rebuild.InPlace {
			continue
		}
		comp := opsRes.Cluster.Spec.GetComponentByName(rebuild.ComponentName)
		if comp == nil {
			return intctrlutil.NewFatalError("sharding cluster only supports to rebuild instance in place")
		}
		runtime, err := opsRes.GetRuntime(comp.Name)
		if err != nil {
			return err
		}
		workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, comp.Name)
		if err != nil {
			return err
		}
		active, complete, err := activeAssignmentsForTarget(workload, comp)
		if err != nil {
			return err
		}
		if !complete {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for the source instance allocation of component %q", comp.Name)
		}
		for _, instance := range rebuild.Instances {
			if _, ok := active[instance.Name]; !ok {
				return intctrlutil.NewFatalError(fmt.Sprintf("rebuild instance %q is not in the Active allocation", instance.Name))
			}
		}
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[comp.Name]
		for _, name := range slices.Sorted(maps.Keys(active)) {
			last.SourceInstanceAssignments = append(last.SourceInstanceAssignments, opsv1alpha1.InstanceTemplateAssignment{
				WorkloadName: constant.GenerateClusterComponentName(opsRes.Cluster.Name, comp.Name),
				PodName:      name, TemplateName: active[name], DesiredState: workloads.InstanceDesiredStateActive,
			})
		}
		opsRes.OpsRequest.Status.LastConfiguration.Components[comp.Name] = last
	}
	return nil
}

// Placement must be written before the replacement is created. Only the non-flat
// naming scheme can supply that name before InstanceSet allocates it.
func (r rebuildInstanceOpsHandler) validatePlacement(opsRes *OpsResource) error {
	for _, rebuild := range opsRes.OpsRequest.Spec.RebuildFrom {
		comp := opsRes.Cluster.Spec.GetComponentByName(rebuild.ComponentName)
		if !rebuild.InPlace && comp != nil && comp.FlatInstanceOrdinal && rebuildNeedsPlacement(rebuild) {
			return intctrlutil.NewFatalError("non-in-place rebuild with targetNodeName is not supported for flat instance ordinals")
		}
	}
	return nil
}

func rebuildNeedsPlacement(rebuild opsv1alpha1.RebuildInstance) bool {
	return slices.ContainsFunc(rebuild.Instances, func(instance opsv1alpha1.Instance) bool { return instance.TargetNodeName != "" })
}

func (r rebuildInstanceOpsHandler) getInstanceProgressDetail(compStatus opsv1alpha1.OpsRequestComponentStatus, instance string) opsv1alpha1.ProgressStatusDetail {
	objectKey := getProgressObjectKey(constant.PodKind, instance)
	progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
	if progressDetail != nil {
		return *progressDetail
	}
	return opsv1alpha1.ProgressStatusDetail{
		ObjectKey: objectKey,
		Status:    opsv1alpha1.ProcessingProgressStatus,
		Message:   fmt.Sprintf("Start to rebuild pod %s", instance),
	}
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r rebuildInstanceOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		oldOpsRequest   = opsRes.OpsRequest.DeepCopy()
		oldCluster      = opsRes.Cluster.DeepCopy()
		opsRequestPhase = opsRes.OpsRequest.Status.Phase
		expectCount     int
		completedCount  int
		failedCount     int
		err             error
	)
	if opsRes.OpsRequest.Status.Components == nil {
		opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	}
	for _, v := range opsRes.OpsRequest.Spec.RebuildFrom {
		compStatus := opsRes.OpsRequest.Status.Components[v.ComponentName]
		var (
			subCompletedCount int
			subFailedCount    int
		)
		if v.InPlace {
			// rebuild instances in place.
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesInPlace(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				return opsRequestPhase, 0, err
			}
		} else {
			// rebuild instances with horizontal scaling
			if subCompletedCount, subFailedCount, err = r.rebuildInstancesWithHScaling(reqCtx, cli, opsRes, v, &compStatus); err != nil {
				if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
					return opsv1alpha1.OpsFailedPhase, 0, err
				}
				return opsRequestPhase, 0, err
			}
		}
		expectCount += len(v.Instances)
		completedCount += subCompletedCount
		failedCount += subFailedCount
		opsRes.OpsRequest.Status.Components[v.ComponentName] = compStatus
	}
	if !reflect.DeepEqual(oldCluster.Spec, opsRes.Cluster.Spec) {
		if err = cli.Update(reqCtx.Ctx, opsRes.Cluster); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if err = syncProgressToOpsRequest(reqCtx, cli, opsRes, oldOpsRequest, completedCount, expectCount); err != nil {
		return opsRequestPhase, 0, err
	}
	// check if the ops has been finished.
	if completedCount != expectCount {
		return opsRequestPhase, 0, nil
	}
	if failedCount == 0 {
		return opsv1alpha1.OpsSucceedPhase, 0, r.cleanupTmpResources(reqCtx, cli, opsRes)
	}
	return opsv1alpha1.OpsFailedPhase, 0, nil
}

func (r rebuildInstanceOpsHandler) rebuildInstancesInPlace(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	// rebuild instances in place.
	var (
		completedCount int
		failedCount    int
	)
	for i, instance := range rebuildInstance.Instances {
		progressDetail := r.getInstanceProgressDetail(*compStatus, instance.Name)
		if isCompletedProgressStatus(progressDetail.Status) {
			completedCount += 1
			if progressDetail.Status == opsv1alpha1.FailedProgressStatus {
				failedCount += 1
			}
			continue
		}
		// rebuild instance
		completed, err := r.rebuildInstanceInPlace(reqCtx, cli, opsRes, &progressDetail, rebuildInstance, instance, i)
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
			// If a fatal error occurs, this instance rebuilds failed.
			progressDetail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus, err.Error())
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if err != nil {
			return 0, 0, err
		}
		if completed {
			// if the pod has been rebuilt, set progressDetail phase to Succeed.
			progressDetail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
				fmt.Sprintf("Rebuild pod %s successfully", instance.Name))
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
	}
	return completedCount, failedCount, nil
}

// rebuildInstance rebuilds the instance.
func (r rebuildInstanceOpsHandler) rebuildInstanceInPlace(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	progressDetail *opsv1alpha1.ProgressStatusDetail,
	rebuildFrom opsv1alpha1.RebuildInstance,
	instance opsv1alpha1.Instance,
	index int) (bool, error) {
	inPlaceHelper, err := r.prepareInplaceRebuildHelper(reqCtx, cli, opsRes, rebuildFrom, instance, index)
	if err != nil {
		return false, err
	}

	if rebuildFrom.BackupName == "" {
		return inPlaceHelper.rebuildInstanceWithNoBackup(reqCtx, cli, opsRes, progressDetail)
	}
	return inPlaceHelper.rebuildInstanceWithBackup(reqCtx, cli, opsRes, progressDetail)
}

func (r rebuildInstanceOpsHandler) rebuildInstancesWithHScaling(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, error) {
	var (
		completedCount int
		failedCount    int
		err            error
	)
	if len(compStatus.ProgressDetails) == 0 {
		// 1. scale out the required instances
		err := r.scaleOutRequiredInstances(reqCtx, cli, opsRes, rebuildInstance, compStatus)
		return 0, 0, err
	}
	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name != rebuildInstance.ComponentName {
			continue
		}
		// 2. check if the new pods are available.
		var instancesNeedToOffline []string
		if completedCount, failedCount, instancesNeedToOffline, err = r.checkProgressForScalingOutPods(reqCtx,
			cli, opsRes, rebuildInstance, compSpec, compStatus); err != nil {
			return 0, 0, err
		}

		if len(instancesNeedToOffline) > 0 {
			// 3. offline the instances that require rebuilding when the new pod successfully scales out.
			if err := r.offlineSpecifiedInstances(opsRes, compSpec, instancesNeedToOffline); err != nil {
				return 0, 0, err
			}
		}
		break
	}
	return completedCount, failedCount, nil
}

func (r rebuildInstanceOpsHandler) scaleOutRequiredInstances(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) error {
	// Assemble the corresponding replicas and instances based on the saved templates.
	rebuildInsWrapper, err := r.getRebuildInstanceWrapper(opsRes, rebuildInstance)
	if err != nil {
		return err
	}

	compName := rebuildInstance.ComponentName

	for i := range opsRes.Cluster.Spec.ComponentSpecs {
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[i]
		if compSpec.Name != compName {
			continue
		}
		if !rebuildNeedsPlacement(rebuildInstance) {
			return r.scaleOutAllocatedInstances(opsRes, compSpec, rebuildInstance, compStatus, rebuildInsWrapper)
		}
		return r.scaleOutWithPlacement(reqCtx, cli, opsRes, compSpec, rebuildInstance, compStatus, rebuildInsWrapper)
	}
	return nil
}

// getRebuildInstanceWrapper assembles the corresponding replicas and instances based on the template
func (r rebuildInstanceOpsHandler) getRebuildInstanceWrapper(opsRes *OpsResource, rebuildInstance opsv1alpha1.RebuildInstance) (map[string]*rebuildInstanceWrapper, error) {
	rebuildInsWrapper := map[string]*rebuildInstanceWrapper{}
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[rebuildInstance.ComponentName]
	source := sourceAssignmentsForWorkload(last, constant.GenerateClusterComponentName(opsRes.Cluster.Name, rebuildInstance.ComponentName))
	for _, ins := range rebuildInstance.Instances {
		insTplName, ok := source[ins.Name]
		if !ok {
			return nil, fmt.Errorf("missing source assignment for rebuild instance %q", ins.Name)
		}
		if _, ok := rebuildInsWrapper[insTplName]; !ok {
			rebuildInsWrapper[insTplName] = &rebuildInstanceWrapper{replicas: 1, insNames: []string{ins.Name}}
		} else {
			rebuildInsWrapper[insTplName].replicas += 1
			rebuildInsWrapper[insTplName].insNames = append(rebuildInsWrapper[insTplName].insNames, ins.Name)
		}
	}
	return rebuildInsWrapper, nil
}

// rebuildExpansion derives the target from saved configuration, never from a
// possibly already-expanded live spec. This also recovers a lost progress patch.
func rebuildExpansion(opsRes *OpsResource, comp *appsv1.ClusterComponentSpec,
	wrappers map[string]*rebuildInstanceWrapper) (*appsv1.ClusterComponentSpec, error) {
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[comp.Name]
	if last.Replicas == nil {
		return nil, fmt.Errorf("missing source configuration for component %q", comp.Name)
	}
	source := comp.DeepCopy()
	source.Replicas, source.Instances, source.OfflineInstances = *last.Replicas, last.Instances, last.OfflineInstances
	target := source.DeepCopy()
	for _, wrapper := range wrappers {
		target.Replicas += wrapper.replicas
	}
	for i := range target.Instances {
		if wrapper := wrappers[target.Instances[i].Name]; wrapper != nil {
			target.Instances[i].Replicas = pointer.Int32(target.Instances[i].GetReplicas() + wrapper.replicas)
		}
	}
	sameAllocationSpec := func(a, b *appsv1.ClusterComponentSpec) bool {
		return a.Replicas == b.Replicas && reflect.DeepEqual(a.Instances, b.Instances) && slices.Equal(a.OfflineInstances, b.OfflineInstances)
	}
	if !sameAllocationSpec(comp, source) && !sameAllocationSpec(comp, target) {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf("the replica configuration of component %q has been modified by another operation", comp.Name))
	}
	return target, nil
}

func (r rebuildInstanceOpsHandler) scaleOutAllocatedInstances(opsRes *OpsResource,
	comp *appsv1.ClusterComponentSpec, rebuild opsv1alpha1.RebuildInstance,
	status *opsv1alpha1.OpsRequestComponentStatus, wrappers map[string]*rebuildInstanceWrapper) error {
	targetSpec, err := rebuildExpansion(opsRes, comp, wrappers)
	if err != nil {
		return err
	}
	if comp.Replicas != targetSpec.Replicas {
		comp.Replicas, comp.Instances = targetSpec.Replicas, targetSpec.Instances
		return nil
	}
	runtime, err := opsRes.GetRuntime(comp.Name)
	if err != nil {
		return err
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, comp.Name)
	if err != nil {
		return err
	}
	target, complete, err := activeAssignmentsForTarget(workload, targetSpec)
	if err != nil || !complete {
		return err // no mapping yet: ReconcileAction continues reporting Running.
	}
	mapping, err := replacementInstanceMapping(opsRes, comp.Name, target, wrappers)
	if err != nil {
		return err
	}
	r.recordReplacementInstances(opsRes, rebuild, status, mapping)
	return nil
}

func replacementInstanceMapping(opsRes *OpsResource, componentName string, target map[string]string,
	wrappers map[string]*rebuildInstanceWrapper) (map[string]string, error) {
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[componentName]
	source := sourceAssignmentsForWorkload(last, constant.GenerateClusterComponentName(opsRes.Cluster.Name, componentName))
	if last.Replicas == nil || !assignmentsMatchComponent(source, &appsv1.ClusterComponentSpec{Replicas: *last.Replicas, Instances: last.Instances}) {
		return nil, fmt.Errorf("source allocation of component %q is incomplete", componentName)
	}
	for name, template := range source {
		if actual, ok := target[name]; !ok || actual != template {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for the replacement allocation of component %q", componentName)
		}
	}
	created, _ := diffAssignments(source, target)
	byTemplate := map[string][]string{}
	for name, template := range created {
		byTemplate[template] = append(byTemplate[template], name)
	}
	if len(byTemplate) != len(wrappers) {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for replacement instances in the requested templates")
	}
	mapping := map[string]string{}
	for template, wrapper := range wrappers {
		names := byTemplate[template]
		if len(names) != len(wrapper.insNames) {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for replacement instances in template %q", template)
		}
		slices.Sort(names)
		originals := slices.Clone(wrapper.insNames)
		slices.Sort(originals)
		for i, name := range originals {
			mapping[name] = names[i]
		}
	}
	return mapping, nil
}

func (r rebuildInstanceOpsHandler) recordReplacementInstances(opsRes *OpsResource, rebuild opsv1alpha1.RebuildInstance,
	status *opsv1alpha1.OpsRequestComponentStatus, mapping map[string]string) {
	for _, instance := range rebuild.Instances {
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &status.ProgressDetails,
			opsv1alpha1.ProgressStatusDetail{
				ObjectKey: getProgressObjectKey(constant.PodKind, instance.Name),
				Status:    opsv1alpha1.ProcessingProgressStatus,
				Message:   r.buildScalingOutPodMessage(mapping[instance.Name], "Processing"),
			})
	}
}

// Only non-flat TargetNodeName requests may use predicted replacement names.
// Persist placement before the Cluster spec update can trigger Pod creation.
func (r rebuildInstanceOpsHandler) scaleOutWithPlacement(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compSpec *appsv1.ClusterComponentSpec,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compStatus *opsv1alpha1.OpsRequestComponentStatus,
	rebuildInsWrapper map[string]*rebuildInstanceWrapper) error {
	if compSpec.FlatInstanceOrdinal {
		return intctrlutil.NewFatalError("non-in-place rebuild with targetNodeName is not supported for flat instance ordinals")
	}
	targetSpec, err := rebuildExpansion(opsRes, compSpec, rebuildInsWrapper)
	if err != nil {
		return err
	}
	runtime, err := opsRes.GetRuntime(compSpec.Name)
	if err != nil {
		return err
	}
	target, err := runtime.GenerateInstanceNameSet(opsRes.Cluster.Name, compSpec.Name, targetSpec.Replicas, targetSpec.Instances, targetSpec.OfflineInstances)
	if err != nil {
		return err
	}
	scaleOutInsMap, err := replacementInstanceMapping(opsRes, compSpec.Name, target, rebuildInsWrapper)
	if err != nil {
		return err
	}

	its := &workloads.InstanceSet{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: constant.GenerateClusterComponentName(opsRes.Cluster.Name, compSpec.Name), Namespace: opsRes.OpsRequest.Namespace}, its); err != nil {
		return err
	}
	itsUpdated := false
	for _, ins := range rebuildInstance.Instances {
		scaleOutInsName := scaleOutInsMap[ins.Name]

		// specify node to scale out
		if ins.TargetNodeName != "" {
			if err := instanceset.MergeNodeSelectorOnceAnnotation(its, map[string]string{scaleOutInsName: ins.TargetNodeName}); err != nil {
				return err
			}
			itsUpdated = true
		}
	}

	if itsUpdated {
		if err := cli.Update(reqCtx.Ctx, its); err != nil {
			return err
		}
	}
	compSpec.Replicas, compSpec.Instances = targetSpec.Replicas, targetSpec.Instances
	r.recordReplacementInstances(opsRes, rebuildInstance, compStatus, scaleOutInsMap)
	return nil
}

// checkProgressForScalingOutPods checks if the new pods are available.
func (r rebuildInstanceOpsHandler) checkProgressForScalingOutPods(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	compSpec *appsv1.ClusterComponentSpec,
	compStatus *opsv1alpha1.OpsRequestComponentStatus) (int, int, []string, error) {
	var (
		instancesNeedToOffline []string
		failedCount            int
		completedCount         int
	)
	runtime, err := opsRes.GetRuntime(compSpec.Name)
	if err != nil {
		return 0, 0, nil, err
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, compSpec.Name)
	if err != nil {
		return 0, 0, nil, err
	}
	currPodSet, complete, err := activeAssignmentsForTarget(workload, compSpec)
	if err != nil || !complete {
		return 0, 0, nil, err
	}
	synthesizedComp, err := r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, compSpec.Name)
	if err != nil {
		return 0, 0, nil, err
	}
	roleAware := len(synthesizedComp.Roles) > 0
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[compSpec.Name]
	source := sourceAssignmentsForWorkload(last, constant.GenerateClusterComponentName(opsRes.Cluster.Name, compSpec.Name))
	for _, instance := range rebuildInstance.Instances {
		progressDetail := r.getInstanceProgressDetail(*compStatus, instance.Name)
		scalingOutPodName := r.getScalingOutPodNameFromMessage(progressDetail.Message)
		if template, ok := currPodSet[scalingOutPodName]; !ok || template != source[instance.Name] {
			return 0, 0, nil, intctrlutil.NewFatalError(fmt.Sprintf(`the replicas of the component "%s" has been modified by another operation`, compSpec.Name))
		}
		scaledOutInstance, err := runtime.GetInstance(opsRes.Cluster.Namespace, opsRes.Cluster.Name, compSpec.Name, scalingOutPodName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				reqCtx.Log.Info(fmt.Sprintf("waiting to create the pod %s", scalingOutPodName))
				continue
			}
			return 0, 0, nil, err
		}
		if scaledOutInstance.IsFailedAndTimedOut() {
			failedCount += 1
			completedCount += 1
			progressDetail.SetStatusAndMessage(opsv1alpha1.FailedProgressStatus,
				r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.UnavailablePhase)))
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if !scaledOutInstance.IsAvailable(synthesizedComp.MinReadySeconds, roleAware) {
			reqCtx.Log.Info(fmt.Sprintf("waiting to create the pod %s", scalingOutPodName))
			continue
		}
		if slices.Contains(compSpec.OfflineInstances, instance.Name) {
			oldInstance, err := runtime.GetInstance(opsRes.Cluster.Namespace, opsRes.Cluster.Name, compSpec.Name, instance.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				return 0, 0, nil, err
			}
			if apierrors.IsNotFound(err) || !oldInstance.HasPod() {
				// Retained PVCs do not prevent completion once the old Pod is gone.
				// it indicates that the rebuild process has been completed.
				completedCount += 1
				progressDetail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus,
					r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.AvailablePhase)))
			} else {
				progressDetail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus,
					r.buildScalingOutPodMessage(scalingOutPodName, string(opsv1alpha1.AvailablePhase)))
				if oldInstance.IsDeleting() && opsRes.OpsRequest.Force() {
					pod := &corev1.Pod{}
					if getErr := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: instance.Name, Namespace: opsRes.Cluster.Namespace}, pod); getErr == nil {
						_ = intctrlutil.BackgroundDeleteObject(cli, reqCtx.Ctx, pod, client.GracePeriodSeconds(0))
					}
				}
			}
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
		} else {
			instancesNeedToOffline = append(instancesNeedToOffline, instance.Name)
		}
	}
	return completedCount, failedCount, instancesNeedToOffline, nil
}

// offlineSpecifiedInstances to take the specific instances offline.
func (r rebuildInstanceOpsHandler) offlineSpecifiedInstances(opsRes *OpsResource, compSpec *appsv1.ClusterComponentSpec, instancesNeedToOffline []string) error {
	last := opsRes.OpsRequest.Status.LastConfiguration.Components[compSpec.Name]
	source := sourceAssignmentsForWorkload(last, constant.GenerateClusterComponentName(opsRes.Cluster.Name, compSpec.Name))
	for _, name := range instancesNeedToOffline {
		if _, ok := source[name]; !ok {
			return fmt.Errorf("missing source template for rebuild instance %q", name)
		}
	}
	for _, insName := range instancesNeedToOffline {
		compSpec.OfflineInstances = append(compSpec.OfflineInstances, insName)
		templateName := source[insName]
		if templateName == constant.EmptyInsTemplateName {
			continue
		}
		for j := range compSpec.Instances {
			instanceTpl := &compSpec.Instances[j]
			if instanceTpl.Name == templateName {
				instanceTpl.Replicas = pointer.Int32(instanceTpl.GetReplicas() - 1)
			}
		}
	}
	compSpec.Replicas -= int32(len(instancesNeedToOffline))
	return nil
}

func (r rebuildInstanceOpsHandler) buildScalingOutPodMessage(scaleOutPodName string, status string) string {
	return fmt.Sprintf("%s: %s, status: %s", scalingOutPodPrefixMsg, scaleOutPodName, status)
}

func (r rebuildInstanceOpsHandler) getScalingOutPodNameFromMessage(progressMsg string) string {
	if !strings.HasPrefix(progressMsg, scalingOutPodPrefixMsg) {
		return ""
	}
	strArr := strings.Split(progressMsg, ",")
	return strings.Replace(strArr[0], scalingOutPodPrefixMsg+": ", "", 1)
}

func (r rebuildInstanceOpsHandler) buildSynthesizedComponent(ctx context.Context,
	cli client.Client,
	cluster *appsv1.Cluster,
	compName string) (*component.SynthesizedComponent, error) {
	comp, compDef, err := component.GetCompNCompDefByName(ctx, cli, cluster.Namespace, constant.GenerateClusterComponentName(cluster.Name, compName))
	if err != nil {
		return nil, err
	}
	return component.BuildSynthesizedComponent(ctx, cli, compDef, comp)
}

func (r rebuildInstanceOpsHandler) prepareInplaceRebuildHelper(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rebuildInstance opsv1alpha1.RebuildInstance,
	instance opsv1alpha1.Instance,
	index int) (*inplaceRebuildHelper, error) {
	var (
		backup          *dpv1alpha1.Backup
		actionSet       *dpv1alpha1.ActionSet
		synthesizedComp *component.SynthesizedComponent
		err             error
	)
	if rebuildInstance.BackupName != "" {
		// prepare backup infos
		backup = &dpv1alpha1.Backup{}
		if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: rebuildInstance.BackupName, Namespace: opsRes.Cluster.Namespace}, backup); err != nil {
			return nil, err
		}
		if !slices.Contains([]string{string(dpv1alpha1.BackupTypeFull), string(dpv1alpha1.BackupTypeIncremental)}, backup.Labels[dptypes.BackupTypeLabelKey]) {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" is not a Full or Incremental backup`, rebuildInstance.BackupName))
		}
		if backup.Status.Phase != dpv1alpha1.BackupPhaseCompleted {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backup "%s" phase is not Completed`, rebuildInstance.BackupName))
		}
		if backup.Status.BackupMethod == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`the backupMethod of the backup "%s" can not be empty`, rebuildInstance.BackupName))
		}
		actionSet, err = dputils.GetActionSetByName(reqCtx, cli, backup.Status.BackupMethod.ActionSetName)
		if err != nil {
			return nil, err
		}
	}
	targetPod := &corev1.Pod{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: instance.Name, Namespace: opsRes.Cluster.Namespace}, targetPod); err != nil {
		return nil, err
	}
	synthesizedComp, err = r.buildSynthesizedComponent(reqCtx.Ctx, cli, opsRes.Cluster, targetPod.Labels[constant.KBAppComponentLabelKey])
	if err != nil {
		return nil, err
	}
	rebuildPrefix := fmt.Sprintf("rebuild-%s", opsRes.OpsRequest.UID[:8])
	runtime, err := opsRes.GetRuntime(rebuildInstance.ComponentName)
	if err != nil {
		return nil, err
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, synthesizedComp.Name)
	if err != nil {
		return nil, err
	}
	templateName, err := instanceTemplateByName(workload.GetInstanceStatuses(), instance.Name)
	if err != nil {
		return nil, err
	}
	pvcMap, volumes, volumeMounts, err := getPVCMapAndVolumes(opsRes, synthesizedComp, targetPod, templateName, rebuildPrefix, index, rebuildInstance.BackupName == "")
	if err != nil {
		return nil, err
	}
	return &inplaceRebuildHelper{
		index:                  index,
		backup:                 backup,
		instance:               instance,
		actionSet:              actionSet,
		synthesizedComp:        synthesizedComp,
		sourceBackupTargetName: rebuildInstance.SourceBackupTargetName,
		pvcMap:                 pvcMap,
		volumes:                volumes,
		targetPod:              targetPod,
		volumeMounts:           volumeMounts,
		rebuildPrefix:          rebuildPrefix,
		envForRestore:          rebuildInstance.RestoreEnv,
	}, nil
}

// cleanupTmpResources clean up the temporary resources generated during the process of rebuilding the instance.
func (r rebuildInstanceOpsHandler) cleanupTmpResources(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource) error {
	matchLabels := client.MatchingLabels{
		constant.OpsRequestNameLabelKey:      opsRes.OpsRequest.Name,
		constant.OpsRequestNamespaceLabelKey: opsRes.OpsRequest.Namespace,
	}
	// TODO: need to delete the restore CR?
	// Pods are limited in k8s, so we need to release them if they are not needed.
	return intctrlutil.DeleteOwnedResources(reqCtx.Ctx, cli, opsRes.OpsRequest, matchLabels, generics.PodSignature)
}
