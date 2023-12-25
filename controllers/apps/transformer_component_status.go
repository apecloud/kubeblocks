/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

const (
	// componentPhaseTransition the event reason indicates that the component transits to a new phase.
	componentPhaseTransition = "ComponentPhaseTransition"

	// podContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	podContainerFailedTimeout = 10 * time.Second

	// podScheduledFailedTimeout timeout for scheduling failure.
	podScheduledFailedTimeout = 30 * time.Second
)

// componentStatusTransformer computes the current status: read the underlying rsm status and update the component status
type componentStatusTransformer struct {
	client.Client
}

// componentStatusHandler handles the component status
type componentStatusHandler struct {
	cli            client.Client
	reqCtx         intctrlutil.RequestCtx
	cluster        *appsv1alpha1.Cluster
	comp           *appsv1alpha1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	// runningRSM is a snapshot of the rsm that is already running
	runningRSM *workloads.ReplicatedStateMachine
	// protoRSM is the rsm object that is rebuilt from scratch during each reconcile process
	protoRSM *workloads.ReplicatedStateMachine
	// podsReady indicates if the component's underlying pods are ready
	podsReady bool
}

var _ graph.Transformer = &componentStatusTransformer{}

func (t *componentStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if transCtx.RunningWorkload == nil {
		transCtx.Logger.Info(fmt.Sprintf("skip reconcile component status because underlying workload object not found, generation: %d", comp.Generation))
		return nil
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent
	runningRSM, _ := transCtx.RunningWorkload.(*workloads.ReplicatedStateMachine)
	protoRSM, _ := transCtx.ProtoWorkload.(*workloads.ReplicatedStateMachine)
	switch {
	case model.IsObjectUpdating(transCtx.ComponentOrig):
		transCtx.Logger.Info(fmt.Sprintf("update component status after applying resources, generation: %d", comp.Generation))
		comp.Status.ObservedGeneration = comp.Generation
	case model.IsObjectStatusUpdating(transCtx.ComponentOrig):
		// reconcile the component status and sync the component status to cluster status
		csh := newComponentStatusHandler(reqCtx, t.Client, cluster, comp, synthesizeComp, runningRSM, protoRSM, dag)
		if err := csh.reconcileComponentStatus(); err != nil {
			return err
		}
		comp = csh.comp
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	graphCli.Status(dag, transCtx.ComponentOrig, comp)

	return nil
}

// reconcileComponentStatus reconciles component status.
func (r *componentStatusHandler) reconcileComponentStatus() error {
	if r.runningRSM == nil {
		return nil
	}

	// check if the rsm is deleting
	isDeleting := func() bool {
		return !r.runningRSM.DeletionTimestamp.IsZero()
	}()

	// check if the rsm replicas is zero
	isZeroReplica := func() bool {
		return (r.runningRSM.Spec.Replicas == nil || *r.runningRSM.Spec.Replicas == 0) && r.synthesizeComp.Replicas == 0
	}()

	// get the component's underlying pods
	pods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli,
		r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
	hasComponentPod := func() bool {
		return len(pods) > 0
	}()

	// check if the rsm is running
	isRSMRunning, err := r.isRSMRunning()
	if err != nil {
		return err
	}

	// check if all configTemplates are synced
	isAllConfigSynced, err := r.isAllConfigSynced()
	if err != nil {
		return err
	}

	// check if the component has failed pod
	hasFailedPod, messages, err := r.hasFailedPod(pods)
	if err != nil {
		return err
	}

	// check if the component scale out failed
	isScaleOutFailed, err := r.isScaleOutFailed()
	if err != nil {
		return err
	}

	// check if the volume expansion is running
	hasRunningVolumeExpansion, hasFailedVolumeExpansion, err := r.hasVolumeExpansionRunning()
	if err != nil {
		return err
	}

	// calculate if the component has failure
	hasFailure := func() bool {
		return hasFailedPod || isScaleOutFailed || hasFailedVolumeExpansion
	}()

	// check if the component is available
	isComponentAvailable, err := r.isComponentAvailable(pods)
	if err != nil {
		return err
	}

	// check if the component is in creating phase
	isInCreatingPhase := func() bool {
		phase := r.comp.Status.Phase
		return phase == "" || phase == appsv1alpha1.CreatingClusterCompPhase
	}()

	r.reqCtx.Log.Info(
		fmt.Sprintf("component status conditions, isRSMRunning: %v, isAllConfigSynced: %v, hasRunningVolumeExpansion: %v, hasFailure: %v,  isInCreatingPhase: %v, isComponentAvailable: %v",
			isRSMRunning, isAllConfigSynced, hasRunningVolumeExpansion, hasFailure, isInCreatingPhase, isComponentAvailable))

	r.podsReady = false
	switch {
	case isDeleting:
		r.setComponentStatusPhase(appsv1alpha1.DeletingClusterCompPhase, nil, "component is Deleting")
	case isZeroReplica && hasComponentPod:
		r.setComponentStatusPhase(appsv1alpha1.StoppingClusterCompPhase, nil, "component is Stopping")
		r.podsReady = true
	case isZeroReplica:
		r.setComponentStatusPhase(appsv1alpha1.StoppedClusterCompPhase, nil, "component is Stopped")
		r.podsReady = true
	case isRSMRunning && isAllConfigSynced && !hasRunningVolumeExpansion:
		r.setComponentStatusPhase(appsv1alpha1.RunningClusterCompPhase, nil, "component is Running")
		r.podsReady = true
	case !hasFailure && isInCreatingPhase:
		r.setComponentStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "component is Creating")
	case !hasFailure:
		r.setComponentStatusPhase(appsv1alpha1.UpdatingClusterCompPhase, nil, "component is Updating")
	case !isComponentAvailable:
		r.setComponentStatusPhase(appsv1alpha1.FailedClusterCompPhase, messages, "component is Failed")
	default:
		r.setComponentStatusPhase(appsv1alpha1.AbnormalClusterCompPhase, nil, "component is Abnormal")
	}

	// update component info to pods' annotations
	// TODO(xingran): should be move this to rsm controller
	if err := UpdateComponentInfoToPods(r.reqCtx.Ctx, r.cli, r.cluster, r.synthesizeComp, r.dag); err != nil {
		return err
	}

	// patch the current componentSpec workload's custom labels
	// TODO(xingran): should be move this to rsm controller, and add custom annotations support. then add a independent transformer to deal with component level custom labels and annotations.
	if err := UpdateCustomLabelToPods(r.reqCtx.Ctx, r.cli, r.cluster, r.synthesizeComp, r.dag); err != nil {
		r.reqCtx.Event(r.cluster, corev1.EventTypeWarning, "Component Controller PatchWorkloadCustomLabelFailed", err.Error())
		return err
	}

	// set primary-pod annotation
	// TODO(free6om): primary-pod is only used in redis to bootstrap the redis cluster correctly.
	// it is too hacky to be replaced by a better design.
	if err := r.updatePrimaryIndex(); err != nil {
		return err
	}

	return nil
}

// isComponentAvailable tells whether the component is basically available, ether working well or in a fragile state:
// 1. at least one pod is available
// 2. with latest revision
// 3. and with leader role label set
func (r *componentStatusHandler) isComponentAvailable(pods []*corev1.Pod) (bool, error) {
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningRSM); err != nil {
		return false, err
	} else if !isLatestRevision {
		return false, nil
	}

	shouldCheckRole := len(r.synthesizeComp.Roles) > 0

	hasLeaderRoleLabel := func(pod *corev1.Pod) bool {
		roleName, ok := pod.Labels[constant.RoleLabelKey]
		if !ok {
			return false
		}
		for _, replicaRole := range r.runningRSM.Spec.Roles {
			if roleName == replicaRole.Name && replicaRole.IsLeader {
				return true
			}
		}
		return false
	}

	hasPodAvailable := false
	for _, pod := range pods {
		if !podutils.IsPodAvailable(pod, 0, metav1.Time{Time: time.Now()}) {
			continue
		}
		if shouldCheckRole && hasLeaderRoleLabel(pod) {
			return true, nil
		}
		if !hasPodAvailable {
			hasPodAvailable = !shouldCheckRole
		}
	}
	return hasPodAvailable, nil
}

