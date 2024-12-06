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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/component/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	// TODO: use replicas status
	stopReplicasSnapshotKey = "apps.kubeblocks.io/stop-replicas-snapshot"
)

// componentWorkloadTransformer handles component workload generation
type componentWorkloadTransformer struct {
	client.Client
}

// componentWorkloadOps handles component workload ops
type componentWorkloadOps struct {
	cli            client.Client
	reqCtx         intctrlutil.RequestCtx
	cluster        *appsv1.Cluster
	component      *appsv1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	// runningITS is a snapshot of the InstanceSet that is already running
	runningITS *workloads.InstanceSet
	// protoITS is the InstanceSet object that is rebuilt from scratch during each reconcile process
	protoITS              *workloads.InstanceSet
	desiredCompPodNames   []string
	runningItsPodNames    []string
	desiredCompPodNameSet sets.Set[string]
	runningItsPodNameSet  sets.Set[string]
}

var _ graph.Transformer = &componentWorkloadTransformer{}

func (t *componentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	cluster := transCtx.Cluster
	compDef := transCtx.CompDef
	comp := transCtx.Component
	synthesizeComp := transCtx.SynthesizeComponent
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	runningITS, err := t.runningInstanceSetObject(ctx, synthesizeComp)
	if err != nil {
		return err
	}
	transCtx.RunningWorkload = runningITS

	// inject volume mounts and build its proto
	buildPodSpecVolumeMounts(synthesizeComp)
	protoITS, err := factory.BuildInstanceSet(synthesizeComp, compDef)
	if err != nil {
		return err
	}
	transCtx.ProtoWorkload = protoITS

	if err = t.reconcileWorkload(transCtx.Context, t.Client, synthesizeComp, comp, runningITS, protoITS); err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if runningITS == nil {
		if protoITS != nil {
			if err := setCompOwnershipNFinalizer(comp, protoITS); err != nil {
				return err
			}
			graphCli.Create(dag, protoITS)
			return nil
		}
	} else {
		if protoITS == nil {
			graphCli.Delete(dag, runningITS)
		} else {
			err = t.handleUpdate(reqCtx, graphCli, dag, cluster, synthesizeComp, comp, runningITS, protoITS)
		}
	}
	return err
}

func (t *componentWorkloadTransformer) runningInstanceSetObject(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent) (*workloads.InstanceSet, error) {
	objs, err := component.ListOwnedWorkloads(ctx.GetContext(), ctx.GetClient(),
		synthesizeComp.Namespace, synthesizeComp.ClusterName, synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, nil
	}
	return objs[0], nil
}

func (t *componentWorkloadTransformer) reconcileWorkload(ctx context.Context, cli client.Reader,
	synthesizedComp *component.SynthesizedComponent, comp *appsv1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	// if runningITS already exists, the image changes in protoITS will be
	// rollback to the original image in `checkNRollbackProtoImages`.
	// So changing registry configs won't affect existing clusters.
	for i, container := range protoITS.Spec.Template.Spec.Containers {
		protoITS.Spec.Template.Spec.Containers[i].Image = intctrlutil.ReplaceImageRegistry(container.Image)
	}
	for i, container := range protoITS.Spec.Template.Spec.InitContainers {
		protoITS.Spec.Template.Spec.InitContainers[i].Image = intctrlutil.ReplaceImageRegistry(container.Image)
	}

	buildInstanceSetPlacementAnnotation(comp, protoITS)

	if err := t.reconcileReplicasStatus(ctx, cli, synthesizedComp, runningITS, protoITS); err != nil {
		return err
	}

	// build configuration template annotations to workload
	configuration.BuildConfigTemplateAnnotations(protoITS, synthesizedComp)

	return nil
}

func (t *componentWorkloadTransformer) reconcileReplicasStatus(ctx context.Context, cli client.Reader,
	synthesizedComp *component.SynthesizedComponent, runningITS, protoITS *workloads.InstanceSet) error {
	var (
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)

	// HACK: sync replicas status from runningITS to protoITS
	component.BuildReplicasStatus(runningITS, protoITS)

	replicas, err := func() ([]string, error) {
		pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, compName)
		if err != nil {
			return nil, err
		}
		podNameSet := sets.New[string]()
		for _, pod := range pods {
			podNameSet.Insert(pod.Name)
		}

		desiredPodNames, err := generatePodNames(synthesizedComp)
		if err != nil {
			return nil, err
		}
		desiredPodNameSet := sets.New(desiredPodNames...)

		return desiredPodNameSet.Intersection(podNameSet).UnsortedList(), nil
	}()
	if err != nil {
		return err
	}

	hasMemberJoinDefined, hasDataActionDefined := hasMemberJoinNDataActionDefined(synthesizedComp.LifecycleActions)
	return component.StatusReplicasStatus(protoITS, replicas, hasMemberJoinDefined, hasDataActionDefined)
}

