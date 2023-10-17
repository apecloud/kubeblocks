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
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	lorry "github.com/apecloud/kubeblocks/lorry/client"
)

// ComponentWorkloadTransformer handles component rsm workload generation
type ComponentWorkloadTransformer struct {
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
	// workloadVertex is the vertex of the protoRSM workload in the DAG
	workloadVertex *ictrltypes.LifecycleVertex
}

var _ graph.Transformer = &ComponentWorkloadTransformer{}

func (t *ComponentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ComponentTransformContext)
	comp := transCtx.Component
	compOrig := transCtx.ComponentOrig
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent

	// build synthesizeComp podSpec volumeMounts
	buildPodSpecVolumeMounts(synthesizeComp)

	// build rsm workload
	// TODO(xingran): BuildRSM relies on the deprecated fields of the component, for example component.WorkloadType, which should be removed in the future
	rsm, err := factory.BuildRSM(cluster, synthesizeComp)
	if err != nil {
		return err
	}
	objects := []client.Object{rsm}

	// build PDB for backward compatibility
	// MinAvailable is used to determine whether to create a PDB (Pod Disruption Budget) object. However, the functionality of PDB should be implemented within the RSM.
	// Therefore, PDB objects are no longer needed in the new API, and the MinAvailable field should be deprecated.
	// The old MinAvailable field, which value is determined based on the deprecated "workloadType" field, is also no longer applicable in the new API.
	// TODO(xingran): which should be removed when workloadType and ClusterCompDefName are removed
	if synthesizeComp.MinAvailable != nil {
		pdb := factory.BuildPDB(cluster, synthesizeComp)
		objects = append(objects, pdb)
	}

	// read cache snapshot
	ml := constant.GetComponentWellKnownLabels(cluster.Name, comp.Name)
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, comp, ml, ownedWorkloadKinds()...)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	newSnapshot := make(map[model.GVKNObjKey]client.Object)
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		newSnapshot[*name] = object
	}

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	buildWorkloadVertex := func(dag *graph.DAG) *ictrltypes.LifecycleVertex {
		var rsmWorkloadVertex *ictrltypes.LifecycleVertex
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*ictrltypes.LifecycleVertex)
			if _, ok := v.Obj.(*workloads.ReplicatedStateMachine); ok {
				rsmWorkloadVertex = v
				break
			}
		}
		return rsmWorkloadVertex
	}

	createNewObjects := func() error {
		for name := range createSet {
			// TODO: use graphClient.Create(dag, newSnapshot[name]) instead
			createResource(dag, newSnapshot[name], nil)
		}
		return nil
	}

	updateObjects := func() error {
		for name := range updateSet {
			oldObj := oldSnapshot[name]
			newObj := copyAndMerge(oldObj, newSnapshot[name], cluster)
			// TODO: use graphClient.Update(dag, oldObj, newObj) instead
			updateResource(dag, newObj, root)

			// to work around that the scaled PVC will be deleted at object action.
			newRsmObj, ok := newObj.(*workloads.ReplicatedStateMachine)
			if !ok {
				continue
			}
			rsmWorkloadVertex := buildWorkloadVertex(dag)
			if rsmWorkloadVertex == nil {
				return errors.New("rsm workload vertex not found")
			}
			if err := updateVolumes(reqCtx, t.Client, synthesizeComp, newRsmObj, dag, rsmWorkloadVertex); err != nil {
				return err
			}

		}
		return nil
	}

	deleteOrphanObjects := func() error {
		for name := range deleteSet {
			// TODO: use graphClient.Delete(dag, oldSnapshot[name]) instead
			deleteResource(dag, oldSnapshot[name], nil)
		}
		return nil
	}

	// handle rsm workload restart/expandVolume/horizontalScale
	// TODO(xingran): Some RSM workload operations should be moved down to Lorry implementation. Subsequent operations such as horizontal scaling will be removed from the component controller
	rsmWorkloadOps := func() error {
		for name := range updateSet {
			oldRsmObj, ok := oldSnapshot[name].(*workloads.ReplicatedStateMachine)
			if !ok {
				continue
			}
			// build rsm workload vertex
			rsmWorkloadVertex := buildWorkloadVertex(dag)
			if rsmWorkloadVertex == nil {
				rsmWorkloadVertex = updateResource(dag, rsm, nil)
			}
			cwo := newComponentWorkloadOps(reqCtx, t.Client, cluster, synthesizeComp, oldRsmObj, rsm, rsmWorkloadVertex, dag)

			// handle rsm workload restart
			if err := cwo.restart(); err != nil {
				return err
			}

			// handle rsm expand volume
			if err := cwo.expandVolume(); err != nil {
				return err
			}

			// handle rsm workload horizontal scale
			if err := cwo.horizontalScale(); err != nil {
				return err
			}
			dag = cwo.dag
		}

		return nil
	}

	// handle rsm workload restart/expandVolume/horizontalScale ops
	if err := rsmWorkloadOps(); err != nil {
		return err
	}

	// objects to be created
	if err := createNewObjects(); err != nil {
		return err
	}

	// objects to be updated
	if err := updateObjects(); err != nil {
		return err
	}

	// objects to be deleted
	if err := deleteOrphanObjects(); err != nil {
		return err
	}

	return nil
}

func ownedWorkloadKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.ReplicatedStateMachineList{},
		&policyv1.PodDisruptionBudgetList{},
	}
}

// buildPodSpecVolumeMounts builds podSpec volumeMounts
func buildPodSpecVolumeMounts(synthesizeComp *component.SynthesizedComponent) {
	podSpec := synthesizeComp.PodSpec
	for _, cc := range []*[]corev1.Container{&podSpec.Containers, &podSpec.InitContainers} {
		volumes := podSpec.Volumes
		for _, c := range *cc {
			for _, v := range c.VolumeMounts {
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

// copyAndMerge merges two objects for updating:
// 1. new an object targetObj by copying from oldObj
// 2. merge all fields can be updated from newObj into targetObj
func copyAndMerge(oldObj, newObj client.Object, cluster *appsv1alpha1.Cluster) client.Object {
	if reflect.TypeOf(oldObj) != reflect.TypeOf(newObj) {
		return nil
	}

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
	buildWorkLoadAnnotations := func(obj client.Object, cluster *appsv1alpha1.Cluster) {
		workloadAnnotations := obj.GetAnnotations()
		if workloadAnnotations == nil {
			workloadAnnotations = map[string]string{}
		}
		// record the cluster generation to check if the sts is latest
		workloadAnnotations[constant.KubeBlocksGenerationKey] = strconv.FormatInt(cluster.Generation, 10)
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

	copyAndMergeRsm := func(oldRsm, newRsm *workloads.ReplicatedStateMachine) client.Object {
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
		buildWorkLoadAnnotations(rsmObjCopy, cluster)

		// keep the original template annotations.
		// if annotations exist and are replaced, the rsm will be updated.
		mergeMetadataMap(rsmObjCopy.Spec.Template.Annotations, &rsmProto.Spec.Template.Annotations)
		rsmObjCopy.Spec.Template = rsmProto.Spec.Template
		rsmObjCopy.Spec.Replicas = rsmProto.Spec.Replicas
		updateUpdateStrategy(rsmObjCopy, rsmProto)
		rsmObjCopy.Spec.Service = rsmProto.Spec.Service
		rsmObjCopy.Spec.AlternativeServices = rsmProto.Spec.AlternativeServices
		rsmObjCopy.Spec.Roles = rsmProto.Spec.Roles
		rsmObjCopy.Spec.RoleProbe = rsmProto.Spec.RoleProbe
		rsmObjCopy.Spec.MembershipReconfiguration = rsmProto.Spec.MembershipReconfiguration
		rsmObjCopy.Spec.MemberUpdateStrategy = rsmProto.Spec.MemberUpdateStrategy
		rsmObjCopy.Spec.Credential = rsmProto.Spec.Credential

		components.ResolvePodSpecDefaultFields(oldRsm.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)
		components.DelayUpdatePodSpecSystemFields(oldRsm.Spec.Template.Spec, &rsmObjCopy.Spec.Template.Spec)

		isTemplateUpdated := !reflect.DeepEqual(&oldRsm.Spec, &rsmObjCopy.Spec)
		if isTemplateUpdated {
			components.UpdatePodSpecSystemFields(&rsmObjCopy.Spec.Template.Spec)
		}

		return rsmObjCopy
	}

	copyAndMergePDB := func(oldPDB, newPDB *policyv1.PodDisruptionBudget) client.Object {
		pdbObjCopy := oldPDB.DeepCopy()
		mergeMetadataMap(pdbObjCopy.Annotations, &newPDB.Annotations)
		pdbObjCopy.Annotations = newPDB.Annotations
		pdbObjCopy.Spec = newPDB.Spec
		return pdbObjCopy
	}

	switch o := newObj.(type) {
	case *workloads.ReplicatedStateMachine:
		return copyAndMergeRsm(oldObj.(*workloads.ReplicatedStateMachine), o)
	case *policyv1.PodDisruptionBudget:
		return copyAndMergePDB(oldObj.(*policyv1.PodDisruptionBudget), o)
	default:
		return newObj
	}
}

// restart handles rsm workload restart by patch pod template annotation
func (r *componentWorkloadOps) restart() error {
	return components.RestartPod(&r.runningRSM.Spec.Template)
}

// expandVolume handles rsm workload expand volume
func (r *componentWorkloadOps) expandVolume() error {
	for _, vct := range r.runningRSM.Spec.VolumeClaimTemplates {
		var proto *corev1.PersistentVolumeClaimTemplate
		for _, v := range r.synthesizeComp.VolumeClaimTemplates {
			if v.Name == vct.Name {
				proto = &v
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
	sts := components.ConvertRSMToSTS(r.runningRSM)
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

	if err := r.updatePodReplicaLabel4Scaling(r.synthesizeComp.Replicas); err != nil {
		return err
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

	d, err := components.NewDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, stsObj, stsObj, snapshotKey)
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
		for _, obj := range tmpObjs {
			deleteResource(r.dag, obj, nil)
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
	err := r.leaveMember4ScaleIn()
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

	r.workloadVertex.Immutable = true
	rsmProto := r.workloadVertex.Obj.(*workloads.ReplicatedStateMachine)
	stsProto := components.ConvertRSMToSTS(rsmProto)
	d, err := components.NewDataClone(r.reqCtx, r.cli, r.cluster, r.synthesizeComp, stsObj, stsProto, backupKey)
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
		r.workloadVertex.Immutable = false
		return r.postScaleOut(stsObj)
	} else {
		r.workloadVertex.Immutable = true
		// update objs will trigger reconcile, no need to requeue error
		objs, err := d.CloneData(d)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			createResource(r.dag, obj, nil)
		}
		return nil
	}
}

func (r *componentWorkloadOps) updatePodReplicaLabel4Scaling(replicas int32) error {
	pods, err := components.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
	for _, pod := range pods {
		obj := pod.DeepCopy()
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(replicas))
		updateResource(r.dag, obj, r.workloadVertex)
	}
	return nil
}

func (r *componentWorkloadOps) leaveMember4ScaleIn() error {
	pods, err := components.ListPodOwnedByComponent(r.reqCtx.Ctx, r.cli, r.cluster.Namespace, constant.GetComponentWellKnownLabels(r.cluster.Name, r.synthesizeComp.Name))
	if err != nil {
		return err
	}
	for _, pod := range pods {
		subs := strings.Split(pod.Name, "-")
		if ordinal, err := strconv.ParseInt(subs[len(subs)-1], 10, 32); err != nil {
			return err
		} else if int32(ordinal) < r.synthesizeComp.Replicas {
			continue
		}
		lorryCli, err1 := lorry.NewClient(r.synthesizeComp.CharacterType, *pod)
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

		if err2 := lorryCli.LeaveMember(r.reqCtx.Ctx); err2 != nil {
			if err == nil {
				err = err2
			}
		}
	}
	return err // TODO: use requeue-after
}

func (r *componentWorkloadOps) deletePVCs4ScaleIn(stsObj *apps.StatefulSet) error {
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
			deleteResource(r.dag, &pvc, r.workloadVertex)
		}
	}
	return nil
}

func (r *componentWorkloadOps) expandVolumes(vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	pvcNotFound := false
	for i := *r.runningRSM.Spec.Replicas - 1; i >= 0; i-- {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: r.runningRSM.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s-%d", vctName, r.runningRSM.Name, i),
		}
		if err := r.cli.Get(r.reqCtx.Ctx, pvcKey, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				pvcNotFound = true
			} else {
				return err
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

	type pvcRecreateStep int
	const (
		pvPolicyRetainStep pvcRecreateStep = iota
		deletePVCStep
		removePVClaimRefStep
		createPVCStep
		pvRestorePolicyStep
	)

	addStepMap := map[pvcRecreateStep]func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex{
		pvPolicyRetainStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
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
			return patchResource(r.dag, retainPV, pv, fromVertex)
		},
		deletePVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := patchResource(r.dag, removeFinalizerPVC, pvc, fromVertex)
			return deleteResource(r.dag, pvc, removeFinalizerPVCVertex)
		},
		removePVClaimRefStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return patchResource(r.dag, removeClaimRefPV, pv, fromVertex)
		},
		createPVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return createResource(r.dag, newPVC, fromVertex)
		},
		pvRestorePolicyStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return patchResource(r.dag, restorePV, pv, fromVertex)
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := r.workloadVertex
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
		updateResource(r.dag, newPVC, r.workloadVertex)
		return nil
	}
	// all the else means no need to update

	return nil
}

func updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, synthesizeComp *component.SynthesizedComponent,
	rsmObj *workloads.ReplicatedStateMachine, dag *graph.DAG, workloadVertex *ictrltypes.LifecycleVertex) error {
	getRunningVolumes := func(vctName string) ([]*corev1.PersistentVolumeClaim, error) {
		pvcs, err := components.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature,
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
	for _, v := range ictrltypes.FindAll[*corev1.PersistentVolumeClaim](dag) {
		pvcNameSet.Insert(v.(*ictrltypes.LifecycleVertex).Obj.GetName())
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
			noopResource(dag, pvc, workloadVertex)
		}
	}
	return nil
}

func createResource(dag *graph.DAG, obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectCreate(dag, obj, parent)
}

func deleteResource(dag *graph.DAG, obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectDelete(dag, obj, parent)
}

func updateResource(dag *graph.DAG, obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectUpdate(dag, obj, parent)
}

func patchResource(dag *graph.DAG, obj client.Object, objCopy client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectPatch(dag, obj, objCopy, parent)
}

func noopResource(dag *graph.DAG, obj client.Object, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex {
	return ictrltypes.LifecycleObjectNoop(dag, obj, parent)
}

func newComponentWorkloadOps(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	synthesizeComp *component.SynthesizedComponent,
	runningRSM *workloads.ReplicatedStateMachine,
	protoRSM *workloads.ReplicatedStateMachine,
	workloadVertex *ictrltypes.LifecycleVertex,
	dag *graph.DAG) *componentWorkloadOps {
	return &componentWorkloadOps{
		cli:            cli,
		reqCtx:         reqCtx,
		cluster:        cluster,
		synthesizeComp: synthesizeComp,
		runningRSM:     runningRSM,
		protoRSM:       protoRSM,
		workloadVertex: workloadVertex,
		dag:            dag,
	}
}
