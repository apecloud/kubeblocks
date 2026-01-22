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

package component

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentWorkloadOps struct {
	transCtx       *componentTransformContext
	cli            client.Client
	component      *appsv1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	// runningITS is a snapshot of the InstanceSet that is already running
	runningITS *workloads.InstanceSet
	// protoITS is the InstanceSet object that is rebuilt from scratch during each reconcile process
	protoITS              *workloads.InstanceSet
	desiredCompPodNameSet sets.Set[string]
	runningItsPodNameSet  sets.Set[string]
}

func newComponentWorkloadOps(transCtx *componentTransformContext,
	cli client.Client,
	synthesizedComp *component.SynthesizedComponent,
	comp *appsv1.Component,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet,
	dag *graph.DAG) (*componentWorkloadOps, error) {
	runningITSPodNames, err := component.GetCurrentPodNamesByITS(runningITS)
	if err != nil {
		return nil, err
	}
	protoITSPodNames, err := component.GetDesiredPodNamesByITS(runningITS, protoITS)
	if err != nil {
		return nil, err
	}
	return &componentWorkloadOps{
		transCtx:              transCtx,
		cli:                   cli,
		component:             comp,
		synthesizeComp:        synthesizedComp,
		runningITS:            runningITS,
		protoITS:              protoITS,
		dag:                   dag,
		desiredCompPodNameSet: sets.New(protoITSPodNames...),
		runningItsPodNameSet:  sets.New(runningITSPodNames...),
	}, nil
}

func (r *componentWorkloadOps) horizontalScale() error {
	var (
		in  = r.runningItsPodNameSet.Difference(r.desiredCompPodNameSet)
		out = r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet)
	)
	if in.Len() == 0 && out.Len() == 0 {
		return r.postHorizontalScale() // TODO: how about consecutive horizontal scales?
	}

	if in.Len() > 0 {
		if err := r.scaleIn(); err != nil {
			return err
		}
	}

	if out.Len() > 0 {
		if err := r.scaleOut(); err != nil {
			return err
		}
	}

	r.transCtx.EventRecorder.Eventf(r.component,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		r.synthesizeComp.Name, r.synthesizeComp.ClusterName, int(*r.runningITS.Spec.Replicas), r.synthesizeComp.Replicas)

	return nil
}

func (r *componentWorkloadOps) scaleIn() error {
	if r.synthesizeComp.Replicas == 0 && len(r.synthesizeComp.VolumeClaimTemplates) > 0 {
		if r.synthesizeComp.PVCRetentionPolicy.WhenScaled != appsv1.RetainPersistentVolumeClaimRetentionPolicyType {
			return fmt.Errorf("when intending to scale-in to 0, only the \"Retain\" option is supported for the PVC retention policy")
		}
	}

	deleteReplicas := r.runningItsPodNameSet.Difference(r.desiredCompPodNameSet).UnsortedList()
	joinedReplicas := make([]string, 0)
	err := component.DeleteReplicasStatus(r.protoITS, deleteReplicas, func(s component.ReplicaStatus) {
		// has no member join defined or has joined successfully
		if s.Provisioned && (s.MemberJoined == nil || *s.MemberJoined) {
			joinedReplicas = append(joinedReplicas, s.Name)
		}
	})
	if err != nil {
		return err
	}

	// TODO: check the component definition to determine whether we need to call leave member before deleting replicas.
	if err := r.leaveMember4ScaleIn(deleteReplicas, joinedReplicas); err != nil {
		r.transCtx.Logger.Error(err, "leave member at scale-in error")
		return err
	}
	return nil
}

