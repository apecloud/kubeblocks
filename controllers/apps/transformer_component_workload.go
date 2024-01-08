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
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	lorry "github.com/apecloud/kubeblocks/pkg/lorry/client"
)

// componentWorkloadTransformer handles component rsm workload generation
type componentWorkloadTransformer struct {
	client.Client
}

// componentWorkloadOps handles component rsm workload ops
type componentWorkloadOps struct {
	cli            client.Client
	reqCtx         intctrlutil.RequestCtx
	cluster        *appsv1alpha1.Cluster
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	// runningRSM is a snapshot of the rsm that is already running
	runningRSM *workloads.ReplicatedStateMachine
	// protoRSM is the rsm object that is rebuilt from scratch during each reconcile process
	protoRSM *workloads.ReplicatedStateMachine
}

var _ graph.Transformer = &componentWorkloadTransformer{}

func (t *componentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	runningRSM, err := t.runningRSMObject(ctx, synthesizeComp)
	if err != nil {
		return err
	}
	transCtx.RunningWorkload = runningRSM

	// build synthesizeComp podSpec volumeMounts
	buildPodSpecVolumeMounts(synthesizeComp)

	// build rsm workload
	if synthesizeComp.RsmTransformPolicy == workloads.ToPod {
		err = BuildNodesAssignment(transCtx.Context, t.Client, synthesizeComp, runningRSM, cluster)
		if err != nil {
			return err
		}
	}

	protoRSM, err := factory.BuildRSM(cluster, synthesizeComp)
	if err != nil {
		return err
	}
	if runningRSM != nil {
		*protoRSM.Spec.Selector = *runningRSM.Spec.Selector
		protoRSM.Spec.Template.Labels = runningRSM.Spec.Template.Labels
	}
	transCtx.ProtoWorkload = protoRSM

	// build configuration template annotations to rsm workload
	buildRSMConfigTplAnnotations(protoRSM, synthesizeComp)

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if runningRSM == nil {
		if protoRSM != nil {
			graphCli.Create(dag, protoRSM)
			return nil
		}
	} else {
		if protoRSM == nil {
			graphCli.Delete(dag, runningRSM)
		} else {
			err = t.handleUpdate(reqCtx, graphCli, dag, cluster, synthesizeComp, runningRSM, protoRSM)
		}
	}
	return err
}

func (t *componentWorkloadTransformer) runningRSMObject(ctx graph.TransformContext,
	synthesizeComp *component.SynthesizedComponent) (*workloads.ReplicatedStateMachine, error) {
	rsmKey := types.NamespacedName{
		Namespace: synthesizeComp.Namespace,
		Name:      constant.GenerateRSMNamePattern(synthesizeComp.ClusterName, synthesizeComp.Name),
	}
	rsm := &workloads.ReplicatedStateMachine{}
	if err := ctx.GetClient().Get(ctx.GetContext(), rsmKey, rsm); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return rsm, nil
}

func (t *componentWorkloadTransformer) handleUpdate(reqCtx intctrlutil.RequestCtx, cli model.GraphClient, dag *graph.DAG,
	cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent, runningRSM, protoRSM *workloads.ReplicatedStateMachine) error {
	// TODO(xingran): Some RSM workload operations should be moved down to Lorry implementation. Subsequent operations such as horizontal scaling will be removed from the component controller
	if err := t.handleWorkloadUpdate(reqCtx, dag, cluster, synthesizeComp, runningRSM, protoRSM); err != nil {
		return err
	}

	objCopy := copyAndMergeRSM(runningRSM, protoRSM, synthesizeComp)
	if objCopy != nil && !cli.IsAction(dag, objCopy, model.ActionNoopPtr()) {
		cli.Update(dag, nil, objCopy, &model.ReplaceIfExistingOption{})
	}

	// to work around that the scaled PVC will be deleted at object action.
	if err := updateVolumes(reqCtx, t.Client, synthesizeComp, runningRSM, dag); err != nil {
		return err
	}
	return nil
}