// isRunning checks if the component underlying rsm workload is running.
func (r *componentStatusHandler) isRSMRunning() (bool, error) {
	if r.runningRSM == nil {
		return false, nil
	}
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningRSM); err != nil {
		return false, err
	} else if !isLatestRevision {
		return false, nil
	}

	// whether rsm is ready
	return rsmcore.IsRSMReady(r.runningRSM), nil
}

// isAllConfigSynced checks if all configTemplates are synced.
func (r *componentStatusHandler) isAllConfigSynced() (bool, error) {
	var (
		cmKey client.ObjectKey
		cmObj = &corev1.ConfigMap{}
	)

	if len(r.synthesizeComp.ConfigTemplates) == 0 {
		return true, nil
	}

	configurationKey := client.ObjectKey{
		Namespace: r.cluster.Namespace,
		Name:      cfgcore.GenerateComponentConfigurationName(r.cluster.Name, r.synthesizeComp.Name),
	}
	configuration := &appsv1alpha1.Configuration{}
	if err := r.cli.Get(r.reqCtx.Ctx, configurationKey, configuration); err != nil {
		return false, err
	}
	for _, configSpec := range r.synthesizeComp.ConfigTemplates {
		item := configuration.Spec.GetConfigurationItem(configSpec.Name)
		status := configuration.Status.GetItemStatus(configSpec.Name)
		// for creating phase
		if item == nil || status == nil {
			return false, nil
		}
		cmKey = client.ObjectKey{
			Namespace: r.cluster.Namespace,
			Name:      cfgcore.GetComponentCfgName(r.cluster.Name, r.synthesizeComp.Name, configSpec.Name),
		}
		if err := r.cli.Get(r.reqCtx.Ctx, cmKey, cmObj); err != nil {
			return false, err
		}
		if intctrlutil.GetConfigSpecReconcilePhase(cmObj, *item, status) != appsv1alpha1.CFinishedPhase {
			return false, nil
		}
	}
	return true, nil
}