func (r *componentWorkloadOps) leaveMember4ScaleIn(deleteReplicas, joinedReplicas []string) error {
	pods, err := component.ListOwnedPods(r.transCtx.Context, r.cli,
		r.synthesizeComp.Namespace, r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
	if err != nil {
		return err
	}

	deleteReplicasSet := sets.New(deleteReplicas...)
	joinedReplicasSet := sets.New(joinedReplicas...)
	hasMemberLeaveDefined := r.synthesizeComp.LifecycleActions.ComponentLifecycleActions != nil && r.synthesizeComp.LifecycleActions.MemberLeave != nil
	r.transCtx.Logger.Info("leave member at scaling-in", "delete replicas", deleteReplicas,
		"joined replicas", joinedReplicas, "has member-leave action defined", hasMemberLeaveDefined)

	leaveErrors := make([]error, 0)
	for _, pod := range pods {
		if deleteReplicasSet.Has(pod.Name) {
			if joinedReplicasSet.Has(pod.Name) { // else: hasn't joined yet, no need to leave
				if err = r.leaveMemberForPod(pod, pods); err != nil {
					leaveErrors = append(leaveErrors, err)
				}
				joinedReplicasSet.Delete(pod.Name)
			}
			deleteReplicasSet.Delete(pod.Name)
		}
	}

	if hasMemberLeaveDefined && len(joinedReplicasSet) > 0 {
		leaveErrors = append(leaveErrors,
			fmt.Errorf("some replicas have joined but not leaved since the Pod object is not exist: %v", sets.List(joinedReplicasSet)))
	}
	if len(leaveErrors) > 0 {
		return intctrlutil.NewRequeueError(time.Second, fmt.Sprintf("%v", leaveErrors))
	}
	return nil
}

func (r *componentWorkloadOps) leaveMemberForPod(pod *corev1.Pod, pods []*corev1.Pod) error {
	var (
		synthesizedComp  = r.synthesizeComp
		lifecycleActions = synthesizedComp.LifecycleActions
	)

	switchover := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		if lifecycleActions.Switchover == nil {
			return nil
		}
		err := lfa.Switchover(r.transCtx.Context, r.cli, nil, "")
		if err == nil {
			r.transCtx.Logger.Info("succeed to call switchover action", "pod", pod.Name)
		} else if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			r.transCtx.Logger.Info("failed to call switchover action, ignore it", "pod", pod.Name, "error", err)
		}
		return nil
	}

	leaveMember := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		if lifecycleActions.MemberLeave == nil {
			return nil
		}
		err := lfa.MemberLeave(r.transCtx.Context, r.cli, nil)
		if err != nil {
			if errors.Is(err, lifecycle.ErrActionNotDefined) {
				return nil
			}
			return err
		}
		r.transCtx.Logger.Info("succeed to call leave member action", "pod", pod.Name)
		return nil
	}

	if lifecycleActions.ComponentLifecycleActions == nil ||
		(lifecycleActions.Switchover == nil && lifecycleActions.MemberLeave == nil) {
		return nil
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		lifecycleActions.ComponentLifecycleActions, synthesizedComp.TemplateVars, pod, pods)
	if err != nil {
		return err
	}

	if err = switchover(lfa, pod); err != nil {
		return err
	}
	if err = leaveMember(lfa, pod); err != nil {
		return err
	}
	return nil
}

func (r *componentWorkloadOps) scaleOut() error {
	if err := r.buildDataReplicationTask(); err != nil {
		return err
	}

	// replicas to be created
	newReplicas := r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet).UnsortedList()
	hasMemberJoinDefined, hasDataActionDefined := hasMemberJoinNDataActionDefined(r.synthesizeComp.LifecycleActions.ComponentLifecycleActions)
	return component.NewReplicasStatus(r.protoITS, newReplicas, hasMemberJoinDefined, hasDataActionDefined)
}

func (r *componentWorkloadOps) buildDataReplicationTask() error {
	_, hasDataActionDefined := hasMemberJoinNDataActionDefined(r.synthesizeComp.LifecycleActions.ComponentLifecycleActions)
	if !hasDataActionDefined {
		return nil
	}

	// replicas to be provisioned
	newReplicas := r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet).UnsortedList()
	// replicas in provisioning that the data has not been loaded
	provisioningReplicas, err := component.GetReplicasStatusFunc(r.protoITS, func(s component.ReplicaStatus) bool {
		return s.DataLoaded != nil && !*s.DataLoaded
	})
	if err != nil {
		return err
	}

	if len(newReplicas) == 0 && len(provisioningReplicas) == 0 {
		return nil
	}

	// the source replica
	source, err := r.sourceReplica(r.synthesizeComp.LifecycleActions.DataDump, provisioningReplicas)
	if err != nil {
		return err
	}

	replicas := append(slices.Clone(newReplicas), provisioningReplicas...)
	parameters, err := component.NewReplicaTask(r.synthesizeComp.FullCompName, r.synthesizeComp.Generation, source, replicas)
	if err != nil {
		return err
	}
	// apply the updated env to the env CM
	transCtx := &componentTransformContext{
		Context:             r.transCtx.Context,
		Client:              model.NewGraphClient(r.cli),
		SynthesizeComponent: r.synthesizeComp,
		Component:           r.component,
	}
	return createOrUpdateEnvConfigMap(transCtx, r.dag, nil, parameters)
}