func (t *componentWorkloadTransformer) handleWorkloadUpdate(reqCtx intctrlutil.RequestCtx, dag *graph.DAG,
	cluster *appsv1alpha1.Cluster, synthesizeComp *component.SynthesizedComponent, obj, rsm *workloads.ReplicatedStateMachine) error {
	cwo := newComponentWorkloadOps(reqCtx, t.Client, cluster, synthesizeComp, obj, rsm, dag)

	// handle rsm expand volume
	if err := cwo.expandVolume(); err != nil {
		return err
	}

	// handle rsm workload horizontal scale
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

// copyAndMergeRSM merges two RSM objects for updating:
//  1. new an object targetObj by copying from oldObj
//  2. merge all fields can be updated from newObj into targetObj
func copyAndMergeRSM(oldRsm, newRsm *workloads.ReplicatedStateMachine, synthesizeComp *component.SynthesizedComponent) *workloads.ReplicatedStateMachine {
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

	// buildWorkLoadAnnotations builds the annotations for Deployment/StatefulSet
	buildWorkLoadAnnotations := func(obj client.Object) {
		workloadAnnotations := obj.GetAnnotations()
		if workloadAnnotations == nil {
			workloadAnnotations = map[string]string{}
		}
		// record the cluster generation to check if the sts is latest
		workloadAnnotations[constant.KubeBlocksGenerationKey] = synthesizeComp.ClusterGeneration
		obj.SetAnnotations(workloadAnnotations)
	}

	updateUpdateStrategy := func(rsmObj, rsmProto *workloads.ReplicatedStateMachine) {
		var objMaxUnavailable *intstr.IntOrString
		if rsmObj.Spec.UpdateStrategy.RollingUpdate != nil {
			objMaxUnavailable = rsmObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
		}
		rsmObj.Spec.UpdateStrategy = rsmProto.Spec.UpdateStrategy
		if objMaxUnavailable == nil && rsmObj.Spec.UpdateStrategy.RollingUpdate != nil {
			// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
			// MaxUnavailableStatefulSet feature.
			// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
			// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
			// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
			rsmObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = nil
		}
	}

	// be compatible with existed cluster
	updateService := func(rsmObj, rsmProto *workloads.ReplicatedStateMachine) *corev1.Service {
		if rsmProto.Spec.Service != nil {
			return rsmProto.Spec.Service
		}
		if rsmObj.Spec.Service == nil {
			return nil
		}
		defaultServiceName := rsmObj.Name
		for _, svc := range synthesizeComp.ComponentServices {
			if svc.GeneratePodOrdinalService {
				continue
			}
			serviceName := constant.GenerateComponentServiceName(synthesizeComp.ClusterName, synthesizeComp.Name, svc.ServiceName)
			if defaultServiceName == serviceName {
				return rsmObj.Spec.Service
			}
		}
		return nil
	}

	rsmObjCopy := oldRsm.DeepCopy()
	rsmProto := newRsm

	// remove original monitor annotations
	if len(rsmObjCopy.Annotations) > 0 {
		maps.DeleteFunc(rsmObjCopy.Annotations, func(k, v string) bool {
			return strings.HasPrefix(k, "monitor.kubeblocks.io")
		})
	}
	mergeMetadataMap(rsmObjCopy.Annotations, &rsmProto.Annotations)
	rsmObjCopy.Annotations = rsmProto.Annotations
	buildWorkLoadAnnotations(rsmObjCopy)

	// keep the original template annotations.
	// if annotations exist and are replaced, the rsm will be updated.
	mergeMetadataMap(rsmObjCopy.Spec.Template.Annotations, &rsmProto.Spec.Template.Annotations)
	rsmObjCopy.Spec.Template = rsmProto.Spec.Template
	rsmObjCopy.Spec.Replicas = rsmProto.Spec.Replicas
	rsmObjCopy.Spec.Service = updateService(rsmObjCopy, rsmProto)
	rsmObjCopy.Spec.AlternativeServices = rsmProto.Spec.AlternativeServices
	rsmObjCopy.Spec.Roles = rsmProto.Spec.Roles
	rsmObjCopy.Spec.RoleProbe = rsmProto.Spec.RoleProbe
	rsmObjCopy.Spec.MembershipReconfiguration = rsmProto.Spec.MembershipReconfiguration
	rsmObjCopy.Spec.MemberUpdateStrategy = rsmProto.Spec.MemberUpdateStrategy
	rsmObjCopy.Spec.Credential = rsmProto.Spec.Credential
	rsmObjCopy.Spec.NodeAssignment = rsmProto.Spec.NodeAssignment

	if rsmProto.Spec.UpdateStrategy.Type != "" || rsmProto.Spec.UpdateStrategy.RollingUpdate != nil {
		updateUpdateStrategy(rsmObjCopy, rsmProto)
	}

	ResolvePodSpecDefaultFields(oldRsm.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)
	DelayUpdatePodSpecSystemFields(oldRsm.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)

	isSpecUpdated := !reflect.DeepEqual(&oldRsm.Spec, &rsmObjCopy.Spec)
	if isSpecUpdated {
		UpdatePodSpecSystemFields(&rsmProto.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)
	}

	isLabelsUpdated := !reflect.DeepEqual(oldRsm.Labels, rsmObjCopy.Labels)
	isAnnotationsUpdated := !reflect.DeepEqual(oldRsm.Annotations, rsmObjCopy.Annotations)
	if !isSpecUpdated && !isLabelsUpdated && !isAnnotationsUpdated {
		return nil
	}
	return rsmObjCopy
}

// expandVolume handles rsm workload expand volume
func (r *componentWorkloadOps) expandVolume() error {
	for _, vct := range r.runningRSM.Spec.VolumeClaimTemplates {
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

// horizontalScale handles rsm workload horizontal scale
func (r *componentWorkloadOps) horizontalScale() error {
	sts := rsmcore.ConvertRSMToSTS(r.runningRSM)
	if sts.Status.ReadyReplicas == r.synthesizeComp.Replicas {
		return nil
	}
	ret := r.horizontalScaling(r.synthesizeComp, sts)
	if ret == 0 {
		if err := r.postScaleIn(); err != nil {
			return err
		}
		if err := r.postScaleOut(sts); err != nil {
			return err
		}
		return nil
	}
	if ret < 0 {
		if err := r.scaleIn(sts); err != nil {
			return err
		}
	} else {
		if err := r.scaleOut(sts); err != nil {
			return err
		}
	}

	if r.synthesizeComp.RsmTransformPolicy != workloads.ToPod {
		if err := r.updatePodReplicaLabel4Scaling(r.synthesizeComp.Replicas); err != nil {
			return err
		}
	}

	r.reqCtx.Recorder.Eventf(r.cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		r.synthesizeComp.Name, r.cluster.Name, int(r.synthesizeComp.Replicas)-ret, r.synthesizeComp.Replicas)

	return nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (r *componentWorkloadOps) horizontalScaling(synthesizeComp *component.SynthesizedComponent, stsObj *apps.StatefulSet) int {
	return int(synthesizeComp.Replicas - *stsObj.Spec.Replicas)
}

func (r *componentWorkloadOps) postScaleIn() error {
	return nil
}

func (r *componentWorkloadOps) postScaleOut(stsObj *apps.StatefulSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, stsObj, stsObj, snapshotKey)
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

func (r *componentWorkloadOps) scaleIn(stsObj *apps.StatefulSet) error {
	// if scale in to 0, do not delete pvcs
	if r.synthesizeComp.Replicas == 0 {
		r.reqCtx.Log.Info("scale in to 0, keep all PVCs")
		return nil
	}
	// TODO: check the component definition to determine whether we need to call leave member before deleting replicas.
	err := r.leaveMember4ScaleIn(stsObj)
	if err != nil {
		r.reqCtx.Log.Info(fmt.Sprintf("leave member at scaling-in error, retry later: %s", err.Error()))
		return err
	}
	return r.deletePVCs4ScaleIn(stsObj)
}

func (r *componentWorkloadOps) scaleOut(stsObj *apps.StatefulSet) error {
	var (
		backupKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	// sts's replicas=0 means it's starting not scaling, skip all the scaling work.
	if *stsObj.Spec.Replicas == 0 {
		return nil
	}
	graphCli := model.NewGraphClient(r.cli)
	graphCli.Noop(r.dag, r.protoRSM)
	stsProto := rsmcore.ConvertRSMToSTS(r.protoRSM)
	d, err := newDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, stsObj, stsProto, backupKey)
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
		// pvcs are ready, rsm.replicas should be updated
		graphCli.Update(r.dag, nil, r.protoRSM)
		return r.postScaleOut(stsObj)
	} else {
		graphCli.Noop(r.dag, r.protoRSM)
		// update objs will trigger reconcile, no need to requeue error
		objs, err := d.CloneData(d)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			graphCli.Do(r.dag, nil, obj, model.ActionCreatePtr(), nil)
		}
		return nil
	}
}

func (r *componentWorkloadOps) updatePodReplicaLabel4Scaling(replicas int32) error {
	graphCli := model.NewGraphClient(r.cli)
	pods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
	for _, pod := range pods {
		obj := pod.DeepCopy()
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(replicas))
		graphCli.Update(r.dag, nil, obj)
	}
	return nil
}

func (r *componentWorkloadOps) leaveMember4ScaleIn(stsObj *apps.StatefulSet) error {
	pods, err := component.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
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

			for _, replicaRole := range r.runningRSM.Spec.Roles {
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
	deletePodList, err := calculateDeletePods(pods, r.synthesizeComp.RsmTransformPolicy, *stsObj.Spec.Replicas-r.synthesizeComp.Replicas,
		r.synthesizeComp.Replicas, r.synthesizeComp.Instances)
	if err != nil {
		return err
	}
	for _, pod := range deletePodList {
		lorryCli, err1 := lorry.NewClient(*pod)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}

		if intctrlutil.IsNil(lorryCli) {
			// no lorry in the pod
			continue
		}

		// switchover if the leaving pod is leader
		if switchoverErr := tryToSwitchover(lorryCli, pod); switchoverErr != nil {
			return switchoverErr
		}

		if err2 := lorryCli.LeaveMember(r.reqCtx.Ctx); err2 != nil {
			// For the purpose of upgrade compatibility, if the version of Lorry is 0.7 and
			// the version of KB is upgraded to 0.8 or newer, lorry client will return an NotImplemented error,
			// in this case, here just ignore it.
			if err2 == lorry.NotImplemented {
				r.reqCtx.Log.Info("lorry leave member api is not implemented")
			} else if err == nil {
				err = err2
			}
		}
	}
	return err // TODO: use requeue-after
}

func (r *componentWorkloadOps) deletePVCs4ScaleIn(stsObj *apps.StatefulSet) error {
	graphCli := model.NewGraphClient(r.cli)
	for i := r.synthesizeComp.Replicas; i < *stsObj.Spec.Replicas; i++ {
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			pvc := corev1.PersistentVolumeClaim{}
			if err := r.cli.Get(r.reqCtx.Ctx, pvcKey, &pvc); err != nil {
				return err
			}
			// Since there are no order guarantee between updating STS and deleting PVCs, if there is any error occurred
			// after updating STS and before deleting PVCs, the PVCs intended to scale-in will be leaked.
			// For simplicity, the updating dependency is added between them to guarantee that the PVCs to scale-in
			// will be deleted or the scaling-in operation will be failed.
			graphCli.Delete(r.dag, &pvc)
		}
	}
	return nil
}