// isScaleOutFailed checks if the component scale out failed.
func (r *componentStatusHandler) isScaleOutFailed() (bool, error) {
	if r.runningRSM == nil {
		return false, nil
	}
	if r.runningRSM.Spec.Replicas == nil {
		return false, nil
	}
	if r.synthesizeComp.Replicas <= *r.runningRSM.Spec.Replicas {
		return false, nil
	}

	// stsObj is the underlying rsm workload which is already running in the component.
	stsObj := rsmcore.ConvertRSMToSTS(r.runningRSM)
	stsProto := rsmcore.ConvertRSMToSTS(r.protoRSM)
	backupKey := types.NamespacedName{
		Namespace: stsObj.Namespace,
		Name:      stsObj.Name + "-scaling",
	}
	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, stsObj, stsProto, backupKey)
	if err != nil {
		return false, err
	}
	if status, err := d.CheckBackupStatus(); err != nil {
		return false, err
	} else if status == backupStatusFailed {
		return true, nil
	}
	for i := *r.runningRSM.Spec.Replicas; i < r.synthesizeComp.Replicas; i++ {
		if status, err := d.CheckRestoreStatus(i); err != nil {
			return false, err
		} else if status == backupStatusFailed {
			return true, nil
		}
	}
	return false, nil
}

// hasVolumeExpansionRunning checks if the volume expansion is running.
func (r *componentStatusHandler) hasVolumeExpansionRunning() (bool, bool, error) {
	var (
		running bool
		failed  bool
	)
	for _, vct := range r.runningRSM.Spec.VolumeClaimTemplates {
		volumes, err := r.getRunningVolumes(r.reqCtx, r.cli, vct.Name, r.runningRSM)
		if err != nil {
			return false, false, err
		}
		for _, v := range volumes {
			if v.Status.Capacity == nil || v.Status.Capacity.Storage().Cmp(v.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
				continue
			}
			running = true
			// TODO: how to check the expansion failed?
		}
	}
	return running, failed, nil
}

// getRunningVolumes gets the running volumes of the rsm.
func (r *componentStatusHandler) getRunningVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string,
	rsmObj *workloads.ReplicatedStateMachine) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := component.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature,
		r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	matchedPVCs := make([]*corev1.PersistentVolumeClaim, 0)
	prefix := fmt.Sprintf("%s-%s", vctName, rsmObj.Name)
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, prefix) {
			matchedPVCs = append(matchedPVCs, pvc)
		}
	}
	return matchedPVCs, nil
}