func hasMemberJoinNDataActionDefined(lifecycleActions *appsv1.ComponentLifecycleActions) (bool, bool) {
	if lifecycleActions == nil {
		return false, false
	}
	hasActionDefined := func(actions []*appsv1.Action) bool {
		for _, action := range actions {
			if action == nil || action.Exec == nil {
				return false
			}
		}
		return true
	}
	return hasActionDefined([]*appsv1.Action{lifecycleActions.MemberJoin}),
		hasActionDefined([]*appsv1.Action{lifecycleActions.DataDump, lifecycleActions.DataLoad})
}

func (t *componentWorkloadTransformer) handleUpdate(reqCtx intctrlutil.RequestCtx, cli model.GraphClient, dag *graph.DAG,
	cluster *appsv1.Cluster, synthesizedComp *component.SynthesizedComponent, comp *appsv1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	start, stop, err := t.handleWorkloadStartNStop(synthesizedComp, runningITS, &protoITS)
	if err != nil {
		return err
	}

	if !(start || stop) {
		// postpone the update of the workload until the component is back to running.
		if err := t.handleWorkloadUpdate(reqCtx, dag, cluster, synthesizedComp, comp, runningITS, protoITS); err != nil {
			return err
		}
	}

	objCopy := copyAndMergeITS(runningITS, protoITS)
	if objCopy != nil {
		cli.Update(dag, nil, objCopy, &model.ReplaceIfExistingOption{})
		// make sure the workload is updated after the env CM
		cli.DependOn(dag, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: synthesizedComp.Namespace,
				Name:      constant.GenerateClusterComponentEnvPattern(synthesizedComp.ClusterName, synthesizedComp.Name),
			},
		})
	}

	// if start {
	//	return intctrlutil.NewDelayedRequeueError(time.Second, "workload is starting")
	// }
	return nil
}

func (t *componentWorkloadTransformer) handleWorkloadStartNStop(
	synthesizedComp *component.SynthesizedComponent, runningITS *workloads.InstanceSet, protoITS **workloads.InstanceSet) (bool, bool, error) {
	var (
		stop  = isCompStopped(synthesizedComp)
		start = !stop && isWorkloadStopped(runningITS)
	)
	if start || stop {
		*protoITS = runningITS.DeepCopy() // don't modify the runningITS except for the replicas
	}
	if stop {
		return start, stop, t.stopWorkload(synthesizedComp, runningITS, *protoITS)
	}
	if start {
		return start, stop, t.startWorkload(synthesizedComp, runningITS, *protoITS)
	}
	return start, stop, nil
}

func isCompStopped(synthesizedComp *component.SynthesizedComponent) bool {
	return synthesizedComp.Stop != nil && *synthesizedComp.Stop
}

func isWorkloadStopped(runningITS *workloads.InstanceSet) bool {
	_, ok := runningITS.Annotations[stopReplicasSnapshotKey]
	return ok
}

func (t *componentWorkloadTransformer) stopWorkload(
	synthesizedComp *component.SynthesizedComponent, runningITS, protoITS *workloads.InstanceSet) error {
	// since its doesn't support stop, we achieve it by setting replicas to 0.
	protoITS.Spec.Replicas = ptr.To(int32(0))
	for i := range protoITS.Spec.Instances {
		protoITS.Spec.Instances[i].Replicas = ptr.To(int32(0))
	}

	// backup the replicas of runningITS
	snapshot, ok := runningITS.Annotations[stopReplicasSnapshotKey]
	if !ok {
		replicas := map[string]int32{}
		if runningITS.Spec.Replicas != nil {
			replicas[""] = *runningITS.Spec.Replicas
		}
		for i := range runningITS.Spec.Instances {
			if runningITS.Spec.Instances[i].Replicas != nil {
				replicas[protoITS.Spec.Instances[i].Name] = *runningITS.Spec.Instances[i].Replicas
			}
		}
		out, err := json.Marshal(replicas)
		if err != nil {
			return err
		}
		snapshot = string(out)

		protoITS.Annotations[constant.KubeBlocksGenerationKey] = synthesizedComp.Generation
	}
	protoITS.Annotations[stopReplicasSnapshotKey] = snapshot
	return nil
}

