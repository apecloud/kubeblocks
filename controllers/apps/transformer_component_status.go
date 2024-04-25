/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	// componentPhaseTransition the event reason indicates that the component transits to a new phase.
	componentPhaseTransition = "ComponentPhaseTransition"
)

// componentStatusTransformer computes the current status: read the underlying workload status and update the component status
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

	// runningITS is a snapshot of the ITS that is already running
	runningITS *workloads.InstanceSet
	// protoITS is the ITS object that is rebuilt from scratch during each reconcile process
	protoITS *workloads.InstanceSet
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
	runningITS, _ := transCtx.RunningWorkload.(*workloads.InstanceSet)
	protoITS, _ := transCtx.ProtoWorkload.(*workloads.InstanceSet)
	switch {
	case model.IsObjectUpdating(transCtx.ComponentOrig):
		transCtx.Logger.Info(fmt.Sprintf("update component status after applying resources, generation: %d", comp.Generation))
		comp.Status.ObservedGeneration = comp.Generation
	case model.IsObjectStatusUpdating(transCtx.ComponentOrig):
		// reconcile the component status and sync the component status to cluster status
		csh := newComponentStatusHandler(reqCtx, t.Client, cluster, comp, synthesizeComp, runningITS, protoITS, dag)
		if err := csh.reconcileComponentStatus(); err != nil {
			return err
		}
		comp = csh.comp
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if vertex := graphCli.FindMatchedVertex(dag, comp); vertex != nil {
		// check if the component needs to do other action.
		ov, _ := vertex.(*model.ObjectVertex)
		if ov.Action != model.ActionNoopPtr() {
			return nil
		}
	}
	graphCli.Status(dag, transCtx.ComponentOrig, comp)
	return nil
}

// reconcileComponentStatus reconciles component status.
func (r *componentStatusHandler) reconcileComponentStatus() error {
	if r.runningITS == nil {
		return nil
	}

	// check if the ITS is deleting
	isDeleting := func() bool {
		return !r.runningITS.DeletionTimestamp.IsZero()
	}()

	// check if the ITS replicas is zero
	isZeroReplica := func() bool {
		return (r.runningITS.Spec.Replicas == nil || *r.runningITS.Spec.Replicas == 0) && r.synthesizeComp.Replicas == 0
	}()

	// get the component's underlying pods
	pods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace,
		constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name), inDataContext4C())
	if err != nil && !isUnavailableError(err) {
		return err
	}
	hasComponentPod := func() bool {
		return len(pods) > 0
	}()

	// check if the ITS is running
	isITSRunning, err := r.isInstanceSetRunning()
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
		fmt.Sprintf("component status conditions, isInstanceSetRunning: %v, isAllConfigSynced: %v, hasRunningVolumeExpansion: %v, hasFailure: %v,  isInCreatingPhase: %v, isComponentAvailable: %v",
			isITSRunning, isAllConfigSynced, hasRunningVolumeExpansion, hasFailure, isInCreatingPhase, isComponentAvailable))

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
	case isITSRunning && isAllConfigSynced && !hasRunningVolumeExpansion:
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

	return nil
}

// isComponentAvailable tells whether the component is basically available, ether working well or in a fragile state:
// 1. at least one pod is available
// 2. with latest revision
// 3. and with leader role label set
func (r *componentStatusHandler) isComponentAvailable(pods []*corev1.Pod) (bool, error) {
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningITS); err != nil {
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
		for _, replicaRole := range r.runningITS.Spec.Roles {
			if roleName == replicaRole.Name && replicaRole.IsLeader {
				return true
			}
		}
		return false
	}

	hasPodAvailable := false
	for _, pod := range pods {
		if !podutils.IsPodAvailable(pod, r.runningITS.Spec.MinReadySeconds, metav1.Time{Time: time.Now()}) {
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

// isRunning checks if the component underlying workload is running.
func (r *componentStatusHandler) isInstanceSetRunning() (bool, error) {
	if r.runningITS == nil {
		return false, nil
	}
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningITS); err != nil {
		return false, err
	} else if !isLatestRevision {
		r.reqCtx.Log.Info("underlying workload is not the latest revision")
		return false, nil
	}

	// whether the ITS is ready
	return instanceset.IsInstanceSetReady(r.runningITS), nil
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
		if err := r.cli.Get(r.reqCtx.Ctx, cmKey, cmObj, inDataContext4C()); err != nil {
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
	if r.runningITS == nil {
		return false, nil
	}
	if r.runningITS.Spec.Replicas == nil {
		return false, nil
	}
	if r.synthesizeComp.Replicas <= *r.runningITS.Spec.Replicas {
		return false, nil
	}

	backupKey := types.NamespacedName{
		Namespace: r.runningITS.Namespace,
		Name:      constant.GenerateResourceNameWithScalingSuffix(r.runningITS.Name),
	}
	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, r.runningITS, r.protoITS, backupKey)
	if err != nil {
		return false, err
	}
	if status, err := d.CheckBackupStatus(); err != nil {
		return false, err
	} else if status == backupStatusFailed {
		return true, nil
	}
	for i := *r.runningITS.Spec.Replicas; i < r.synthesizeComp.Replicas; i++ {
		if status, err := d.CheckRestoreStatus(i); err != nil {
			return false, err
		} else if status == dpv1alpha1.RestorePhaseFailed {
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
	for _, vct := range r.runningITS.Spec.VolumeClaimTemplates {
		volumes, err := getRunningVolumes(r.reqCtx.Ctx, r.cli, r.synthesizeComp, r.runningITS, vct.Name)
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

// hasFailedPod checks if the component has failed pod.
// TODO(xingran): remove the dependency of the component's workload type.
func (r *componentStatusHandler) hasFailedPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, error) {
	if isLatestRevision, err := component.IsComponentPodsWithLatestRevision(r.reqCtx.Ctx, r.cli, r.cluster, r.runningITS); err != nil {
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

// hasFailedAndTimedOutPod returns whether the pods of components are still failed after a PodFailedTimeout period.
func hasFailedAndTimedOutPod(pods []*corev1.Pod) (bool, appsv1alpha1.ComponentMessageMap, time.Duration) {
	var (
		hasTimedOutPod bool
		messages       = appsv1alpha1.ComponentMessageMap{}
		hasFailedPod   bool
		requeueAfter   time.Duration
	)
	for _, pod := range pods {
		isFailed, isTimedOut, messageStr := intctrlutil.IsPodFailedAndTimedOut(pod)
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
		requeueAfter = intctrlutil.PodContainerFailedTimeout
	}
	return hasTimedOutPod, messages, requeueAfter
}

// newComponentStatusHandler creates a new componentStatusHandler
func newComponentStatusHandler(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	comp *appsv1alpha1.Component,
	synthesizeComp *component.SynthesizedComponent,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet,
	dag *graph.DAG) *componentStatusHandler {
	return &componentStatusHandler{
		cli:            cli,
		reqCtx:         reqCtx,
		cluster:        cluster,
		comp:           comp,
		synthesizeComp: synthesizeComp,
		runningITS:     runningITS,
		protoITS:       protoITS,
		dag:            dag,
		podsReady:      false,
	}
}