// hasFailedPod checks if the component has failed pod.
// TODO(xingran): remove the dependency of the component's workload type.
func (r *componentStatusHandler) hasFailedPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningRSM); err != nil {
		return false, nil, err
	} else if !isLatestRevision {
		return false, nil, nil
	}

	var messages appsv1alpha1.ComponentMessageMap
	// check pod readiness
	hasFailedPod, msg, _ := hasFailedAndTimedOutPod(pods)
	if hasFailedPod {
		messages = msg
		return true, messages, nil
	}
	// check role probe
	if r.synthesizeComp.WorkloadType != appsv1alpha1.Consensus && r.synthesizeComp.WorkloadType != appsv1alpha1.Replication {
		return false, messages, nil
	}
	hasProbeTimeout := false
	for _, pod := range pods {
		if _, ok := pod.Labels[constant.RoleLabelKey]; ok {
			continue
		}
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady || condition.Status != corev1.ConditionTrue {
				continue
			}
			podsReadyTime := &condition.LastTransitionTime
			if IsProbeTimeout(r.synthesizeComp.Probes, podsReadyTime) {
				hasProbeTimeout = true
				if messages == nil {
					messages = appsv1alpha1.ComponentMessageMap{}
				}
				messages.SetObjectMessage(pod.Kind, pod.Name, "Role probe timeout, check whether the application is available")
			}
		}
	}
	return hasProbeTimeout, messages, nil
}

// setComponentStatusPhase sets the component phase and messages conditionally.
func (r *componentStatusHandler) setComponentStatusPhase(phase appsv1alpha1.ClusterComponentPhase, statusMessage appsv1alpha1.ComponentMessageMap, phaseTransitionMsg string) {
	updateFn := func(status *appsv1alpha1.ComponentStatus) error {
		if status.Phase == phase {
			return nil
		}
		status.Phase = phase
		if status.Message == nil {
			status.Message = statusMessage
		} else {
			for k, v := range statusMessage {
				status.Message[k] = v
			}
		}
		return nil
	}
	if err := r.updateComponentStatus(phaseTransitionMsg, updateFn); err != nil {
		panic(fmt.Sprintf("unexpected error occurred while updating component status: %s", err.Error()))
	}
}

// updateComponentStatus updates the component status by @updateFn, with additional message to explain the transition occurred.
func (r *componentStatusHandler) updateComponentStatus(phaseTransitionMsg string, updateFn func(status *appsv1alpha1.ComponentStatus) error) error {
	if updateFn == nil {
		return nil
	}
	phase := r.comp.Status.Phase
	err := updateFn(&r.comp.Status)
	if err != nil {
		return err
	}
	if phase != r.comp.Status.Phase {
		if r.reqCtx.Recorder != nil && phaseTransitionMsg != "" {
			r.reqCtx.Recorder.Eventf(r.comp, corev1.EventTypeNormal, componentPhaseTransition, phaseTransitionMsg)
		}
	}
	return nil
}

// updatePrimaryIndex updates the primary pod index to the pod annotations.
// TODO: the need to update primary info is because some database engines currently require external specification of the primary node.
// Based on the specified primary node, the primary-secondary relationship can be established during startup. Examples of such engines include primary-secondary Redis.
// In the future, there is a need for a better design to replace this kind of workaround.
func (r *componentStatusHandler) updatePrimaryIndex() error {
	// TODO(xingran): consider if there are alternative ways to determine whether it is necessary to specify primary info in the Controller
	if r.synthesizeComp.RoleArbitrator == nil || *r.synthesizeComp.RoleArbitrator != appsv1alpha1.LorryRoleArbitrator {
		return nil
	}
	podList, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
	if len(podList) == 0 {
		return nil
	}
	slices.SortFunc(podList, func(a, b *corev1.Pod) bool {
		return a.GetName() < b.GetName()
	})
	primaryPods := make([]string, 0)
	emptyRolePods := make([]string, 0)
	for _, pod := range podList {
		role, ok := pod.Labels[constant.RoleLabelKey]
		if !ok || role == "" {
			emptyRolePods = append(emptyRolePods, pod.Name)
			continue
		}
		if role == constant.Primary {
			primaryPods = append(primaryPods, pod.Name)
		}
	}
	primaryPodName, err := func() (string, error) {
		switch {
		// if the workload is newly created, and the role label is not set, we set the pod with index=0 as the primary by default.
		case len(emptyRolePods) == len(podList):
			return podList[0].Name, nil
		case len(primaryPods) != 1:
			return "", fmt.Errorf("the number of primary pod is not equal to 1, primary pods: %v, emptyRole pods: %v", primaryPods, emptyRolePods)
		default:
			return primaryPods[0], nil
		}
	}()
	if err != nil {
		return err
	}
	graphCli := model.NewGraphClient(r.cli)
	for _, pod := range podList {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pi, ok := pod.Annotations[constant.PrimaryAnnotationKey]
		if !ok || pi != primaryPodName {
			origPod := pod.DeepCopy()
			pod.Annotations[constant.PrimaryAnnotationKey] = primaryPodName
			graphCli.Do(r.dag, origPod, pod, model.ActionUpdatePtr(), nil)
		}
	}
	return nil
}