func (r *componentWorkloadOps) expandVolumes(vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	for i := *r.runningRSM.Spec.Replicas - 1; i >= 0; i-- {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: r.cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s-%d", vctName, r.runningRSM.Name, i),
		}
		pvcNotFound := false
		if err := r.cli.Get(r.reqCtx.Ctx, pvcKey, pvc); err != nil {
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
		if err := r.cli.List(r.reqCtx.Ctx, &pvList, ml); err != nil {
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
	pvKey := types.NamespacedName{
		Namespace: pvcKey.Namespace,
		Name:      newPVC.Spec.VolumeName,
	}
	if err := r.cli.Get(r.reqCtx.Ctx, pvKey, pv); err != nil {
		if apierrors.IsNotFound(err) {
			pvNotFound = true
		} else {
			return err
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
			return graphCli.Do(r.dag, pv, retainPV, model.ActionPatchPtr(), fromVertex)
		},
		deletePVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := graphCli.Do(r.dag, pvc, removeFinalizerPVC, model.ActionPatchPtr(), fromVertex)
			return graphCli.Do(r.dag, nil, removeFinalizerPVC, model.ActionDeletePtr(), removeFinalizerPVCVertex)
		},
		removePVClaimRefStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return graphCli.Do(r.dag, pv, removeClaimRefPV, model.ActionPatchPtr(), fromVertex)
		},
		createPVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return graphCli.Do(r.dag, nil, newPVC, model.ActionCreatePtr(), fromVertex)
		},
		pvRestorePolicyStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return graphCli.Do(r.dag, pv, restorePV, model.ActionPatchPtr(), fromVertex)
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := r.buildProtoRSMWorkloadVertex()
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
		graphCli.Update(r.dag, nil, newPVC)
		return nil
	}
	// all the else means no need to update

	return nil
}

