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
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
)

// componentWorkloadTransformer handles component workload generation
type componentWorkloadTransformer struct {
	client.Client
}

// componentWorkloadOps handles component workload ops
type componentWorkloadOps struct {
	cli            client.Client
	reqCtx         intctrlutil.RequestCtx
	cluster        *appsv1alpha1.Cluster
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG
	component      *appsv1alpha1.Component

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
	synthesizeComp := transCtx.SynthesizeComponent
	component := transCtx.Component
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

	if err = t.reconcileWorkload(synthesizeComp, transCtx.Component, runningITS, protoITS); err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if runningITS == nil {
		if protoITS != nil {
			graphCli.Create(dag, protoITS)
			return nil
		}
	} else {
		if protoITS == nil {
			graphCli.Delete(dag, runningITS)
		} else {
			err = t.handleUpdate(reqCtx, graphCli, dag, cluster, synthesizeComp, runningITS, protoITS, component)
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

func (t *componentWorkloadTransformer) reconcileWorkload(synthesizedComp *component.SynthesizedComponent,
	comp *appsv1alpha1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	if runningITS != nil {
		*protoITS.Spec.Selector = *runningITS.Spec.Selector
		protoITS.Spec.Template.Labels = intctrlutil.MergeMetadataMaps(runningITS.Spec.Template.Labels, synthesizedComp.UserDefinedLabels)
	}

	buildInstanceSetPlacementAnnotation(comp, protoITS)

	// build configuration template annotations to workload
	configuration.BuildConfigTemplateAnnotations(protoITS, synthesizedComp)

	// mark proto its as stopped if the component is stopped
	if isCompStopped(synthesizedComp) {
		t.stopWorkload(protoITS)
	}

	return nil
}

func isCompStopped(synthesizedComp *component.SynthesizedComponent) bool {
	return synthesizedComp.Stop != nil && *synthesizedComp.Stop
}

func (t *componentWorkloadTransformer) stopWorkload(protoITS *workloads.InstanceSet) {
	zero := func() *int32 { r := int32(0); return &r }()
	// since its doesn't support stop, we achieve it by setting replicas to 0.
	protoITS.Spec.Replicas = zero
	for i := range protoITS.Spec.Instances {
		protoITS.Spec.Instances[i].Replicas = zero
	}
}

func (t *componentWorkloadTransformer) handleUpdate(reqCtx intctrlutil.RequestCtx, cli model.GraphClient, dag *graph.DAG, cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent, runningITS, protoITS *workloads.InstanceSet, component *appsv1alpha1.Component) error {
	if !isCompStopped(synthesizeComp) {
		// postpone the update of the workload until the component is back to running.
		if err := t.handleWorkloadUpdate(reqCtx, dag, cluster, synthesizeComp, runningITS, protoITS, component); err != nil {
			return err
		}
	}

	objCopy := copyAndMergeITS(runningITS, protoITS, synthesizeComp)
	if objCopy != nil && !cli.IsAction(dag, objCopy, model.ActionNoopPtr()) {
		cli.Update(dag, nil, objCopy, &model.ReplaceIfExistingOption{})
	}

	// to work around that the scaled PVC will be deleted at object action.
	if err := updateVolumes(reqCtx, t.Client, synthesizeComp, runningITS, dag); err != nil {
		return err
	}
	return nil
}

func (t *componentWorkloadTransformer) handleWorkloadUpdate(reqCtx intctrlutil.RequestCtx, dag *graph.DAG, cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent, obj, its *workloads.InstanceSet, component *appsv1alpha1.Component) error {
	cwo, err := newComponentWorkloadOps(reqCtx, t.Client, cluster, synthesizeComp, obj, its, dag, component)
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

	// dag = cwo.dag

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
				// if persistence is not found, add emptyDir pod.spec.volumes[]
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
func copyAndMergeITS(oldITS, newITS *workloads.InstanceSet, synthesizeComp *component.SynthesizedComponent) *workloads.InstanceSet {
	// mergeAnnotations keeps the original annotations.
	mergeMetadataMap := func(originalMap map[string]string, targetMap *map[string]string) {
		if targetMap == nil || originalMap == nil {
			return
		}
		if *targetMap == nil {
			*targetMap = map[string]string{}
		}
		for k, v := range originalMap {
			// if the annotation not exist in targetAnnotations, copy it from original.
			if _, ok := (*targetMap)[k]; !ok {
				(*targetMap)[k] = v
			}
		}
	}

	// be compatible with existed cluster
	updateService := func(itsObj, itsProto *workloads.InstanceSet) *corev1.Service {
		if itsProto.Spec.Service != nil {
			return itsProto.Spec.Service
		}
		if itsObj.Spec.Service == nil {
			return nil
		}
		defaultServiceName := itsObj.Name
		for _, svc := range synthesizeComp.ComponentServices {
			if svc.PodService != nil && *svc.PodService || svc.DisableAutoProvision != nil && *svc.DisableAutoProvision {
				continue
			}
			serviceName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, svc.ServiceName)
			if defaultServiceName == serviceName {
				return itsObj.Spec.Service
			}
		}
		return nil
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
	mergeMetadataMap(itsObjCopy.Annotations, &itsProto.Annotations)
	itsObjCopy.Annotations = itsProto.Annotations

	// keep the original template annotations.
	// if annotations exist and are replaced, the its will be updated.
	mergeMetadataMap(itsObjCopy.Spec.Template.Annotations, &itsProto.Spec.Template.Annotations)
	itsObjCopy.Spec.Template = *itsProto.Spec.Template.DeepCopy()
	itsObjCopy.Spec.Replicas = itsProto.Spec.Replicas
	itsObjCopy.Spec.Service = updateService(itsObjCopy, itsProto)
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
	itsObjCopy.Spec.UpdateStrategy = itsProto.Spec.UpdateStrategy

	intctrlutil.ResolvePodSpecDefaultFields(oldITS.Spec.Template.Spec, &itsObjCopy.Spec.Template.Spec)
	DelayUpdateInstanceSetSystemFields(oldITS.Spec, &itsObjCopy.Spec)

	isSpecUpdated := !reflect.DeepEqual(&oldITS.Spec, &itsObjCopy.Spec)
	if isSpecUpdated {
		UpdateInstanceSetSystemFields(itsProto.Spec, &itsObjCopy.Spec)
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
	for _, vct := range r.runningITS.Spec.VolumeClaimTemplates {
		var proto *corev1.PersistentVolumeClaimTemplate
		for i, v := range r.synthesizeComp.VolumeClaimTemplates {
			if v.Name == vct.Name {
				proto = &r.synthesizeComp.VolumeClaimTemplates[i]
				break
			}
		}
		// REVIEW: seems we can remove a volume claim from templates at runtime, without any changes and warning messages?
		if proto == nil {
			continue
		}

		if err := r.expandVolumes(vct.Name, proto); err != nil {
			return err
		}
	}
	return nil
}

// horizontalScale handles workload horizontal scale
func (r *componentWorkloadOps) horizontalScale() error {
	its := r.runningITS
	// handle memberjoin lifecycle action
	if err := r.checkAndDoMemberJoin(); err != nil {
		return err
	}
	doScaleOut, doScaleIn := r.horizontalScaling()
	if !doScaleOut && !doScaleIn {
		if err := r.postScaleIn(); err != nil {
			return err
		}
		if err := r.postScaleOut(its); err != nil {
			return err
		}
		return nil
	}
	if doScaleIn {
		if err := r.scaleIn(its); err != nil {
			return err
		}
	}
	if doScaleOut {
		if err := r.scaleOut(its); err != nil {
			return err
		}
	}

	r.reqCtx.Recorder.Eventf(r.cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		r.synthesizeComp.Name, r.cluster.Name, int(*its.Spec.Replicas), r.synthesizeComp.Replicas)

	return nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (r *componentWorkloadOps) horizontalScaling() (bool, bool) {
	var (
		doScaleOut bool
		doScaleIn  bool
	)
	for _, podName := range r.desiredCompPodNames {
		if _, ok := r.runningItsPodNameSet[podName]; !ok {
			doScaleOut = true
			break
		}
	}
	for _, podName := range r.runningItsPodNames {
		if _, ok := r.desiredCompPodNameSet[podName]; !ok {
			doScaleIn = true
			break
		}
	}
	return doScaleOut, doScaleIn
}

func (r *componentWorkloadOps) postScaleIn() error {
	return nil
}

func (r *componentWorkloadOps) postScaleOut(itsObj *workloads.InstanceSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: itsObj.Namespace,
			Name:      constant.GenerateResourceNameWithScalingSuffix(itsObj.Name),
		}
	)

	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, itsObj, itsObj, snapshotKey)
	if err != nil {
		return err
	}
	if d != nil {
		// clean backup resources.
		// there will not be any backup resources other than scale out.
		tmpObjs, err := d.ClearTmpResources()
		if err != nil {
			return err
		}
		graphCli := model.NewGraphClient(r.cli)
		for _, obj := range tmpObjs {
			graphCli.Do(r.dag, nil, obj, model.ActionDeletePtr(), nil)
		}
	}

	return nil
}

func (r *componentWorkloadOps) scaleIn(itsObj *workloads.InstanceSet) error {
	// if scale in to 0, do not delete pvcs
	if r.synthesizeComp.Replicas == 0 {
		r.reqCtx.Log.Info("scale in to 0, keep all PVCs")
		return nil
	}
	// TODO: check the component definition to determine whether we need to call leave member before deleting replicas.
	err := r.leaveMember4ScaleIn()
	if err != nil {
		r.reqCtx.Log.Info(fmt.Sprintf("leave member at scaling-in error, retry later: %s", err.Error()))
		return err
	}
	return r.deletePVCs4ScaleIn(itsObj)
}

func (r *componentWorkloadOps) scaleOut(itsObj *workloads.InstanceSet) error {
	var (
		backupKey = types.NamespacedName{
			Namespace: itsObj.Namespace,
			Name:      constant.GenerateResourceNameWithScalingSuffix(itsObj.Name),
		}
	)

	// its's replicas=0 means it's starting not scaling, skip all the scaling work.
	if *itsObj.Spec.Replicas == 0 {
		return nil
	}

	r.annotateInstanceSetForMemberJoin()

	graphCli := model.NewGraphClient(r.cli)
	graphCli.Noop(r.dag, r.protoITS)
	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, itsObj, r.protoITS, backupKey)
	if err != nil {
		return err
	}
	var succeed bool
	if d == nil {
		succeed = true
	} else {
		succeed, err = d.Succeed()
		if err != nil {
			return err
		}
	}
	if succeed {
		// pvcs are ready, ITS.replicas should be updated
		graphCli.Update(r.dag, nil, r.protoITS)
		return r.postScaleOut(itsObj)
	} else {
		graphCli.Noop(r.dag, r.protoITS)
		// update objs will trigger reconcile, no need to requeue error
		objs1, objs2, err := d.CloneData(d)
		if err != nil {
			return err
		}
		for _, obj := range objs1 {
			graphCli.Do(r.dag, nil, obj, model.ActionCreatePtr(), nil)
		}
		for _, obj := range objs2 {
			graphCli.Do(r.dag, nil, obj, model.ActionCreatePtr(), nil, inDataContext4G())
		}
		return nil
	}
}

func getHealthyLorryClient(pods []*corev1.Pod) (lorry.Client, error) {
	for _, pod := range pods {
		lorryCli, err := lorry.NewClient(*pod)
		if err == nil {
			return lorryCli, nil
		}
	}
	return nil, fmt.Errorf("no health lorry client found")
}

func (r *componentWorkloadOps) annotateInstanceSetForMemberJoin() {
	if r.synthesizeComp.LifecycleActions.MemberJoin == nil {
		return
	}

	podsToMemberjoin := getPodsToMemberJoinFromAnno(r.runningITS)

	for podName := range r.desiredCompPodNameSet {
		if r.runningItsPodNameSet.Has(podName) {
			continue
		}
		if podsToMemberjoin.Has(podName) {
			continue
		}
		podsToMemberjoin.Insert(podName)
	}

	if podsToMemberjoin.Len() > 0 {
		r.protoITS.Annotations[constant.MemberJoinStatusAnnotationKey] = strings.Join(sets.List(podsToMemberjoin), ",")
	}
}

func getPodsToMemberJoinFromAnno(instanceSet *workloads.InstanceSet) sets.Set[string] {
	podsToMemberjoin := sets.New[string]()
	if instanceSet == nil {
		return podsToMemberjoin
	}

	if instanceSet.Annotations == nil {
		return podsToMemberjoin
	}

	if memberJoinStatus := instanceSet.Annotations[constant.MemberJoinStatusAnnotationKey]; memberJoinStatus != "" {
		podsToMemberjoin.Insert(strings.Split(memberJoinStatus, ",")...)
	}

	return podsToMemberjoin
}

func (r *componentWorkloadOps) leaveMember4ScaleIn() error {
	labels := constant.GetComponentWellKnownLabels(r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
	pods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.synthesizeComp.Namespace, labels, inDataContext4C())
	if err != nil {
		return err
	}

	// TODO: Move memberLeave to the ITS controller. Instead of performing a switchover, we can directly scale down the non-leader nodes. This is because the pod ordinal is not guaranteed to be continuous.
	podsToMemberLeave := make([]*corev1.Pod, 0)

	podsToMemberjoin := getPodsToMemberJoinFromAnno(r.runningITS)
	for _, pod := range pods {
		// if the pod not exists in the generated pod names, it should be a member that needs to leave
		if _, ok := r.desiredCompPodNameSet[pod.Name]; ok {
			continue
		}
		podsToMemberLeave = append(podsToMemberLeave, pod)
	}

	var leaveErrors []error
	for _, pod := range podsToMemberLeave {
		if podsToMemberjoin.Has(pod.Name) {
			leaveErrors = append(leaveErrors, fmt.Errorf("pod %s is in memberjoin process", pod.Name))
			continue
		}
		if err := r.leaveMemberForPod(pod, pods); err != nil {
			leaveErrors = append(leaveErrors, err)
		}
	}
	if len(leaveErrors) > 0 {
		return newRequeueError(time.Second, fmt.Sprintf("%v", leaveErrors))
	}
	return nil
}

func (r *componentWorkloadOps) leaveMemberForPod(pod *corev1.Pod, pods []*corev1.Pod) error {
	tryToSwitchover := func(lorryCli lorry.Client, pod *corev1.Pod) error {
		if pod == nil || len(pod.Labels) == 0 {
			return nil
		}
		// if pod is not leader/primary, no need to switchover
		isLeader := func() bool {
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
		if !isLeader() {
			return nil
		}
		// if HA functionality is not enabled, no need to switchover
		err := lorryCli.Switchover(r.reqCtx.Ctx, pod.Name, "", false)
		if err == lorry.NotImplemented {
			// For the purpose of upgrade compatibility, if the version of Lorry is 0.7 and
			// the version of KB is upgraded to 0.8 or newer, lorry client will return an NotImplemented error,
			// in this case, here just return success.
			r.reqCtx.Log.Info("lorry switchover api is not implemented")
			return nil
		}
		if err == nil {
			return fmt.Errorf("switchover succeed, wait role label to be updated")
		}
		if strings.Contains(err.Error(), "cluster's ha is disabled") {
			return nil
		}
		return err
	}

	// try the pod to leave first
	lorryCli, err := lorry.NewClient(*pod)
	if err != nil {
		// try another pod
		lorryCli, err = getHealthyLorryClient(pods)
		if err != nil {
			return err
		}
	}

	if intctrlutil.IsNil(lorryCli) {
		// no lorry in the pod
		return nil
	}

	// switchover if the leaving pod is leader
	if switchoverErr := tryToSwitchover(lorryCli, pod); switchoverErr != nil {
		return switchoverErr
	}

	if err = lorryCli.LeaveMember(r.reqCtx.Ctx, pod.Name); err != nil {
		if err != lorry.NotImplemented {
			return err
		}
	}
	return nil
}

func (r *componentWorkloadOps) checkAndDoMemberJoin() error {
	// just wait for memberjoin anno to be updated
	if r.protoITS.Annotations[constant.MemberJoinStatusAnnotationKey] != "" {
		return nil
	}

	podsToMemberjoin := getPodsToMemberJoinFromAnno(r.runningITS)
	if len(podsToMemberjoin) == 0 {
		return nil
	}

	if r.synthesizeComp.LifecycleActions == nil || r.synthesizeComp.LifecycleActions.MemberJoin == nil {
		podsToMemberjoin.Clear()
	}
	err := r.doMemberJoin(podsToMemberjoin)
	if err != nil {
		return err
	}

	if podsToMemberjoin.Len() == 0 {
		// Anno will be merged later, so it should be deleted from both protoITS and runningITS
		delete(r.protoITS.Annotations, constant.MemberJoinStatusAnnotationKey)
		delete(r.runningITS.Annotations, constant.MemberJoinStatusAnnotationKey)
	} else {
		r.protoITS.Annotations[constant.MemberJoinStatusAnnotationKey] = strings.Join(sets.List(podsToMemberjoin), ",")
	}
	return nil
}

func (r *componentWorkloadOps) precondition(name string, action *appsv1alpha1.Action) error {
	if action == nil || action.PreCondition == nil {
		return nil
	}

	switch *action.PreCondition {
	case appsv1alpha1.ImmediatelyPreConditionType:
		return nil
	case appsv1alpha1.ComponentReadyPreConditionType:
		if r.component == nil || r.component.Status.Phase != appsv1alpha1.RunningClusterCompPhase {
			return fmt.Errorf("component is nil when checking RuntimeReady preCondition in %s action", name)
		}
	case appsv1alpha1.RuntimeReadyPreConditionType:
		if r.runningITS == nil || !instanceset.IsInstancesReady(r.runningITS) {
			return fmt.Errorf("runtime is nil when checking RuntimeReady preCondition in %s action", name)
		}
	case appsv1alpha1.ClusterReadyPreConditionType:
		if r.cluster == nil || r.cluster.Status.Phase != appsv1alpha1.RunningClusterPhase {
			return fmt.Errorf("cluster is nil when checking RuntimeReady preCondition in %s action", name)
		}
	default:
		return fmt.Errorf("unknown precondition type %s", *action.PreCondition)
	}

	return nil
}

func (r *componentWorkloadOps) doMemberJoin(podSet sets.Set[string]) error {
	if len(podSet) == 0 {
		return nil
	}

	if r.synthesizeComp.LifecycleActions == nil || r.synthesizeComp.LifecycleActions.MemberJoin == nil {
		return nil
	}

	if err := r.precondition(constant.MemberJoinAction, r.synthesizeComp.LifecycleActions.MemberJoin.CustomHandler); err != nil {
		return err
	}

	labels := constant.GetComponentWellKnownLabels(r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
	runningPods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.synthesizeComp.Namespace, labels, inDataContext4C())
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	var joinErrors []error
	for _, pod := range runningPods {
		if !podSet.Has(pod.Name) {
			continue
		}

		if err := r.joinMemberForPod(pod, podSet); err != nil {
			joinErrors = append(joinErrors, fmt.Errorf("pod %s: %w", pod.Name, err))
		} else {
			podSet.Delete(pod.Name)
		}
	}

	if len(joinErrors) > 0 {
		return newRequeueError(time.Second, fmt.Sprintf("%v", joinErrors))
	}
	return nil
}

func (r *componentWorkloadOps) joinMemberForPod(pod *corev1.Pod, podSet sets.Set[string]) error {
	lorryCli, err := lorry.NewClient(*pod)
	if err != nil {
		return fmt.Errorf("failed to create lorry client: %w", err)
	}

	if intctrlutil.IsNil(lorryCli) {
		r.reqCtx.Log.Info("skipping pod with nil lorry client", "pod", pod.Name)
		return nil
	}

	if err = lorryCli.JoinMember(r.reqCtx.Ctx); err != nil {
		if err == lorry.NotImplemented {
			r.reqCtx.Log.Info("lorry join member API not implemented", "pod", pod.Name)
			return nil
		}
		return fmt.Errorf("join member failed: %w", err)
	}

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

func (r *componentWorkloadOps) expandVolumes(vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
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

func updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, synthesizeComp *component.SynthesizedComponent,
	itsObj *workloads.InstanceSet, dag *graph.DAG) error {
	graphCli := model.NewGraphClient(cli)

	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, obj := range graphCli.FindAll(dag, &corev1.PersistentVolumeClaim{}) {
		pvcNameSet.Insert(obj.GetName())
	}

	for _, vct := range synthesizeComp.VolumeClaimTemplates {
		pvcs, err := getRunningVolumes(reqCtx.Ctx, cli, synthesizeComp, itsObj, vct.Name)
		if err != nil {
			return err
		}
		for _, pvc := range pvcs {
			if pvcNameSet.Has(pvc.Name) {
				continue
			}
			graphCli.Noop(dag, pvc)
		}
	}
	return nil
}

// getRunningVolumes gets the running volumes of the ITS.
func getRunningVolumes(ctx context.Context, cli client.Client, synthesizedComp *component.SynthesizedComponent,
	itsObj *workloads.InstanceSet, vctName string) ([]*corev1.PersistentVolumeClaim, error) {
	labels := constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name)
	pvcs, err := component.ListObjWithLabelsInNamespace(ctx, cli, generics.PersistentVolumeClaimSignature, synthesizedComp.Namespace, labels, inDataContext4C())
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

func buildInstanceSetPlacementAnnotation(comp *appsv1alpha1.Component, its *workloads.InstanceSet) {
	p := placement(comp)
	if len(p) > 0 {
		if its.Annotations == nil {
			its.Annotations = make(map[string]string)
		}
		its.Annotations[constant.KBAppMultiClusterPlacementKey] = p
	}
}

func newComponentWorkloadOps(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent, runningITS *workloads.InstanceSet, protoITS *workloads.InstanceSet, dag *graph.DAG, component *appsv1alpha1.Component) (*componentWorkloadOps, error) {
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
		synthesizeComp:        synthesizeComp,
		runningITS:            runningITS,
		protoITS:              protoITS,
		dag:                   dag,
		desiredCompPodNames:   compPodNames,
		runningItsPodNames:    itsPodNames,
		desiredCompPodNameSet: sets.New(compPodNames...),
		runningItsPodNameSet:  sets.New(itsPodNames...),
		component:             component,
	}, nil
}
