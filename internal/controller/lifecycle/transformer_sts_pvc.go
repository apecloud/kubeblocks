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
	"time"

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

	// reference: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
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
		retainPV := pv.DeepCopy()
		if retainPV.Labels == nil {
			retainPV.Labels = make(map[string]string)
		}
		// add label to pv, in case pvc get deleted, and we can't find pv
		retainPV.Labels[constant.PVCNameLabelKey] = pvcKey.Name
		retainPV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		retainPVVertex := &lifecycleVertex{
			obj:    retainPV,
			oriObj: pv,
			action: actionPtr(PATCH),
		}

		// step 2: delete pvc, this will not delete pv because policy is 'retain'
		deletePVCVertex := &lifecycleVertex{obj: pvc, action: actionPtr(DELETE)}
		removeFinalizerPVC := pvc.DeepCopy()
		removeFinalizerPVC.SetFinalizers([]string{})
		removeFinalizerPVCVertex := &lifecycleVertex{
			obj:    removeFinalizerPVC,
			oriObj: pvc,
			action: actionPtr(PATCH),
		}

		// step 3: remove claimRef in pv
		removeClaimRefPV := retainPV.DeepCopy()
		removeClaimRefPV.Spec.ClaimRef = nil
		removeClaimRefVertex := &lifecycleVertex{
			obj:    removeClaimRefPV,
			oriObj: retainPV,
			action: actionPtr(PATCH),
		}

		// step 4: create new pvc
		newPVC.SetResourceVersion("")
		createNewPVCVertex := &lifecycleVertex{
			obj:    newPVC,
			action: actionPtr(CREATE),
		}

		// step 5: restore to previous pv policy
		restorePV := removeClaimRefPV.DeepCopy()
		restorePV.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
		restorePVVertex := &lifecycleVertex{
			obj:    restorePV,
			oriObj: removeClaimRefPV,
			action: actionPtr(PATCH),
		}

		targetQuantity := vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
		if pvcNotFound && !pvNotFound {
			// this could happen if some steps failed
			// step 3: remove claimRef in pv
			dag.AddVertex(removeClaimRefVertex)
			// step 4: create new pvc
			dag.AddVertex(createNewPVCVertex)
			dag.Connect(createNewPVCVertex, removeClaimRefVertex)
			// step 5: restore to previous pv policy
			dag.AddVertex(restorePVVertex)
			dag.Connect(restorePVVertex, createNewPVCVertex)
			dag.Connect(vertex, restorePVVertex)
		} else if pvcNotFound && pvNotFound {
			// if both pvc and pv not found, do nothing
		} else if reflect.DeepEqual(pvc.Spec.Resources, newPVC.Spec.Resources) && pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
			// this could happen if create pvc succeeded but last step failed
			// step 5: restore to previous pv policy
			dag.AddVertex(restorePVVertex)
			dag.Connect(vertex, restorePVVertex)
		} else if pvcQuantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; pvcQuantity.Cmp(targetQuantity) == 1 && // check if it's compressing volume
			targetQuantity.Cmp(*pvc.Status.Capacity.Storage()) >= 0 { // check if target size is greater than or equal to actual size
			// this branch means we can update pvc size by recreate it
			tmpPV := &corev1.PersistentVolume{}
			if err := transCtx.Client.Get(transCtx.Context, pvKey, tmpPV); client.IgnoreNotFound(err) != nil {
				return err
			}
			if tmpPV.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
				// make sure pv policy is 'retain' before deleting pvc
				// step 2: delete pvc, this will not delete pv because policy is 'retain'
				dag.AddVertex(deletePVCVertex)
				dag.AddVertex(removeFinalizerPVCVertex)
				dag.Connect(removeFinalizerPVCVertex, deletePVCVertex)
				// step 3: remove claimRef in pv
				dag.AddVertex(removeClaimRefVertex)
				dag.Connect(removeClaimRefVertex, deletePVCVertex)
				// step 4: create new pvc
				dag.AddVertex(createNewPVCVertex)
				dag.Connect(createNewPVCVertex, removeClaimRefVertex)
				// step 5: restore to previous pv policy
				dag.AddVertex(restorePVVertex)
				dag.Connect(restorePVVertex, createNewPVCVertex)
				dag.Connect(vertex, restorePVVertex)
			} else {
				// step 1: update pv to retain
				dag.AddVertex(retainPVVertex)
				dag.Connect(vertex, retainPVVertex)
				return newRequeueError(time.Second, "pv not in retain policy")
			}
		} else if pvcQuantity.Cmp(vctProto.Spec.Resources.Requests[corev1.ResourceStorage]) != 0 {
			// use pvc's update without anything extra
			dag.AddVertex(simpleUpdateVertex)
			dag.Connect(vertex, simpleUpdateVertex)
		} else {
			// this branch means no need to update
		}

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