// buildProtoRSMWorkloadVertex builds protoRSM workload vertex
func (r *componentWorkloadOps) buildProtoRSMWorkloadVertex() *model.ObjectVertex {
	for _, vertex := range r.dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if v.Obj == r.protoRSM {
			return v
		}
	}
	return nil
}

func updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, synthesizeComp *component.SynthesizedComponent,
	rsmObj *workloads.ReplicatedStateMachine, dag *graph.DAG) error {
	graphCli := model.NewGraphClient(cli)
	getRunningVolumes := func(vctName string) ([]*corev1.PersistentVolumeClaim, error) {
		pvcs, err := component.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature,
			rsmObj.Namespace, constant.GetComponentWellKnownLabels(synthesizeComp.ClusterName, synthesizeComp.Name))
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

	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, obj := range graphCli.FindAll(dag, &corev1.PersistentVolumeClaim{}) {
		pvcNameSet.Insert(obj.GetName())
	}

	for _, vct := range synthesizeComp.VolumeClaimTemplates {
		pvcs, err := getRunningVolumes(vct.Name)
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

// buildRSMConfigTplAnnotations builds config tpl annotations for rsm
func buildRSMConfigTplAnnotations(rsm *workloads.ReplicatedStateMachine, synthesizedComp *component.SynthesizedComponent) {
	configTplAnnotations := make(map[string]string)
	for _, configTplSpec := range synthesizedComp.ConfigTemplates {
		configTplAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(configTplSpec.Name)] = core.GetComponentCfgName(synthesizedComp.ClusterName, synthesizedComp.Name, configTplSpec.Name)
	}
	for _, scriptTplSpec := range synthesizedComp.ScriptTemplates {
		configTplAnnotations[core.GenerateTPLUniqLabelKeyWithConfig(scriptTplSpec.Name)] = core.GetComponentCfgName(synthesizedComp.ClusterName, synthesizedComp.Name, scriptTplSpec.Name)
	}
	updateRSMAnnotationsWithTemplate(rsm, configTplAnnotations)
}

func updateRSMAnnotationsWithTemplate(rsm *workloads.ReplicatedStateMachine, allTemplateAnnotations map[string]string) {
	// full configmap upgrade
	existLabels := make(map[string]string)
	annotations := rsm.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for key, val := range annotations {
		if strings.HasPrefix(key, constant.ConfigurationTplLabelPrefixKey) {
			existLabels[key] = val
		}
	}

	// delete not exist configmap label
	deletedLabels := cfgutil.MapKeyDifference(existLabels, allTemplateAnnotations)
	for l := range deletedLabels.Iter() {
		delete(annotations, l)
	}

	for key, val := range allTemplateAnnotations {
		annotations[key] = val
	}
	rsm.SetAnnotations(annotations)
}

func newComponentWorkloadOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *component.SynthesizedComponent,
	runningRSM *workloads.ReplicatedStateMachine,
	protoRSM *workloads.ReplicatedStateMachine,
	dag *graph.DAG) *componentWorkloadOps {
	return &componentWorkloadOps{
		cli:            cli,
		reqCtx:         reqCtx,
		cluster:        cluster,
		synthesizeComp: synthesizeComp,
		runningRSM:     runningRSM,
		protoRSM:       protoRSM,
		dag:            dag,
	}
}