func (t *componentWorkloadTransformer) startWorkload(
	synthesizedComp *component.SynthesizedComponent, runningITS, protoITS *workloads.InstanceSet) error {
	snapshot := runningITS.Annotations[stopReplicasSnapshotKey]
	replicas := map[string]int32{}
	if err := json.Unmarshal([]byte(snapshot), &replicas); err != nil {
		return err
	}

	restore := func(p **int32, key string) {
		val, ok := replicas[key]
		if ok {
			*p = ptr.To(val)
		} else {
			*p = nil
		}
	}

	// restore the replicas of runningITS
	restore(&protoITS.Spec.Replicas, "")
	for i := range runningITS.Spec.Instances {
		for j := range protoITS.Spec.Instances {
			if runningITS.Spec.Instances[i].Name == protoITS.Spec.Instances[j].Name {
				restore(&protoITS.Spec.Instances[j].Replicas, runningITS.Spec.Instances[i].Name)
				break
			}
		}
	}

	delete(protoITS.Annotations, stopReplicasSnapshotKey)
	delete(runningITS.Annotations, stopReplicasSnapshotKey)

	return nil
}

func (t *componentWorkloadTransformer) handleWorkloadUpdate(reqCtx intctrlutil.RequestCtx, dag *graph.DAG,
	cluster *appsv1.Cluster, synthesizeComp *component.SynthesizedComponent, comp *appsv1.Component, obj, its *workloads.InstanceSet) error {
	cwo, err := newComponentWorkloadOps(reqCtx, t.Client, cluster, synthesizeComp, comp, obj, its, dag)
	if err != nil {
		return err
	}

	// handle expand volume
	if err := cwo.expandVolume(); err != nil {
		return err
	}

	// handle workload horizontal scale
	if err := cwo.horizontalScale(); err != nil {
		return err
	}

	return nil
}

// buildPodSpecVolumeMounts builds podSpec volumeMounts
func buildPodSpecVolumeMounts(synthesizeComp *component.SynthesizedComponent) {
	kbScriptAndConfigVolumeNames := make([]string, 0)
	for _, v := range synthesizeComp.ScriptTemplates {
		kbScriptAndConfigVolumeNames = append(kbScriptAndConfigVolumeNames, v.VolumeName)
	}
	for _, v := range synthesizeComp.ConfigTemplates {
		kbScriptAndConfigVolumeNames = append(kbScriptAndConfigVolumeNames, v.VolumeName)
	}

	podSpec := synthesizeComp.PodSpec
	for _, cc := range []*[]corev1.Container{&podSpec.Containers, &podSpec.InitContainers} {
		volumes := podSpec.Volumes
		for _, c := range *cc {
			for _, v := range c.VolumeMounts {
				// if volumeMounts belongs to kbScriptAndConfigVolumeNames, skip
				if slices.Contains(kbScriptAndConfigVolumeNames, v.Name) {
					continue
				}
				// if persistence is not found, add an emptyDir to pod.spec.volumes
				createFn := func(_ string) corev1.Volume {
					return corev1.Volume{
						Name: v.Name,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}
				}
				volumes, _ = intctrlutil.CreateOrUpdateVolume(volumes, v.Name, createFn, nil)
			}
		}
		podSpec.Volumes = volumes
	}
	synthesizeComp.PodSpec = podSpec
}

