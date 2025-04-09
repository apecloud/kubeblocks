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
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
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

var _ graph.Transformer = &componentWorkloadTransformer{}

func (t *componentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	compDef := transCtx.CompDef
	comp := transCtx.Component
	synthesizeComp := transCtx.SynthesizeComponent

	runningITS, err := t.runningInstanceSetObject(ctx, synthesizeComp)
	if err != nil {
		return err
	}
	transCtx.RunningWorkload = runningITS

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
			err = t.handleUpdate(transCtx, graphCli, dag, synthesizeComp, comp, runningITS, protoITS)
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

	t.buildInstanceSetPlacementAnnotation(comp, protoITS)

	if err := t.reconcileReplicasStatus(ctx, cli, synthesizedComp, runningITS, protoITS); err != nil {
		return err
	}

	return nil
}

func (t *componentWorkloadTransformer) buildInstanceSetPlacementAnnotation(comp *appsv1.Component, its *workloads.InstanceSet) {
	p := appsutil.Placement(comp)
	if len(p) > 0 {
		if its.Annotations == nil {
			its.Annotations = make(map[string]string)
		}
		its.Annotations[constant.KBAppMultiClusterPlacementKey] = p
	}
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

func (t *componentWorkloadTransformer) handleUpdate(transCtx *componentTransformContext, cli model.GraphClient, dag *graph.DAG,
	synthesizedComp *component.SynthesizedComponent, comp *appsv1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	start, stop, err := t.handleWorkloadStartNStop(synthesizedComp, runningITS, &protoITS)
	if err != nil {
		return err
	}

	if !(start || stop) {
		// postpone the update of the workload until the component is back to running.
		if err := t.handleWorkloadUpdate(transCtx, dag, synthesizedComp, comp, runningITS, protoITS); err != nil {
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

	// set the pvc retention policy as Retain explicitly
	if protoITS.Spec.PersistentVolumeClaimRetentionPolicy == nil {
		protoITS.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{}
	}
	protoITS.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = appsv1.RetainPersistentVolumeClaimRetentionPolicyType

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

func (t *componentWorkloadTransformer) handleWorkloadUpdate(transCtx *componentTransformContext, dag *graph.DAG,
	synthesizeComp *component.SynthesizedComponent, comp *appsv1.Component, obj, its *workloads.InstanceSet) error {
	cwo, err := newComponentWorkloadOps(transCtx, t.Client, synthesizeComp, comp, obj, its, dag)
	if err != nil {
		return err
	}
	if err := cwo.expandVolume(); err != nil {
		return err
	}
	if err := cwo.horizontalScale(); err != nil {
		return err
	}
	if err := cwo.reconfigure(); err != nil {
		return err
	}
	return nil
}

// copyAndMergeITS merges two ITS objects for updating:
//  1. new an object targetObj by copying from oldObj
//  2. merge all fields can be updated from newObj into targetObj
func copyAndMergeITS(oldITS, newITS *workloads.InstanceSet) *workloads.InstanceSet {
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
	// merge pod spec template annotations
	intctrlutil.MergeMetadataMapInplace(itsProto.Spec.Template.Annotations, &itsObjCopy.Spec.Template.Annotations)
	podTemplateCopy := *itsProto.Spec.Template.DeepCopy()
	podTemplateCopy.Annotations = itsObjCopy.Spec.Template.Annotations

	itsObjCopy.Spec.Template = podTemplateCopy
	itsObjCopy.Spec.Replicas = itsProto.Spec.Replicas
	itsObjCopy.Spec.Roles = itsProto.Spec.Roles
	itsObjCopy.Spec.MembershipReconfiguration = itsProto.Spec.MembershipReconfiguration
	itsObjCopy.Spec.TemplateVars = itsProto.Spec.TemplateVars
	itsObjCopy.Spec.Instances = itsProto.Spec.Instances
	itsObjCopy.Spec.OfflineInstances = itsProto.Spec.OfflineInstances
	itsObjCopy.Spec.MinReadySeconds = itsProto.Spec.MinReadySeconds
	itsObjCopy.Spec.VolumeClaimTemplates = itsProto.Spec.VolumeClaimTemplates
	itsObjCopy.Spec.PersistentVolumeClaimRetentionPolicy = itsProto.Spec.PersistentVolumeClaimRetentionPolicy
	itsObjCopy.Spec.ParallelPodManagementConcurrency = itsProto.Spec.ParallelPodManagementConcurrency
	itsObjCopy.Spec.PodUpdatePolicy = itsProto.Spec.PodUpdatePolicy
	itsObjCopy.Spec.InstanceUpdateStrategy = itsProto.Spec.InstanceUpdateStrategy
	itsObjCopy.Spec.MemberUpdateStrategy = itsProto.Spec.MemberUpdateStrategy
	itsObjCopy.Spec.Paused = itsProto.Spec.Paused
	itsObjCopy.Spec.Configs = itsProto.Spec.Configs
	itsObjCopy.Spec.Selector = itsProto.Spec.Selector

	if itsObjCopy.Spec.InstanceUpdateStrategy != nil && itsObjCopy.Spec.InstanceUpdateStrategy.RollingUpdate != nil {
		// use oldITS because itsObjCopy has been overwritten
		if oldITS.Spec.InstanceUpdateStrategy != nil &&
			oldITS.Spec.InstanceUpdateStrategy.RollingUpdate != nil &&
			oldITS.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable == nil {
			// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
			// MaxUnavailableStatefulSet feature.
			// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
			// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
			// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
			itsObjCopy.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable = nil
		}
	}

	intctrlutil.ResolvePodSpecDefaultFields(oldITS.Spec.Template.Spec, &itsObjCopy.Spec.Template.Spec)

	isSpecUpdated := !reflect.DeepEqual(&oldITS.Spec, &itsObjCopy.Spec)
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