func (r *componentWorkloadOps) sourceReplica(dataDump *appsv1.Action, provisioningReplicas []string) (*corev1.Pod, error) {
	pods, err := component.ListOwnedPods(r.transCtx.Context, r.cli,
		r.synthesizeComp.Namespace, r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if len(provisioningReplicas) > 0 {
		// exclude provisioning replicas
		pods = slices.DeleteFunc(pods, func(pod *corev1.Pod) bool {
			return slices.Contains(provisioningReplicas, pod.Name)
		})
	}
	if len(pods) > 0 {
		if len(dataDump.TargetPodSelector) == 0 && (dataDump.Exec == nil || len(dataDump.Exec.TargetPodSelector) == 0) {
			dataDump.TargetPodSelector = appsv1.AnyReplica
		}
		// TODO: idempotence for provisioning replicas
		pods, err = lifecycle.SelectTargetPods(pods, nil, dataDump)
		if err != nil {
			return nil, err
		}
		if len(pods) > 0 {
			return pods[0], nil
		}
	}
	return nil, fmt.Errorf("no available pod to dump data")
}

func (r *componentWorkloadOps) postHorizontalScale() error {
	if err := r.postScaleOut(); err != nil {
		return err
	}
	return nil
}

func (r *componentWorkloadOps) postScaleOut() error {
	if err := r.buildDataReplicationTask(); err != nil {
		return err
	}
	if err := r.joinMember4ScaleOut(); err != nil {
		return err
	}
	return nil
}

func (r *componentWorkloadOps) joinMember4ScaleOut() error {
	pods, err := component.ListOwnedPods(r.transCtx.Context, r.cli,
		r.synthesizeComp.Namespace, r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
	if err != nil {
		return err
	}

	joinErrors := make([]error, 0)
	if err = component.UpdateReplicasStatusFunc(r.protoITS, func(replicas *component.ReplicasStatus) error {
		for _, pod := range pods {
			i := slices.IndexFunc(replicas.Status, func(r component.ReplicaStatus) bool {
				return r.Name == pod.Name
			})
			if i < 0 {
				continue // the pod is not in the replicas status?
			}

			status := replicas.Status[i]
			if status.MemberJoined == nil || *status.MemberJoined {
				continue // no need to join or already joined
			}

			// TODO: should wait for the data to be loaded before joining the member?

			if err := r.joinMemberForPod(pod, pods); err != nil {
				joinErrors = append(joinErrors, fmt.Errorf("pod %s: %w", pod.Name, err))
			} else {
				replicas.Status[i].MemberJoined = ptr.To(true)
			}
		}

		notJoinedReplicas := make([]string, 0)
		for _, r := range replicas.Status {
			if r.MemberJoined != nil && !*r.MemberJoined {
				notJoinedReplicas = append(notJoinedReplicas, r.Name)
			}
		}
		if len(notJoinedReplicas) > 0 {
			joinErrors = append(joinErrors, fmt.Errorf("some replicas have not joined: %v", notJoinedReplicas))
		}
		return nil
	}); err != nil {
		return err
	}

	if len(joinErrors) > 0 {
		return intctrlutil.NewRequeueError(time.Second, fmt.Sprintf("%v", joinErrors))
	}
	return nil
}

func (r *componentWorkloadOps) joinMemberForPod(pod *corev1.Pod, pods []*corev1.Pod) error {
	synthesizedComp := r.synthesizeComp
	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		synthesizedComp.LifecycleActions.ComponentLifecycleActions, synthesizedComp.TemplateVars, pod, pods)
	if err != nil {
		return err
	}
	if err = lfa.MemberJoin(r.transCtx.Context, r.cli, nil); err != nil {
		if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			return err
		}
	}
	r.transCtx.Logger.Info("succeed to join member for pod", "pod", pod.Name)
	return nil
}