// copyAndMergeITS merges two ITS objects for updating:
//  1. new an object targetObj by copying from oldObj
//  2. merge all fields can be updated from newObj into targetObj
func copyAndMergeITS(oldITS, newITS *workloads.InstanceSet) *workloads.InstanceSet {

	updateUpdateStrategy := func(itsObj, itsProto *workloads.InstanceSet) {
		var objMaxUnavailable *intstr.IntOrString
		if itsObj.Spec.UpdateStrategy.RollingUpdate != nil {
			objMaxUnavailable = itsObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
		}
		itsObj.Spec.UpdateStrategy = itsProto.Spec.UpdateStrategy
		if objMaxUnavailable == nil && itsObj.Spec.UpdateStrategy.RollingUpdate != nil {
			// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
			// MaxUnavailableStatefulSet feature.
			// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
			// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
			// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
			itsObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = nil
		}
	}

	itsObjCopy := oldITS.DeepCopy()
	itsProto := newITS

	// If the service version and component definition are not updated, we should not update the images in workload.
	checkNRollbackProtoImages(itsObjCopy, itsProto)

	// remove original monitor annotations
	if len(itsObjCopy.Annotations) > 0 {
		maps.DeleteFunc(itsObjCopy.Annotations, func(k, v string) bool {
			return strings.HasPrefix(k, "monitor.kubeblocks.io")
		})
	}
	intctrlutil.MergeMetadataMapInplace(itsProto.Annotations, &itsObjCopy.Annotations)
	intctrlutil.MergeMetadataMapInplace(itsProto.Labels, &itsObjCopy.Labels)
	itsObjCopy.Spec.Template = *itsProto.Spec.Template.DeepCopy()
	itsObjCopy.Spec.Replicas = itsProto.Spec.Replicas
	itsObjCopy.Spec.Roles = itsProto.Spec.Roles
	itsObjCopy.Spec.RoleProbe = itsProto.Spec.RoleProbe
	itsObjCopy.Spec.MembershipReconfiguration = itsProto.Spec.MembershipReconfiguration
	itsObjCopy.Spec.MemberUpdateStrategy = itsProto.Spec.MemberUpdateStrategy
	itsObjCopy.Spec.Credential = itsProto.Spec.Credential
	itsObjCopy.Spec.Instances = itsProto.Spec.Instances
	itsObjCopy.Spec.OfflineInstances = itsProto.Spec.OfflineInstances
	itsObjCopy.Spec.MinReadySeconds = itsProto.Spec.MinReadySeconds
	itsObjCopy.Spec.VolumeClaimTemplates = itsProto.Spec.VolumeClaimTemplates
	itsObjCopy.Spec.ParallelPodManagementConcurrency = itsProto.Spec.ParallelPodManagementConcurrency
	itsObjCopy.Spec.PodUpdatePolicy = itsProto.Spec.PodUpdatePolicy

	if itsProto.Spec.UpdateStrategy.Type != "" || itsProto.Spec.UpdateStrategy.RollingUpdate != nil {
		updateUpdateStrategy(itsObjCopy, itsProto)
	}

	intctrlutil.ResolvePodSpecDefaultFields(oldITS.Spec.Template.Spec, &itsObjCopy.Spec.Template.Spec)
	delayUpdateInstanceSetSystemFields(oldITS.Spec, &itsObjCopy.Spec)

	isSpecUpdated := !reflect.DeepEqual(&oldITS.Spec, &itsObjCopy.Spec)
	if isSpecUpdated {
		updateInstanceSetSystemFields(itsProto.Spec, &itsObjCopy.Spec)
	}

	isLabelsUpdated := !reflect.DeepEqual(oldITS.Labels, itsObjCopy.Labels)
	isAnnotationsUpdated := !reflect.DeepEqual(oldITS.Annotations, itsObjCopy.Annotations)
	if !isSpecUpdated && !isLabelsUpdated && !isAnnotationsUpdated {
		return nil
	}
	return itsObjCopy
}

func checkNRollbackProtoImages(itsObj, itsProto *workloads.InstanceSet) {
	if itsObj.Annotations == nil || itsProto.Annotations == nil {
		return
	}

	annotationUpdated := func(key string) bool {
		using, ok1 := itsObj.Annotations[key]
		proto, ok2 := itsProto.Annotations[key]
		if !ok1 || !ok2 {
			return true
		}
		if len(using) == 0 || len(proto) == 0 {
			return true
		}
		return using != proto
	}

	compDefUpdated := func() bool {
		return annotationUpdated(constant.AppComponentLabelKey)
	}

	serviceVersionUpdated := func() bool {
		return annotationUpdated(constant.KBAppServiceVersionKey)
	}

	if compDefUpdated() || serviceVersionUpdated() {
		return
	}

	// otherwise, roll-back the images in proto
	images := make([]map[string]string, 2)
	for i, cc := range [][]corev1.Container{itsObj.Spec.Template.Spec.InitContainers, itsObj.Spec.Template.Spec.Containers} {
		images[i] = make(map[string]string)
		for _, c := range cc {
			// skip the kb-agent container
			if component.IsKBAgentContainer(&c) {
				continue
			}
			images[i][c.Name] = c.Image
		}
	}
	rollback := func(idx int, c *corev1.Container) {
		if image, ok := images[idx][c.Name]; ok {
			c.Image = image
		}
	}
	for i := range itsProto.Spec.Template.Spec.InitContainers {
		rollback(0, &itsProto.Spec.Template.Spec.InitContainers[i])
	}
	for i := range itsProto.Spec.Template.Spec.Containers {
		rollback(1, &itsProto.Spec.Template.Spec.Containers[i])
	}
}