// hasFailedAndTimedOutPod returns whether the pods of components are still failed after a PodFailedTimeout period.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, time.Duration) {
	var (
		hasTimedOutPod bool
		messages       = appsv1alpha1.ComponentMessageMap{}
		hasFailedPod   bool
		requeueAfter   time.Duration
	)
	for _, pod := range pods {
		isFailed, isTimedOut, messageStr := isPodFailedAndTimedOut(pod)
		if !isFailed {
			continue
		}
		if isTimedOut {
			hasTimedOutPod = true
			messages.SetObjectMessage(pod.Kind, pod.Name, messageStr)
		} else {
			hasFailedPod = true
		}
	}
	if hasFailedPod && !hasTimedOutPod {
		requeueAfter = podContainerFailedTimeout
	}
	return hasTimedOutPod, messages, requeueAfter
}

// isPodFailedAndTimedOut checks if the pod is failed and timed out.
func isPodFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	if isFailed, isTimedOut, message := isPodScheduledFailedAndTimedOut(pod); isFailed {
		return isFailed, isTimedOut, message
	}
	initContainerFailed, message := isAnyContainerFailed(pod.Status.InitContainerStatuses)
	if initContainerFailed {
		return initContainerFailed, isContainerFailedAndTimedOut(pod, corev1.PodInitialized), message
	}
	containerFailed, message := isAnyContainerFailed(pod.Status.ContainerStatuses)
	if containerFailed {
		return containerFailed, isContainerFailedAndTimedOut(pod, corev1.ContainersReady), message
	}
	return false, false, ""
}

// isPodScheduledFailedAndTimedOut checks whether the unscheduled pod has timed out.
func isPodScheduledFailedAndTimedOut(pod *corev1.Pod) (bool, bool, string) {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodScheduled {
			continue
		}
		if cond.Status == corev1.ConditionTrue {
			return false, false, ""
		}
		return true, time.Now().After(cond.LastTransitionTime.Add(podScheduledFailedTimeout)), cond.Message
	}
	return false, false, ""
}

// isAnyContainerFailed checks whether any container in the list is failed.
func isAnyContainerFailed(containersStatus []corev1.ContainerStatus) (bool, string) {
	for _, v := range containersStatus {
		waitingState := v.State.Waiting
		if waitingState != nil && waitingState.Message != "" {
			return true, waitingState.Message
		}
		terminatedState := v.State.Terminated
		if terminatedState != nil && terminatedState.Message != "" {
			return true, terminatedState.Message
		}
	}
	return false, ""
}

// isContainerFailedAndTimedOut checks whether the failed container has timed out.
func isContainerFailedAndTimedOut(pod *corev1.Pod, podConditionType corev1.PodConditionType) bool {
	containerReadyCondition := intctrlutil.GetPodCondition(&pod.Status, podConditionType)
	if containerReadyCondition == nil || containerReadyCondition.LastTransitionTime.IsZero() {
		return false
	}
	return time.Now().After(containerReadyCondition.LastTransitionTime.Add(podContainerFailedTimeout))
}

// newComponentStatusHandler creates a new componentStatusHandler
func newComponentStatusHandler(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *component.SynthesizedComponent,
	runningRSM *workloads.ReplicatedStateMachine,
	protoRSM *workloads.ReplicatedStateMachine,
	dag *graph.DAG) *componentStatusHandler {
	return &componentStatusHandler{
		cli:            cli,
		reqCtx:         reqCtx,
		cluster:        cluster,
		comp:           comp,
		synthesizeComp: synthesizeComp,
		runningRSM:     runningRSM,
		protoRSM:       protoRSM,
		dag:            dag,
		podsReady:      false,
	}
}
