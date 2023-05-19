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

package lifecycle

import (
	"fmt"
	"reflect"

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type StsPVCTransformer struct{}

func (t *StsPVCTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster

	if isClusterDeleting(*origCluster) {
		return nil
	}

	// reference: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#recovering-from-failure-when-expanding-volumes
	// 1. Mark the PersistentVolume(PV) that is bound to the PersistentVolumeClaim(PVC) with Retain reclaim policy.
	// 2. Delete the PVC. Since PV has Retain reclaim policy - we will not lose any data when we recreate the PVC.
	// 3. Delete the claimRef entry from PV specs, so as new PVC can bind to it. This should make the PV Available.
	// 4. Re-create the PVC with smaller size than PV and set volumeName field of the PVC to the name of the PV. This should bind new PVC to existing PV.
	// 5. Don't forget to restore the reclaim policy of the PV.
	updatePVCSize := func(vertex *lifecycleVertex, pvcKey types.NamespacedName, pvc *corev1.PersistentVolumeClaim, pvcNotFound bool, vctProto *corev1.PersistentVolumeClaim) error {

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
			if err := transCtx.Client.List(transCtx.Context, &pvList, ml); err != nil {
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

		// for simple update
		simpleUpdateVertex := &lifecycleVertex{
			obj:    newPVC,
			oriObj: pvc,
			action: actionPtr(UPDATE),
		}

		pvNotFound := false

		// step 1: update pv to retain
		pv := &corev1.PersistentVolume{}
		pvKey := types.NamespacedName{
			Namespace: pvcKey.Namespace,
			Name:      newPVC.Spec.VolumeName,
		}
		if err := transCtx.Client.Get(transCtx.Context, pvKey, pv); err != nil {
			if errors.IsNotFound(err) {
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

		addStepVertex := func(fromVertex *lifecycleVertex, step pvcRecreateStep) *lifecycleVertex {
			switch step {
			case pvPolicyRetainStep:
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
				retainPVVertex := &lifecycleVertex{
					obj:    retainPV,
					oriObj: pv,
					action: actionPtr(PATCH),
				}
				dag.AddVertex(retainPVVertex)
				dag.Connect(fromVertex, retainPVVertex)
				return retainPVVertex
			case deletePVCStep:
				// step 2: delete pvc, this will not delete pv because policy is 'retain'
				deletePVCVertex := &lifecycleVertex{obj: pvc, action: actionPtr(DELETE)}
				removeFinalizerPVC := pvc.DeepCopy()
				removeFinalizerPVC.SetFinalizers([]string{})
				removeFinalizerPVCVertex := &lifecycleVertex{
					obj:    removeFinalizerPVC,
					oriObj: pvc,
					action: actionPtr(PATCH),
				}
				dag.AddVertex(deletePVCVertex)
				dag.AddVertex(removeFinalizerPVCVertex)
				dag.Connect(removeFinalizerPVCVertex, deletePVCVertex)
				dag.Connect(fromVertex, removeFinalizerPVCVertex)
				return deletePVCVertex
			case removePVClaimRefStep:
				// step 3: remove claimRef in pv
				removeClaimRefPV := pv.DeepCopy()
				if removeClaimRefPV.Spec.ClaimRef != nil {
					removeClaimRefPV.Spec.ClaimRef.UID = ""
					removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
				}
				removeClaimRefVertex := &lifecycleVertex{
					obj:    removeClaimRefPV,
					oriObj: pv,
					action: actionPtr(PATCH),
				}
				dag.AddVertex(removeClaimRefVertex)
				dag.Connect(fromVertex, removeClaimRefVertex)
				return removeClaimRefVertex
			case createPVCStep:
				// step 4: create new pvc
				newPVC.SetResourceVersion("")
				createNewPVCVertex := &lifecycleVertex{
					obj:    newPVC,
					action: actionPtr(CREATE),
				}
				dag.AddVertex(createNewPVCVertex)
				dag.Connect(fromVertex, createNewPVCVertex)
				return createNewPVCVertex
			case pvRestorePolicyStep:
				// step 5: restore to previous pv policy
				restorePV := pv.DeepCopy()
				policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
				if len(policy) == 0 {
					policy = corev1.PersistentVolumeReclaimDelete
				}
				restorePV.Spec.PersistentVolumeReclaimPolicy = policy
				restorePVVertex := &lifecycleVertex{
					obj:    restorePV,
					oriObj: pv,
					action: actionPtr(PATCH),
				}
				dag.AddVertex(restorePVVertex)
				dag.Connect(fromVertex, restorePVVertex)
				return restorePVVertex
			}
			return nil
		}

		updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
			lastVertex := vertex
			for i := pvRestorePolicyStep; i >= fromStep && i >= pvPolicyRetainStep; i-- {
				lastVertex = addStepVertex(lastVertex, i)
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
			dag.AddVertex(simpleUpdateVertex)
			dag.Connect(vertex, simpleUpdateVertex)
			return nil
		}
		// all the else means no need to update

		return nil
	}

	handlePVCUpdate := func(vertex *lifecycleVertex) error {
		stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
		stsProto, _ := vertex.obj.(*appsv1.StatefulSet)
		// check stsObj.Spec.VolumeClaimTemplates storage
		// request size and find attached PVC and patch request
		// storage size
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			var vctProto *corev1.PersistentVolumeClaim
			for _, v := range stsProto.Spec.VolumeClaimTemplates {
				if v.Name == vct.Name {
					vctProto = &v
					break
				}
			}

			// REVIEW: how could VCT proto is nil?
			if vctProto == nil {
				continue
			}

			pvcNotFound := false
			for i := *stsObj.Spec.Replicas - 1; i >= 0; i-- {
				pvc := &corev1.PersistentVolumeClaim{}
				pvcKey := types.NamespacedName{
					Namespace: stsObj.Namespace,
					Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
				}
				if err := transCtx.Client.Get(transCtx.Context, pvcKey, pvc); err != nil {
					if errors.IsNotFound(err) {
						pvcNotFound = true
					} else {
						return err
					}
				}

				if err := updatePVCSize(vertex, pvcKey, pvc, pvcNotFound, vctProto); err != nil {
					return err
				}
			}
		}
		return nil
	}

	vertices := findAll[*appsv1.StatefulSet](dag)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if v.obj != nil && v.oriObj != nil && v.action != nil && *v.action == UPDATE {
			if err := handlePVCUpdate(v); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ graph.Transformer = &StsPVCTransformer{}