// expandVolume handles workload expand volume
func (r *componentWorkloadOps) expandVolume() error {
	// 1. expand the volumes without instance template name.
	if err := r.expandVolumeClaimTemplates(r.runningITS.Spec.VolumeClaimTemplates, r.synthesizeComp.VolumeClaimTemplates, ""); err != nil {
		return err
	}
	if len(r.runningITS.Spec.Instances) == 0 {
		return nil
	}
	// 2. expand the volumes with instance template name.
	for i := range r.runningITS.Spec.Instances {
		runningInsSpec := r.runningITS.Spec.DeepCopy()
		runningInsTPL := runningInsSpec.Instances[i]
		intctrlutil.MergeList(&runningInsTPL.VolumeClaimTemplates, &runningInsSpec.VolumeClaimTemplates,
			func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
				return func(claim corev1.PersistentVolumeClaim) bool {
					return claim.Name == item.Name
				}
			})

		var protoVCTs []corev1.PersistentVolumeClaimTemplate
		protoVCTs = append(protoVCTs, r.synthesizeComp.VolumeClaimTemplates...)
		for _, v := range r.synthesizeComp.Instances {
			if runningInsTPL.Name == v.Name {
				insVCTs := component.ToVolumeClaimTemplates(v.VolumeClaimTemplates)
				intctrlutil.MergeList(&insVCTs, &protoVCTs,
					func(item corev1.PersistentVolumeClaimTemplate) func(corev1.PersistentVolumeClaimTemplate) bool {
						return func(claim corev1.PersistentVolumeClaimTemplate) bool {
							return claim.Name == item.Name
						}
					})
				break
			}
		}
		if err := r.expandVolumeClaimTemplates(runningInsSpec.VolumeClaimTemplates, protoVCTs, runningInsTPL.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *componentWorkloadOps) expandVolumeClaimTemplates(runningVCTs []corev1.PersistentVolumeClaim, protoVCTs []corev1.PersistentVolumeClaimTemplate, insTPLName string) error {
	for _, vct := range runningVCTs {
		var proto *corev1.PersistentVolumeClaimTemplate
		for i, v := range protoVCTs {
			if v.Name == vct.Name {
				proto = &protoVCTs[i]
				break
			}
		}
		// REVIEW: seems we can remove a volume claim from templates at runtime, without any changes and warning messages?
		if proto == nil {
			continue
		}

		if err := r.expandVolumes(insTPLName, vct.Name, proto); err != nil {
			return err
		}
	}
	return nil
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

	r.reqCtx.Recorder.Eventf(r.cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		r.synthesizeComp.Name, r.cluster.Name, int(*r.runningITS.Spec.Replicas), r.synthesizeComp.Replicas)

	return nil
}

func (r *componentWorkloadOps) scaleIn() error {
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
		r.reqCtx.Log.Error(err, "leave member at scaling-in error")
		return err
	}

	// TODO: if scale in to 0, do not delete pvcs, remove this later
	if r.synthesizeComp.Replicas == 0 {
		r.reqCtx.Log.Info("scale in to 0, keep all PVCs")
		return nil
	}

	return r.deletePVCs4ScaleIn(r.runningITS)
}

func (r *componentWorkloadOps) leaveMember4ScaleIn(deleteReplicas, joinedReplicas []string) error {
	pods, err := component.ListOwnedPods(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, r.cluster.Name, r.synthesizeComp.Name)
	if err != nil {
		return err
	}

	deleteReplicasSet := sets.New(deleteReplicas...)
	joinedReplicasSet := sets.New(joinedReplicas...)
	hasMemberLeaveDefined := r.synthesizeComp.LifecycleActions != nil && r.synthesizeComp.LifecycleActions.MemberLeave != nil
	r.reqCtx.Log.Info("leave member at scaling-in", "delete replicas", deleteReplicas,
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
		return newRequeueError(time.Second, fmt.Sprintf("%v", leaveErrors))
	}
	return nil
}

func (r *componentWorkloadOps) leaveMemberForPod(pod *corev1.Pod, pods []*corev1.Pod) error {
	isLeader := func(pod *corev1.Pod) bool {
		if pod == nil || len(pod.Labels) == 0 {
			return false
		}
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

	tryToSwitchover := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		// if pod is not leader/primary, no need to switchover
		if !isLeader(pod) {
			return nil
		}
		// if HA functionality is not enabled, no need to switchover
		err := lfa.Switchover(r.reqCtx.Ctx, r.cli, nil, "")
		if err != nil && errors.Is(err, lifecycle.ErrActionNotDefined) {
			return nil
		}
		if err == nil {
			return fmt.Errorf("switchover succeed, wait role label to be updated")
		}
		return err
	}

	if !(isLeader(pod) || // if the pod is leader, it needs to call switchover
		(r.synthesizeComp.LifecycleActions != nil && r.synthesizeComp.LifecycleActions.MemberLeave != nil)) { // if the memberLeave action is defined, it needs to call it
		return nil
	}

	lfa, err := lifecycle.New(r.synthesizeComp, pod, pods...)
	if err != nil {
		return err
	}

	// switchover if the leaving pod is leader
	if switchoverErr := tryToSwitchover(lfa, pod); switchoverErr != nil {
		return switchoverErr
	}

	if err = lfa.MemberLeave(r.reqCtx.Ctx, r.cli, nil); err != nil {
		if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			return err
		}
	}
	r.reqCtx.Log.Info("succeed to leave member for pod", "pod", pod.Name)
	return nil
}