func BuildNodesAssignment(ctx context.Context, cli client.Client, synthesizeComp *component.SynthesizedComponent, rsm *workloads.ReplicatedStateMachine, cluster *appsv1alpha1.Cluster) error {
	currentNodesAssignment := make([]workloads.NodeAssignment, 0)
	if rsm != nil {
		currentNodesAssignment = rsm.Spec.NodeAssignment
	}
	instances := synthesizeComp.Instances
	nodes := synthesizeComp.Nodes
	expectedReplicas := synthesizeComp.Replicas

	currentReplicas := int32(len(currentNodesAssignment))
	if currentReplicas > expectedReplicas {
		var err error
		pods, err := component.ListPodOwnedByComponent(ctx, cli, cluster.Namespace, constant.GetComponentWellKnownLabels(cluster.Name, synthesizeComp.Name))
		if err != nil {
			return err
		}
		currentNodesAssignment, err = DeletePodFromInstances(pods, instances, currentReplicas-expectedReplicas, currentNodesAssignment)
		if err != nil {
			return err
		}
	} else if currentReplicas < expectedReplicas {
		res := AllocateNodesForPod(nodes, expectedReplicas-currentReplicas, synthesizeComp.ClusterName, synthesizeComp.Name)
		currentNodesAssignment = append(currentNodesAssignment, res...)
	}
	synthesizeComp.NodesAssignment = currentNodesAssignment
	return nil
}