func (r *componentWorkloadOps) reconfigure() error {
	runningObjs, protoObjs, err := prepareFileTemplateObjects(r.transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	return r.handleReconfigure(r.transCtx, runningObjs, protoObjs, toCreate, toDelete, toUpdate)
}

func (r *componentWorkloadOps) handleReconfigure(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)

	if r.runningITS == nil {
		r.protoITS.Spec.Configs = nil
		return nil // the workload hasn't been provisioned
	}

	if len(toCreate) > 0 || len(toDelete) > 0 {
		// since pod volumes changed, the workload will be restarted
		r.protoITS.Spec.Configs = nil
		return nil
	}

	templateChanges := r.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
	for objName := range toUpdate {
		tplName := fileTemplateNameFromObject(transCtx.SynthesizeComponent, protoObjs[objName])
		if _, ok := templateChanges[tplName]; !ok {
			continue
		}
		for _, tpl := range synthesizedComp.FileTemplates {
			if tpl.Name == tplName {
				if ptr.Deref(tpl.RestartOnFileChange, false) {
					// restart
					if r.protoITS.Spec.Template.Annotations == nil {
						r.protoITS.Spec.Template.Annotations = map[string]string{}
					}
					r.protoITS.Spec.Template.Annotations[constant.RestartAnnotationKey] = metav1.NowMicro().Format(time.RFC3339)
					return nil
				}
			}
		}
	}

	reconfigure := func(tpl component.SynthesizedFileTemplate, changes fileTemplateChanges) {
		var (
			action     *appsv1.Action
			actionName string
		)
		if ptr.Deref(tpl.ExternalManaged, false) {
			if tpl.Reconfigure == nil {
				return // disabled by the external system
			}
		}
		action = tpl.Reconfigure
		actionName = component.UDFReconfigureActionName(tpl)
		if action == nil && synthesizedComp.LifecycleActions.ComponentLifecycleActions != nil {
			action = synthesizedComp.LifecycleActions.Reconfigure
			actionName = "" // default reconfigure action
		}
		if action == nil {
			return // has no reconfigure action defined
		}

		config := workloads.ConfigTemplate{
			Name:                  tpl.Name,
			Generation:            r.component.Generation,
			Reconfigure:           action,
			ReconfigureActionName: actionName,
			Parameters:            lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated),
		}
		if r.protoITS.Spec.Configs == nil {
			r.protoITS.Spec.Configs = make([]workloads.ConfigTemplate, 0)
		}
		idx := slices.IndexFunc(r.protoITS.Spec.Configs, func(cfg workloads.ConfigTemplate) bool {
			return cfg.Name == tpl.Name
		})
		if idx >= 0 {
			r.protoITS.Spec.Configs[idx] = config
		} else {
			r.protoITS.Spec.Configs = append(r.protoITS.Spec.Configs, config)
		}
	}

	// make a copy of configs from the running ITS
	r.protoITS.Spec.Configs = slices.Clone(r.runningITS.Spec.Configs)

	for _, tpl := range synthesizedComp.FileTemplates {
		if changes, ok := templateChanges[tpl.Name]; ok {
			reconfigure(tpl, changes)
		}
	}
	return nil
}

func (r *componentWorkloadOps) templateFileChanges(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, update sets.Set[string]) map[string]fileTemplateChanges {
	diff := func(obj *corev1.ConfigMap, rData, pData map[string]string) fileTemplateChanges {
		var (
			tplName = fileTemplateNameFromObject(transCtx.SynthesizeComponent, obj)
			items   = make([][]string, 3)
		)

		toAdd, toDelete, toUpdate := mapDiff(rData, pData)

		items[0], items[1] = sets.List(toAdd), sets.List(toDelete)
		for item := range toUpdate {
			if !reflect.DeepEqual(rData[item], pData[item]) {
				absPath := r.absoluteFilePath(transCtx, tplName, item)
				if len(absPath) > 0 {
					checksum := sha256.Sum256([]byte(pData[item]))
					items[2] = append(items[2], fmt.Sprintf("%s:%x", absPath, checksum))
				}
			}
		}

		for i := range items {
			slices.Sort(items[i])
		}

		return fileTemplateChanges{
			Created: strings.Join(items[0], ","),
			Removed: strings.Join(items[1], ","),
			Updated: strings.Join(items[2], ","),
		}
	}

	result := make(map[string]fileTemplateChanges)
	for name := range update {
		rData, pData := runningObjs[name].Data, protoObjs[name].Data
		if !reflect.DeepEqual(rData, pData) {
			tplName := fileTemplateNameFromObject(transCtx.SynthesizeComponent, runningObjs[name])
			result[tplName] = diff(runningObjs[name], rData, pData)
		}
	}
	return result
}

func (r *componentWorkloadOps) absoluteFilePath(transCtx *componentTransformContext, tpl, file string) string {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)

	var volName, mountPath string
	for _, fileTpl := range synthesizedComp.FileTemplates {
		if fileTpl.Name == tpl {
			volName = fileTpl.VolumeName
			break
		}
	}
	if volName == "" {
		return "" // has no volumes specified
	}

	for _, container := range synthesizedComp.PodSpec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volName {
				mountPath = mount.MountPath
				break
			}
		}
		if mountPath != "" {
			break
		}
	}
	if mountPath == "" {
		return "" // the template is not mounted, ignore it
	}

	return filepath.Join(mountPath, file)
}

type fileTemplateChanges struct {
	Created string `json:"created,omitempty"`
	Removed string `json:"removed,omitempty"`
	Updated string `json:"updated,omitempty"`
}