func (r *componentWorkloadOps) deletePVCs4ScaleIn(itsObj *workloads.InstanceSet) error {
	graphCli := model.NewGraphClient(r.cli)
	for _, podName := range r.runningItsPodNames {
		if _, ok := r.desiredCompPodNameSet[podName]; ok {
			continue
		}
		for _, vct := range itsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", vct.Name, podName),
			}
			pvc := corev1.PersistentVolumeClaim{}
			if err := r.cli.Get(r.reqCtx.Ctx, pvcKey, &pvc, inDataContext4C()); err != nil {
				if apierrors.IsNotFound(err) {
					continue // the pvc is already deleted or not created
				}
				return err
			}
			// Since there are no order guarantee between updating ITS and deleting PVCs, if there is any error occurred
			// after updating ITS and before deleting PVCs, the PVCs intended to scale-in will be leaked.
			// For simplicity, the updating dependency is added between them to guarantee that the PVCs to scale-in
			// will be deleted or the scaling-in operation will be failed.
			graphCli.Delete(r.dag, &pvc, inDataContext4G())
		}
	}
	return nil
}

func (r *componentWorkloadOps) scaleOut() error {
	// replicas in provisioning that the data has not been loaded
	provisioningReplicas, err := component.GetReplicasStatusFunc(r.protoITS, func(s component.ReplicaStatus) bool {
		return s.DataLoaded != nil && !*s.DataLoaded
	})
	if err != nil {
		return err
	}

	// replicas to be created
	newReplicas := r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet).UnsortedList()

	hasMemberJoinDefined, hasDataActionDefined := hasMemberJoinNDataActionDefined(r.synthesizeComp.LifecycleActions)

	// build and assign data replication tasks
	if err := func() error {
		if !hasDataActionDefined {
			return nil
		}

		source, err := r.sourceReplica(r.synthesizeComp.LifecycleActions.DataDump)
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
			Context:             r.reqCtx.Ctx,
			Client:              model.NewGraphClient(r.cli),
			SynthesizeComponent: r.synthesizeComp,
			Component:           r.component,
		}
		if err = createOrUpdateEnvConfigMap(transCtx, r.dag, nil, parameters); err != nil {
			return err
		}
		return nil
	}(); err != nil {
		return err
	}

	return component.NewReplicasStatus(r.protoITS, newReplicas, hasMemberJoinDefined, hasDataActionDefined)
}

func (r *componentWorkloadOps) sourceReplica(dataDump *appsv1.Action) (*corev1.Pod, error) {
	pods, err := component.ListOwnedPods(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, r.cluster.Name, r.synthesizeComp.Name)
	if err != nil {
		return nil, err
	}
	if len(pods) > 0 {
		if len(dataDump.Exec.TargetPodSelector) == 0 {
			dataDump.Exec.TargetPodSelector = appsv1.AnyReplica
		}
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
	if err := r.joinMember4ScaleOut(); err != nil {
		return err
	}
	return nil
}

func (r *componentWorkloadOps) joinMember4ScaleOut() error {
	pods, err := component.ListOwnedPods(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, r.cluster.Name, r.synthesizeComp.Name)
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
		return newRequeueError(time.Second, fmt.Sprintf("%v", joinErrors))
	}
	return nil
}

func (r *componentWorkloadOps) joinMemberForPod(pod *corev1.Pod, pods []*corev1.Pod) error {
	lfa, err := lifecycle.New(r.synthesizeComp, pod, pods...)
	if err != nil {
		return err
	}
	if err = lfa.MemberJoin(r.reqCtx.Ctx, r.cli, nil); err != nil {
		if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			return err
		}
	}
	r.reqCtx.Log.Info("succeed to join member for pod", "pod", pod.Name)
	return nil
}