func calculateDeletePods(pods []*corev1.Pod, policy workloads.RsmTransformPolicy, deltaReplicas, expectReplicas int32, instances []string) ([]*corev1.Pod, error) {
	if deltaReplicas < 0 {
		return nil, fmt.Errorf("unexpect deltaReplicas: %d", deltaReplicas)
	}
	deletePodList := make([]*corev1.Pod, 0)
	deletePodNames := make(map[string]struct{})
	if policy == workloads.ToPod {
		// select delete pods from instances
		for idx := range instances {
			instance := instances[idx]
			for podIdx := range pods {
				if pods[podIdx].Name == instance && deltaReplicas > 0 {
					if _, exist := deletePodNames[instance]; !exist {
						deletePodList = append(deletePodList, pods[podIdx])
						deltaReplicas--
						break
					}
				}
			}
		}
		// calculate rest pod
		restPods := make([]*corev1.Pod, 0)
		for podIdx := range pods {
			isDelete := false
			for delPodIdx := range deletePodList {
				if deletePodList[delPodIdx].Name == pods[podIdx].Name {
					isDelete = true
					break
				}
			}
			if !isDelete {
				restPods = append(restPods, pods[podIdx])
			}
		}
		pods = restPods
		if deltaReplicas > 0 {
			var activePods podutils.ActivePods = pods
			sort.Sort(activePods)
			deletePodList = append(deletePodList, activePods[:deltaReplicas]...)
		}
	} else {
		for _, pod := range pods {
			subs := strings.Split(pod.Name, "-")
			if ordinal, err := strconv.ParseInt(subs[len(subs)-1], 10, 32); err != nil {
				return nil, err
			} else if int32(ordinal) < expectReplicas {
				continue
			}
			deletePodList = append(deletePodList, pod)
		}
	}
	return deletePodList, nil
}

func DeletePodFromInstances(pods []*corev1.Pod, instances []string, replicas int32, currentNodesAssignment []workloads.NodeAssignment) ([]workloads.NodeAssignment, error) {
	currentNodesAssignmentMap := make(map[string]workloads.NodeAssignment, 0)

	deletedPods, err := calculateDeletePods(pods, workloads.ToPod, replicas, -1, instances)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(currentNodesAssignment); i++ {
		currentNodeAssignment := currentNodesAssignment[i]
		currentNodesAssignmentMap[currentNodeAssignment.Name] = currentNodeAssignment
	}
	for idx := range deletedPods {
		deletedPod := deletedPods[idx]
		delete(currentNodesAssignmentMap, deletedPod.Name)
	}
	nodesAssignment := make([]workloads.NodeAssignment, 0)
	for _, val := range currentNodesAssignmentMap {
		nodesAssignment = append(nodesAssignment, val)
	}
	return nodesAssignment, nil
}

func AllocateNodesForPod(nodes []types.NodeName, replicas int32, clusterName, componentName string) []workloads.NodeAssignment {
	nodesAssignment := make([]workloads.NodeAssignment, 0)
	simpleNameGenerator := names.SimpleNameGenerator
	nodesLen := len(nodes)
	if nodesLen == 0 {
		for i := replicas; i > 0; i-- {
			podName := simpleNameGenerator.GenerateName(clusterName + "-" + componentName + "-")
			nodeAssignment := workloads.NodeAssignment{
				Name: podName,
			}
			nodesAssignment = append(nodesAssignment, nodeAssignment)
		}
	} else {
		for i := 0; i < int(replicas); i++ {
			podName := simpleNameGenerator.GenerateName(clusterName + "-" + componentName + "-")
			nodeAssignment := workloads.NodeAssignment{
				Name: podName,
				NodeSpec: workloads.NodeSpec{
					NodeName: nodes[i%nodesLen],
				},
			}
			nodesAssignment = append(nodesAssignment, nodeAssignment)
		}
	}
	return nodesAssignment
}
