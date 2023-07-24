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

package components

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// statefulComponentBase as a base class for single stateful-set based component (stateful & replication & consensus).
type statefulComponentBase struct {
	componentBase
	// runningWorkload can be nil, and the replicas of workload can be nil (zero)
	runningWorkload *appsv1.StatefulSet
}

func (c *statefulComponentBase) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder, load bool) error {
	var err error
	if builder != nil {
		if err = builder.BuildEnv().
			BuildWorkload().
			BuildPDB().
			BuildHeadlessService().
			BuildConfig().
			BuildTLSVolume().
			BuildVolumeMount().
			BuildService().
			BuildTLSCert().
			Complete(); err != nil {
			return err
		}
	}
	if load {
		c.runningWorkload, err = c.loadRunningWorkload(reqCtx, cli)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *statefulComponentBase) loadRunningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.StatefulSet, error) {
	stsList, err := listStsOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}
	cnt := len(stsList)
	if cnt == 1 {
		return stsList[0], nil
	}
	if cnt == 0 {
		return nil, nil
	} else {
		return nil, fmt.Errorf("more than one workloads found for the component, cluster: %s, component: %s, cnt: %d",
			c.GetClusterName(), c.GetName(), cnt)
	}
}

func (c *statefulComponentBase) GetBuiltObjects(builder componentWorkloadBuilder) ([]client.Object, error) {
	dag := c.Dag
	defer func() {
		c.Dag = dag
	}()

	c.Dag = graph.NewDAG()
	if err := c.init(intctrlutil.RequestCtx{}, nil, builder, false); err != nil {
		return nil, err
	}

	objs := make([]client.Object, 0)
	for _, v := range c.Dag.Vertices() {
		if vv, ok := v.(*ictrltypes.LifecycleVertex); ok {
			objs = append(objs, vv.Obj)
		}
	}
	return objs, nil
}

func (c *statefulComponentBase) Create(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, false); err != nil {
		return err
	}

	if err := c.ValidateObjectsAction(); err != nil {
		return err
	}

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "Create a new component")

	return nil
}

func (c *statefulComponentBase) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(impl): delete component owned resources
	return nil
}

func (c *statefulComponentBase) Update(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}

	if c.runningWorkload != nil {
		if err := c.Restart(reqCtx, cli); err != nil {
			return err
		}

		// cluster.spec.componentSpecs[*].volumeClaimTemplates[*].spec.resources.requests[corev1.ResourceStorage]
		if err := c.ExpandVolume(reqCtx, cli); err != nil {
			return err
		}

		// cluster.spec.componentSpecs[*].replicas
		if err := c.HorizontalScale(reqCtx, cli); err != nil {
			return err
		}
	}

	if err := c.updateUnderlyingResources(reqCtx, cli, c.runningWorkload); err != nil {
		return err
	}

	return c.ResolveObjectsAction(reqCtx, cli)
}

func (c *statefulComponentBase) Status(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}
	if c.runningWorkload == nil {
		return nil
	}

	statusTxn := &statusReconciliationTxn{}

	if err := c.statusUpdateTemplateRenderedConfig(); err != nil {
		return err
	}

	if err := c.statusExpandVolume(reqCtx, cli, statusTxn); err != nil {
		return err
	}

	if err := c.horizontalScale(reqCtx, cli, statusTxn); err != nil {
		return err
	}

	if vertexes, err := c.ComponentSet.HandleRoleChange(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for _, v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	// TODO(impl): restart pod if needed, move it to @Update and restart pod directly.
	if vertexes, err := c.ComponentSet.HandleRestart(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for _, v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	var delayedRequeueError error
	if err := c.StatusWorkload(reqCtx, cli, c.runningWorkload, statusTxn); err != nil {
		if !intctrlutil.IsDelayedRequeueError(err) {
			return err
		}
		delayedRequeueError = err
	}

	if err := statusTxn.commit(); err != nil {
		return err
	}

	c.updateWorkload(c.runningWorkload)
	return delayedRequeueError
}

func (c *statefulComponentBase) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return restartPod(&c.runningWorkload.Spec.Template)
}