func (r *componentWorkloadOps) expandVolumes(insTPLName string, vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	for _, pod := range r.runningItsPodNames {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: r.cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", vctName, pod),
		}
		pvcNotFound := false
		if err := r.cli.Get(r.reqCtx.Ctx, pvcKey, pvc, inDataContext4C()); err != nil {
			if apierrors.IsNotFound(err) {
				pvcNotFound = true
			} else {
				return err
			}
		}
		if insTPLName != pvc.Labels[constant.KBAppComponentInstanceTemplateLabelKey] {
			continue
		}
		if !pvcNotFound {
			quantity := pvc.Spec.Resources.Requests.Storage()
			newQuantity := proto.Spec.Resources.Requests.Storage()
			if quantity.Cmp(*pvc.Status.Capacity.Storage()) == 0 && newQuantity.Cmp(*quantity) < 0 {
				errMsg := fmt.Sprintf("shrinking the volume is not supported, volume: %s, quantity: %s, new quantity: %s",
					pvc.GetName(), quantity.String(), newQuantity.String())
				r.reqCtx.Event(r.cluster, corev1.EventTypeWarning, "VolumeExpansionFailed", errMsg)
				return fmt.Errorf("%s", errMsg)
			}
		}

		if err := r.updatePVCSize(pvcKey, pvc, pvcNotFound, proto); err != nil {
			return err
		}
	}
	return nil
}

func (r *componentWorkloadOps) updatePVCSize(pvcKey types.NamespacedName,
	pvc *corev1.PersistentVolumeClaim, pvcNotFound bool, vctProto *corev1.PersistentVolumeClaimTemplate) error {
	// reference: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#recovering-from-failure-when-expanding-volumes
	// 1. Mark the PersistentVolume(PV) that is bound to the PersistentVolumeClaim(PVC) with Retain reclaim policy.
	// 2. Delete the PVC. Since PV has Retain reclaim policy - we will not lose any data when we recreate the PVC.
	// 3. Delete the claimRef entry from PV specs, so as new PVC can bind to it. This should make the PV Available.
	// 4. Re-create the PVC with smaller size than PV and set volumeName field of the PVC to the name of the PV. This should bind new PVC to existing PV.
	// 5. Don't forget to restore the reclaim policy of the PV.
	newPVC := pvc.DeepCopy()
	if pvcNotFound {
		newPVC.Name = pvcKey.Name
		newPVC.Namespace = pvcKey.Namespace
		newPVC.SetLabels(vctProto.Labels)
		newPVC.Spec = vctProto.Spec
		ml := client.MatchingLabels{
			constant.PVCNameLabelKey: pvcKey.Name,
		}
		pvList := corev1.PersistentVolumeList{}
		if err := r.cli.List(r.reqCtx.Ctx, &pvList, ml, inDataContext4C()); err != nil {
			return err
		}
		for _, pv := range pvList.Items {
			// find pv referenced this pvc
			if pv.Spec.ClaimRef == nil {
				continue
			}
			if pv.Spec.ClaimRef.Name == pvcKey.Name {
				newPVC.Spec.VolumeName = pv.Name
				break
			}
		}
	} else {
		newPVC.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
		// delete annotation to make it re-bind
		delete(newPVC.Annotations, "pv.kubernetes.io/bind-completed")
	}

	pvNotFound := false

	// step 1: update pv to retain
	pv := &corev1.PersistentVolume{}
	if len(newPVC.Spec.VolumeName) == 0 {
		// the PV may be under provisioning
		pvNotFound = true
	} else {
		pvKey := types.NamespacedName{
			Namespace: pvcKey.Namespace,
			Name:      newPVC.Spec.VolumeName,
		}
		if err := r.cli.Get(r.reqCtx.Ctx, pvKey, pv, inDataContext4C()); err != nil {
			if apierrors.IsNotFound(err) {
				pvNotFound = true
			} else {
				return err
			}
		}
	}

	graphCli := model.NewGraphClient(r.cli)

	type pvcRecreateStep int
	const (
		pvPolicyRetainStep pvcRecreateStep = iota
		deletePVCStep
		removePVClaimRefStep
		createPVCStep
		pvRestorePolicyStep
	)

	addStepMap := map[pvcRecreateStep]func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex{
		pvPolicyRetainStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 1: update pv to retain
			retainPV := pv.DeepCopy()
			if retainPV.Labels == nil {
				retainPV.Labels = make(map[string]string)
			}
			// add label to pv, in case pvc get deleted, and we can't find pv
			retainPV.Labels[constant.PVCNameLabelKey] = pvcKey.Name
			if retainPV.Annotations == nil {
				retainPV.Annotations = make(map[string]string)
			}
			retainPV.Annotations[constant.PVLastClaimPolicyAnnotationKey] = string(pv.Spec.PersistentVolumeReclaimPolicy)
			retainPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
			return graphCli.Do(r.dag, pv, retainPV, model.ActionPatchPtr(), fromVertex, inDataContext4G())
		},
		deletePVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := graphCli.Do(r.dag, pvc, removeFinalizerPVC, model.ActionPatchPtr(), fromVertex, inDataContext4G())
			return graphCli.Do(r.dag, nil, removeFinalizerPVC, model.ActionDeletePtr(), removeFinalizerPVCVertex, inDataContext4G())
		},
		removePVClaimRefStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return graphCli.Do(r.dag, pv, removeClaimRefPV, model.ActionPatchPtr(), fromVertex, inDataContext4G())
		},
		createPVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return graphCli.Do(r.dag, nil, newPVC, model.ActionCreatePtr(), fromVertex, inDataContext4G())
		},
		pvRestorePolicyStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return graphCli.Do(r.dag, pv, restorePV, model.ActionPatchPtr(), fromVertex, inDataContext4G())
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := r.buildProtoITSWorkloadVertex()
		// The steps here are decremented in reverse order because during the plan execution, dag.WalkReverseTopoOrder
		// is called to execute all vertices on the graph according to the reverse topological order.
		// Therefore, the vertices need to maintain the following edge linkages:
		// root -> its -> step5 -> step4 -> step3 -> step2 -> step1
		// So that, during execution, the sequence becomes step1 -> step2 -> step3 -> step4 -> step5
		for step := pvRestorePolicyStep; step >= fromStep && step >= pvPolicyRetainStep; step-- {
			lastVertex = addStepMap[step](lastVertex, step)
		}
	}

	targetQuantity := vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
	if pvcNotFound && !pvNotFound {
		// this could happen if create pvc step failed when recreating pvc
		updatePVCByRecreateFromStep(removePVClaimRefStep)
		return nil
	}
	if pvcNotFound && pvNotFound {
		// if both pvc and pv not found, do nothing
		return nil
	}
	if reflect.DeepEqual(pvc.Spec.Resources, newPVC.Spec.Resources) && pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
		// this could happen if create pvc succeeded but last step failed
		updatePVCByRecreateFromStep(pvRestorePolicyStep)
		return nil
	}
	if pvcQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; !viper.GetBool(constant.CfgRecoverVolumeExpansionFailure) &&
		pvcQuantity.Cmp(targetQuantity) == 1 && // check if it's compressing volume
		targetQuantity.Cmp(*pvc.Status.Capacity.Storage()) >= 0 { // check if target size is greater than or equal to actual size
		// this branch means we can update pvc size by recreate it
		updatePVCByRecreateFromStep(pvPolicyRetainStep)
		return nil
	}
	if pvcQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; pvcQuantity.Cmp(vctProto.Spec.Resources.Requests[corev1.ResourceStorage]) != 0 {
		// use pvc's update without anything extra
		graphCli.Update(r.dag, nil, newPVC, inDataContext4G())
		return nil
	}
	// all the else means no need to update

	return nil
}

