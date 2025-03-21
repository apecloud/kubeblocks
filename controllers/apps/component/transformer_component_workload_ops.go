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

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
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
	desiredCompPodNames   []string
	runningItsPodNames    []string
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
	compPodNames, err := generatePodNames(synthesizedComp)
	if err != nil {
		return nil, err
	}
	itsPodNames, err := generatePodNamesByITS(runningITS)
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
		desiredCompPodNames:   compPodNames,
		runningItsPodNames:    itsPodNames,
		desiredCompPodNameSet: sets.New(compPodNames...),
		runningItsPodNameSet:  sets.New(itsPodNames...),
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
	hasMemberLeaveDefined := r.synthesizeComp.LifecycleActions != nil && r.synthesizeComp.LifecycleActions.MemberLeave != nil
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

	trySwitchover := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		if lifecycleActions.Switchover == nil {
			return nil
		}
		err := lfa.Switchover(r.transCtx.Context, r.cli, nil, "")
		if err != nil {
			if errors.Is(err, lifecycle.ErrActionNotDefined) {
				return nil
			}
			return err
		}
		r.transCtx.Logger.Info("successfully call switchover action for pod", "pod", pod.Name)
		return nil
	}

	tryMemberLeave := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
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
		r.transCtx.Logger.Info("successfully call leave member action for pod", "pod", pod.Name)
		return nil
	}

	if lifecycleActions == nil || (lifecycleActions.Switchover == nil && lifecycleActions.MemberLeave == nil) {
		return nil
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		lifecycleActions, synthesizedComp.TemplateVars, pod, pods...)
	if err != nil {
		return err
	}

	if err := trySwitchover(lfa, pod); err != nil {
		return err
	}

	if err := tryMemberLeave(lfa, pod); err != nil {
		return err
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
			Context:             r.transCtx.Context,
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
	pods, err := component.ListOwnedPods(r.transCtx.Context, r.cli,
		r.synthesizeComp.Namespace, r.synthesizeComp.ClusterName, r.synthesizeComp.Name)
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
		synthesizedComp.LifecycleActions, synthesizedComp.TemplateVars, pod, pods...)
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

func (r *componentWorkloadOps) expandVolumes(vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	for _, pod := range r.runningItsPodNames {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: r.synthesizeComp.Namespace,
			Name:      fmt.Sprintf("%s-%s", vctName, pod),
		}
		pvcNotFound := false
		if err := r.cli.Get(r.transCtx.Context, pvcKey, pvc, appsutil.InDataContext4C()); err != nil {
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
				r.transCtx.Event(r.component, corev1.EventTypeWarning, "VolumeExpansionFailed", errMsg)
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
		if err := r.cli.List(r.transCtx.Context, &pvList, ml, appsutil.InDataContext4C()); err != nil {
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
		if err := r.cli.Get(r.transCtx.Context, pvKey, pv, appsutil.InDataContext4C()); err != nil {
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
			return graphCli.Do(r.dag, pv, retainPV, model.ActionPatchPtr(), fromVertex, appsutil.InDataContext4G())
		},
		deletePVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := graphCli.Do(r.dag, pvc, removeFinalizerPVC, model.ActionPatchPtr(), fromVertex, appsutil.InDataContext4G())
			return graphCli.Do(r.dag, nil, removeFinalizerPVC, model.ActionDeletePtr(), removeFinalizerPVCVertex, appsutil.InDataContext4G())
		},
		removePVClaimRefStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return graphCli.Do(r.dag, pv, removeClaimRefPV, model.ActionPatchPtr(), fromVertex, appsutil.InDataContext4G())
		},
		createPVCStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return graphCli.Do(r.dag, nil, newPVC, model.ActionCreatePtr(), fromVertex, appsutil.InDataContext4G())
		},
		pvRestorePolicyStep: func(fromVertex *model.ObjectVertex, step pvcRecreateStep) *model.ObjectVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return graphCli.Do(r.dag, pv, restorePV, model.ActionPatchPtr(), fromVertex, appsutil.InDataContext4G())
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
	if equality.Semantic.DeepEqual(pvc.Spec.Resources, newPVC.Spec.Resources) &&
		pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain &&
		pv.Annotations != nil &&
		len(pv.Annotations[constant.PVLastClaimPolicyAnnotationKey]) > 0 &&
		pv.Annotations[constant.PVLastClaimPolicyAnnotationKey] != string(corev1.PersistentVolumeReclaimRetain) {
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
		graphCli.Update(r.dag, nil, newPVC, appsutil.InDataContext4G())
		return nil
	}
	// all the else means no need to update

	return nil
}

func (r *componentWorkloadOps) buildProtoITSWorkloadVertex() *model.ObjectVertex {
	for _, vertex := range r.dag.Vertices() {
		v, _ := vertex.(*model.ObjectVertex)
		if v.Obj == r.protoITS {
			return v
		}
	}
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

	reconfigure := func(tpl component.SynthesizedFileTemplate, changes fileTemplateChanges) {
		var (
			action     *appsv1.Action
			actionName string
		)
		if tpl.ExternalManaged != nil && *tpl.ExternalManaged {
			if tpl.Reconfigure == nil {
				return // disabled by the external system
			}
		}
		action = tpl.Reconfigure
		actionName = component.UDFReconfigureActionName(tpl)
		if action == nil && synthesizedComp.LifecycleActions != nil {
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

	templateChanges := r.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
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