func (c *statefulComponentBase) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		var proto *corev1.PersistentVolumeClaimTemplate
		for _, v := range c.Component.VolumeClaimTemplates {
			if v.Name == vct.Name {
				proto = &v
				break
			}
		}
		// REVIEW: seems we can remove a volume claim from templates at runtime, without any changes and warning messages?
		if proto == nil {
			continue
		}

		if err := c.expandVolumes(reqCtx, cli, vct.Name, proto); err != nil {
			return err
		}
	}
	return nil
}

func (c *statefulComponentBase) expandVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client,
	vctName string, proto *corev1.PersistentVolumeClaimTemplate) error {
	pvcNotFound := false
	for i := *c.runningWorkload.Spec.Replicas - 1; i >= 0; i-- {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcKey := types.NamespacedName{
			Namespace: c.GetNamespace(),
			Name:      fmt.Sprintf("%s-%s-%d", vctName, c.runningWorkload.Name, i),
		}
		if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
			if apierrors.IsNotFound(err) {
				pvcNotFound = true
			} else {
				return err
			}
		}
		if err := c.updatePVCSize(reqCtx, cli, pvcKey, pvc, pvcNotFound, proto); err != nil {
			return err
		}
	}
	return nil
}

func (c *statefulComponentBase) updatePVCSize(reqCtx intctrlutil.RequestCtx, cli client.Client, pvcKey types.NamespacedName,
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
		if err := cli.List(reqCtx.Ctx, &pvList, ml); err != nil {
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
	if err := cli.Get(reqCtx.Ctx, pvKey, pv); err != nil {
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
			return c.PatchResource(retainPV, pv, fromVertex)
		},
		deletePVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 2: delete pvc, this will not delete pv because policy is 'retain'
			removeFinalizerPVC := pvc.DeepCopy()
			removeFinalizerPVC.SetFinalizers([]string{})
			removeFinalizerPVCVertex := c.PatchResource(removeFinalizerPVC, pvc, fromVertex)
			return c.DeleteResource(pvc, removeFinalizerPVCVertex)
		},
		removePVClaimRefStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 3: remove claimRef in pv
			removeClaimRefPV := pv.DeepCopy()
			if removeClaimRefPV.Spec.ClaimRef != nil {
				removeClaimRefPV.Spec.ClaimRef.UID = ""
				removeClaimRefPV.Spec.ClaimRef.ResourceVersion = ""
			}
			return c.PatchResource(removeClaimRefPV, pv, fromVertex)
		},
		createPVCStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 4: create new pvc
			newPVC.SetResourceVersion("")
			return c.CreateResource(newPVC, fromVertex)
		},
		pvRestorePolicyStep: func(fromVertex *ictrltypes.LifecycleVertex, step pvcRecreateStep) *ictrltypes.LifecycleVertex {
			// step 5: restore to previous pv policy
			restorePV := pv.DeepCopy()
			policy := corev1.PersistentVolumeReclaimPolicy(restorePV.Annotations[constant.PVLastClaimPolicyAnnotationKey])
			if len(policy) == 0 {
				policy = corev1.PersistentVolumeReclaimDelete
			}
			restorePV.Spec.PersistentVolumeReclaimPolicy = policy
			return c.PatchResource(restorePV, pv, fromVertex)
		},
	}

	updatePVCByRecreateFromStep := func(fromStep pvcRecreateStep) {
		lastVertex := c.WorkloadVertex
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
		c.UpdateResource(newPVC, c.WorkloadVertex)
		return nil
	}
	// all the else means no need to update

	return nil
}

// statusUpdateTemplateRenderedConfig transform the action of rendered config by config-template from noop to update in cluster Status reconciliation
func (c *statefulComponentBase) statusUpdateTemplateRenderedConfig() error {
	for _, v := range ictrltypes.FindAll[*corev1.ConfigMap](c.Dag) {
		node, _ := v.(*ictrltypes.LifecycleVertex)
		cm, _ := node.Obj.(*corev1.ConfigMap)
		_, ok := cm.GetLabels()[constant.CMConfigurationTypeLabelKey]
		// generated by config-template and action is default action, so we need to change the default action
		if ok && *node.Action == ictrltypes.NOOP {
			newFlag := cm.Annotations[constant.CMConfigurationNewAnnotationKey]
			if newFlag == "true" {
				node.Action = ictrltypes.ActionCreatePtr()
			} else {
				node.Action = ictrltypes.ActionUpdatePtr()
			}
		}
	}
	return nil
}