// buildProtoITSWorkloadVertex builds protoITS workload vertex
func (r *componentWorkloadOps) buildProtoITSWorkloadVertex() *model.ObjectVertex {
	for _, vertex := range r.dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if v.Obj == r.protoITS {
			return v
		}
	}
	return nil
}

func getRunningVolumes(ctx context.Context, cli client.Client, synthesizedComp *component.SynthesizedComponent,
	itsObj *workloads.InstanceSet, vctName string) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := component.ListOwnedPVCs(ctx, cli, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	matchedPVCs := make([]*corev1.PersistentVolumeClaim, 0)
	prefix := fmt.Sprintf("%s-%s", vctName, itsObj.Name)
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, prefix) {
			matchedPVCs = append(matchedPVCs, pvc)
		}
	}
	return matchedPVCs, nil
}

func buildInstanceSetPlacementAnnotation(comp *appsv1.Component, its *workloads.InstanceSet) {
	p := placement(comp)
	if len(p) > 0 {
		if its.Annotations == nil {
			its.Annotations = make(map[string]string)
		}
		its.Annotations[constant.KBAppMultiClusterPlacementKey] = p
	}
}

func newComponentWorkloadOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1.Cluster,
	synthesizeComp *component.SynthesizedComponent,
	comp *appsv1.Component,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet,
	dag *graph.DAG) (*componentWorkloadOps, error) {
	compPodNames, err := generatePodNames(synthesizeComp)
	if err != nil {
		return nil, err
	}
	itsPodNames, err := generatePodNamesByITS(runningITS)
	if err != nil {
		return nil, err
	}
	return &componentWorkloadOps{
		cli:                   cli,
		reqCtx:                reqCtx,
		cluster:               cluster,
		component:             comp,
		synthesizeComp:        synthesizeComp,
		runningITS:            runningITS,
		protoITS:              protoITS,
		dag:                   dag,
		desiredCompPodNames:   compPodNames,
		runningItsPodNames:    itsPodNames,
		desiredCompPodNameSet: sets.New(compPodNames...),
		runningItsPodNameSet:  sets.New(itsPodNames...),
	}, nil
}