func (c *statefulComponentBase) statusExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		running, failed, err := c.hasVolumeExpansionRunning(reqCtx, cli, vct.Name)
		if err != nil {
			return err
		}
		if failed {
			txn.propose(appsv1alpha1.AbnormalClusterCompPhase, func() {
				c.SetStatusPhase(appsv1alpha1.AbnormalClusterCompPhase, nil, "Volume Expansion failed")
			})
			return nil
		}
		if running {
			txn.propose(appsv1alpha1.SpecReconcilingClusterCompPhase, func() {
				c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, nil, "Volume Expansion failed")
			})
			return nil
		}
	}
	return nil
}

func (c *statefulComponentBase) hasVolumeExpansionRunning(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string) (bool, bool, error) {
	var (
		running bool
		failed  bool
	)
	volumes, err := c.getRunningVolumes(reqCtx, cli, vctName, c.runningWorkload)
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
	return running, failed, nil
}

func (c *statefulComponentBase) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.horizontalScale(reqCtx, cli, nil)
}

func (c *statefulComponentBase) horizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
	sts := c.runningWorkload
	if sts.Status.ReadyReplicas == c.Component.Replicas {
		return nil
	}
	ret := c.horizontalScaling(sts)
	if ret == 0 {
		if err := c.postScaleIn(reqCtx, cli, txn); err != nil {
			return err
		}
		if err := c.postScaleOut(reqCtx, cli, sts); err != nil {
			return err
		}
		return nil
	}
	if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, sts); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli, sts); err != nil {
			return err
		}
	}

	if err := c.updatePodReplicaLabel4Scaling(reqCtx, cli, c.Component.Replicas); err != nil {
		return err
	}

	// update KB_<component-type>_<pod-idx>_<hostname> env needed by pod to obtain hostname.
	c.updatePodEnvConfig()

	reqCtx.Recorder.Eventf(c.Cluster,
		corev1.EventTypeNormal,
		"HorizontalScale",
		"start horizontal scale component %s of cluster %s from %d to %d",
		c.GetName(), c.GetClusterName(), int(c.Component.Replicas)-ret, c.Component.Replicas)

	return nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *statefulComponentBase) horizontalScaling(stsObj *appsv1.StatefulSet) int {
	return int(c.Component.Replicas - *stsObj.Spec.Replicas)
}

func (c *statefulComponentBase) updatePodEnvConfig() {
	for _, v := range ictrltypes.FindAll[*corev1.ConfigMap](c.Dag) {
		node := v.(*ictrltypes.LifecycleVertex)
		// TODO: need a way to reference the env config.
		envConfigName := fmt.Sprintf("%s-%s-env", c.GetClusterName(), c.GetName())
		if node.Obj.GetName() == envConfigName {
			node.Action = ictrltypes.ActionUpdatePtr()
		}
	}
}

func (c *statefulComponentBase) updatePodReplicaLabel4Scaling(reqCtx intctrlutil.RequestCtx, cli client.Client, replicas int32) error {
	pods, err := listPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}
	for _, pod := range pods {
		obj := pod.DeepCopy()
		if obj.Annotations == nil {
			obj.Annotations = make(map[string]string)
		}
		obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(replicas))
		c.UpdateResource(obj, c.WorkloadVertex)
	}
	return nil
}

func (c *statefulComponentBase) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// if scale in to 0, do not delete pvcs
	if c.Component.Replicas == 0 {
		return nil
	}
	for i := c.Component.Replicas; i < *stsObj.Spec.Replicas; i++ {
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			pvc := corev1.PersistentVolumeClaim{}
			if err := cli.Get(reqCtx.Ctx, pvcKey, &pvc); err != nil {
				return err
			}
			c.DeleteResource(&pvc, nil)
		}
	}
	return nil
}

func (c *statefulComponentBase) postScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
	return nil
}

func (c *statefulComponentBase) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
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

	c.WorkloadVertex.Immutable = true
	stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
	d, err := newDataClone(reqCtx, cli, c.Cluster, c.Component, stsObj, stsProto, backupKey)
	if err != nil {
		return err
	}
	var succeed bool
	if d == nil {
		succeed = true
	} else {
		succeed, err = d.succeed()
		if err != nil {
			return err
		}
	}
	if succeed {
		// pvcs are ready, stateful_set.replicas should be updated
		c.WorkloadVertex.Immutable = false
		return c.postScaleOut(reqCtx, cli, stsObj)
	} else {
		c.WorkloadVertex.Immutable = true
		// update objs will trigger cluster reconcile, no need to requeue error
		objs, err := d.cloneData(d)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.CreateResource(obj, nil)
		}
		return nil
	}
}

func (c *statefulComponentBase) postScaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
	)

	d, err := newDataClone(reqCtx, cli, c.Cluster, c.Component, stsObj, stsObj, snapshotKey)
	if err != nil {
		return err
	}
	if d != nil {
		// clean backup resources.
		// there will not be any backup resources other than scale out.
		tmpObjs, err := d.clearTmpResources()
		if err != nil {
			return err
		}
		for _, obj := range tmpObjs {
			c.DeleteResource(obj, nil)
		}
	}

	return nil
}

func (c *statefulComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	if stsObj == nil {
		c.createWorkload()
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, nil, "Component workload created")
	} else {
		if c.updateWorkload(stsObj) {
			c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, nil, "Component workload updated")
		}
		// to work around that the scaled PVC will be deleted at object action.
		if err := c.updateVolumes(reqCtx, cli, stsObj); err != nil {
			return err
		}
	}
	if err := c.UpdatePDB(reqCtx, cli); err != nil {
		return err
	}
	if err := c.UpdateService(reqCtx, cli); err != nil {
		return err
	}
	// update KB_<component-type>_<pod-idx>_<hostname> env needed by pod to obtain hostname.
	c.updatePodEnvConfig()
	return nil
}

func (c *statefulComponentBase) createWorkload() {
	stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
	c.WorkloadVertex.Obj = stsProto
	c.WorkloadVertex.Action = ictrltypes.ActionCreatePtr()
}

func (c *statefulComponentBase) updateWorkload(stsObj *appsv1.StatefulSet) bool {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	mergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	buildWorkLoadAnnotations(stsObjCopy, c.Cluster)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	c.updateUpdateStrategy(stsObjCopy, stsProto)

	resolvePodSpecDefaultFields(stsObj.Spec.Template.Spec, &stsObjCopy.Spec.Template.Spec)

	delayUpdatePodSpecSystemFields(stsObj.Spec.Template.Spec, &stsObjCopy.Spec.Template.Spec)

	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		updatePodSpecSystemFields(&stsObjCopy.Spec.Template.Spec)
		c.WorkloadVertex.Obj = stsObjCopy
		c.WorkloadVertex.Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		return true
	}
	return false
}

func (c *statefulComponentBase) updateUpdateStrategy(stsObj, stsProto *appsv1.StatefulSet) {
	var objMaxUnavailable *intstr.IntOrString
	if stsObj.Spec.UpdateStrategy.RollingUpdate != nil {
		objMaxUnavailable = stsObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
	}
	stsObj.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if objMaxUnavailable == nil && stsObj.Spec.UpdateStrategy.RollingUpdate != nil {
		// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
		// MaxUnavailableStatefulSet feature.
		// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
		// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
		// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
		stsObj.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = nil
	}
}

func (c *statefulComponentBase) updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, v := range ictrltypes.FindAll[*corev1.PersistentVolumeClaim](c.Dag) {
		pvcNameSet.Insert(v.(*ictrltypes.LifecycleVertex).Obj.GetName())
	}

	for _, vct := range c.Component.VolumeClaimTemplates {
		pvcs, err := c.getRunningVolumes(reqCtx, cli, vct.Name, stsObj)
		if err != nil {
			return err
		}
		for _, pvc := range pvcs {
			if pvcNameSet.Has(pvc.Name) {
				continue
			}
			c.NoopResource(pvc, c.WorkloadVertex)
		}
	}
	return nil
}

func (c *statefulComponentBase) getRunningVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string,
	stsObj *appsv1.StatefulSet) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := listObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	matchedPVCs := make([]*corev1.PersistentVolumeClaim, 0)
	prefix := fmt.Sprintf("%s-%s", vctName, stsObj.Name)
	for _, pvc := range pvcs {
		if strings.HasPrefix(pvc.Name, prefix) {
			matchedPVCs = append(matchedPVCs, pvc)
		}
	}
	return matchedPVCs, nil
}
