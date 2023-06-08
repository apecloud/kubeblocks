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

package internal

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// StatefulComponentBase as a base class for single stateful-set based component (stateful & replication & consensus).
type StatefulComponentBase struct {
	ComponentBase
	// runningWorkload can be nil, and the replicas of workload can be nil (zero)
	runningWorkload *appsv1.StatefulSet
}

func (c *StatefulComponentBase) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder ComponentWorkloadBuilder, load bool) error {
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

func (c *StatefulComponentBase) loadRunningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.StatefulSet, error) {
	stsList, err := util.ListStsOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
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

func (c *StatefulComponentBase) GetBuiltObjects(builder ComponentWorkloadBuilder) ([]client.Object, error) {
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

func (c *StatefulComponentBase) Create(reqCtx intctrlutil.RequestCtx, cli client.Client, builder ComponentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, false); err != nil {
		return err
	}

	if err := c.ValidateObjectsAction(); err != nil {
		return err
	}

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "Create a new component")

	return nil
}

func (c *StatefulComponentBase) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(impl): delete component owned resources
	return nil
}

func (c *StatefulComponentBase) Update(reqCtx intctrlutil.RequestCtx, cli client.Client, builder ComponentWorkloadBuilder) error {
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

func (c *StatefulComponentBase) Status(reqCtx intctrlutil.RequestCtx, cli client.Client, builder ComponentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder, true); err != nil {
		return err
	}
	if c.runningWorkload == nil {
		return nil
	}

	statusTxn := &statusReconciliationTxn{}

	if err := c.statusExpandVolume(reqCtx, cli, statusTxn); err != nil {
		return err
	}

	if err := c.statusHorizontalScale(reqCtx, cli, statusTxn); err != nil {
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

	if vertexes, err := c.ComponentSet.HandleSwitchover(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for _, v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	if vertexes, err := c.ComponentSet.HandleFailover(reqCtx.Ctx, c.runningWorkload); err != nil {
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

	if err := c.handleGarbageOfRestoreBeforeRunning(); err != nil {
		return err
	}
	c.updateWorkload(c.runningWorkload)
	return delayedRequeueError
}

func (c *StatefulComponentBase) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return util.RestartPod(&c.runningWorkload.Spec.Template)
}

func (c *StatefulComponentBase) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *StatefulComponentBase) expandVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client,
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

func (c *StatefulComponentBase) updatePVCSize(reqCtx intctrlutil.RequestCtx, cli client.Client, pvcKey types.NamespacedName,
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

func (c *StatefulComponentBase) statusExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
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

func (c *StatefulComponentBase) hasVolumeExpansionRunning(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string) (bool, bool, error) {
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

func (c *StatefulComponentBase) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	ret := c.horizontalScaling(c.runningWorkload)
	if ret == 0 {
		return nil
	}
	if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, c.runningWorkload); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli, c.runningWorkload); err != nil {
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
func (c *StatefulComponentBase) horizontalScaling(stsObj *appsv1.StatefulSet) int {
	return int(c.Component.Replicas - *stsObj.Spec.Replicas)
}

func (c *StatefulComponentBase) updatePodEnvConfig() {
	for _, v := range ictrltypes.FindAll[*corev1.ConfigMap](c.Dag) {
		node := v.(*ictrltypes.LifecycleVertex)
		// TODO: need a way to reference the env config.
		envConfigName := fmt.Sprintf("%s-%s-env", c.GetClusterName(), c.GetName())
		if node.Obj.GetName() == envConfigName {
			node.Action = ictrltypes.ActionUpdatePtr()
		}
	}
}

func (c *StatefulComponentBase) updatePodReplicaLabel4Scaling(reqCtx intctrlutil.RequestCtx, cli client.Client, replicas int32) error {
	pods, err := util.ListPodOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
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

func (c *StatefulComponentBase) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	for i := c.Component.Replicas; i < *stsObj.Spec.Replicas; i++ {
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			// create cronjob to delete pvc after 30 minutes
			if obj, err := checkedCreateDeletePVCCronJob(reqCtx, cli, pvcKey, stsObj, c.Cluster); err != nil {
				return err
			} else if obj != nil {
				c.CreateResource(obj, nil)
			}
		}
	}
	return nil
}

func (c *StatefulComponentBase) postScaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
	hasJobFailed := func(reqCtx intctrlutil.RequestCtx, cli client.Client) (*batchv1.Job, string, error) {
		jobs, err := util.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.JobSignature, c.GetNamespace(), c.GetMatchingLabels())
		if err != nil {
			return nil, "", err
		}
		for _, job := range jobs {
			// TODO: use a better way to check the delete PVC job.
			if !strings.HasPrefix(job.Name, "delete-pvc-") {
				continue
			}
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					return job, fmt.Sprintf("%s-%s", cond.Reason, cond.Message), nil
				}
			}
		}
		return nil, "", nil
	}
	if job, msg, err := hasJobFailed(reqCtx, cli); err != nil {
		return err
	} else if job != nil {
		msgKey := fmt.Sprintf("%s/%s", job.GetObjectKind().GroupVersionKind().Kind, job.GetName())
		statusMessage := appsv1alpha1.ComponentMessageMap{msgKey: msg}
		txn.propose(appsv1alpha1.AbnormalClusterCompPhase, func() {
			c.SetStatusPhase(appsv1alpha1.AbnormalClusterCompPhase, statusMessage, "PVC deletion job failed")
		})
	}
	return nil
}

func (c *StatefulComponentBase) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		key = client.ObjectKey{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name,
		}
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
		horizontalScalePolicy = c.Component.HorizontalScalePolicy

		cleanCronJobs = func() error {
			for i := *stsObj.Spec.Replicas; i < c.Component.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// delete deletion cronjob if exists
					cronJobKey := pvcKey
					cronJobKey.Name = "delete-pvc-" + pvcKey.Name
					cronJob := &batchv1.CronJob{}
					if err := cli.Get(reqCtx.Ctx, cronJobKey, cronJob); err != nil {
						return client.IgnoreNotFound(err)
					}
					c.DeleteResource(cronJob, c.WorkloadVertex)
				}
			}
			return nil
		}

		checkAllPVCsExist = func() (bool, error) {
			for i := *stsObj.Spec.Replicas; i < c.Component.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// check pvc existence
					pvcExists, err := isPVCExists(cli, reqCtx.Ctx, pvcKey)
					if err != nil {
						return true, err
					}
					if !pvcExists {
						return false, nil
					}
				}
			}
			return true, nil
		}
	)

	if err := cleanCronJobs(); err != nil {
		return err
	}

	allPVCsExist, err := checkAllPVCsExist()
	if err != nil {
		return err
	}
	if !allPVCsExist {
		if horizontalScalePolicy == nil {
			c.WorkloadVertex.Immutable = false
			return nil
		}
		// do backup according to component's horizontal scale policy
		c.WorkloadVertex.Immutable = true
		stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
		objs, err := doBackup(reqCtx, cli, c.Cluster, c.Component, snapshotKey, stsProto, stsObj)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.CreateResource(obj, nil)
		}
		return nil
	}
	// pvcs are ready, stateful_set.replicas should be updated
	c.WorkloadVertex.Immutable = false

	return c.postScaleOut(reqCtx, cli, stsObj)
}

func (c *StatefulComponentBase) postScaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	var (
		snapshotKey = types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
		horizontalScalePolicy = c.Component.HorizontalScalePolicy

		checkAllPVCBoundIfNeeded = func() (bool, error) {
			if horizontalScalePolicy == nil ||
				horizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(cli, reqCtx.Ctx) {
				return true, nil
			}
			return isAllPVCBound(cli, reqCtx.Ctx, stsObj, int(c.Component.Replicas))
		}

		cleanBackupResourcesIfNeeded = func() error {
			if horizontalScalePolicy == nil ||
				horizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(cli, reqCtx.Ctx) {
				return nil
			}
			// if all pvc bounded, clean backup resources
			objs, err := deleteSnapshot(cli, reqCtx, snapshotKey, c.GetCluster(), c.GetName())
			if err != nil {
				return err
			}
			for _, obj := range objs {
				c.DeleteResource(obj, nil)
			}
			return nil
		}
	)

	// check all pvc bound, wait next reconciliation if not all ready
	allPVCBounded, err := checkAllPVCBoundIfNeeded()
	if err != nil {
		return err
	}
	if !allPVCBounded {
		return nil
	}
	// clean backup resources.
	// there will not be any backup resources other than scale out.
	if err := cleanBackupResourcesIfNeeded(); err != nil {
		return err
	}

	return nil
}

func (c *StatefulComponentBase) statusHorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client, txn *statusReconciliationTxn) error {
	ret := c.horizontalScaling(c.runningWorkload)
	if ret < 0 {
		return nil
	}
	if ret > 0 {
		// forward the h-scaling progress.
		return c.scaleOut(reqCtx, cli, c.runningWorkload)
	}
	if ret == 0 { // sts has been updated
		if err := c.postScaleIn(reqCtx, cli, txn); err != nil {
			return err
		}
		if err := c.postScaleOut(reqCtx, cli, c.runningWorkload); err != nil {
			return err
		}
	}
	return nil
}

func (c *StatefulComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
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
	return nil
}

func (c *StatefulComponentBase) createWorkload() {
	stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
	c.WorkloadVertex.Obj = stsProto
	c.WorkloadVertex.Action = ictrltypes.ActionCreatePtr()
}

func (c *StatefulComponentBase) updateWorkload(stsObj *appsv1.StatefulSet) bool {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	util.MergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	util.BuildWorkLoadAnnotations(stsObjCopy, c.Cluster)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		// TODO(REVIEW): always return true and update component phase to Updating. stsObj.Spec contains default values which set by Kubernetes
		c.WorkloadVertex.Obj = stsObjCopy
		c.WorkloadVertex.Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		return true
	}
	return false
}

func (c *StatefulComponentBase) updateVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
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

func (c *StatefulComponentBase) getRunningVolumes(reqCtx intctrlutil.RequestCtx, cli client.Client, vctName string,
	stsObj *appsv1.StatefulSet) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs, err := util.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.PersistentVolumeClaimSignature, c.GetNamespace(), c.GetMatchingLabels())
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

// handleGarbageOfRestoreBeforeRunning handles the garbage for restore before cluster phase changes to Running.
// @return ErrNoOps if no operation
// REVIEW: this handling is rather hackish, call for refactor.
// Deprecated: to be removed by PITR feature.
func (c *StatefulComponentBase) handleGarbageOfRestoreBeforeRunning() error {
	clusterBackupResourceMap, err := c.getClusterBackupSourceMap(c.GetCluster())
	if err != nil {
		return err
	}
	if clusterBackupResourceMap == nil {
		return nil
	}
	if c.GetPhase() != appsv1alpha1.RunningClusterCompPhase {
		return nil
	}

	// remove the garbage for restore if the component restores from backup.
	for _, v := range clusterBackupResourceMap {
		// remove the init container for restore
		if err = c.removeStsInitContainerForRestore(v); err != nil {
			return err
		}
	}
	// TODO: remove from the cluster annotation RestoreFromBackUpAnnotationKey?
	return nil
}

// getClusterBackupSourceMap gets the backup source map from cluster.annotations
func (c *StatefulComponentBase) getClusterBackupSourceMap(cluster *appsv1alpha1.Cluster) (map[string]string, error) {
	compBackupMapString := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	if len(compBackupMapString) == 0 {
		return nil, nil
	}
	compBackupMap := map[string]string{}
	err := json.Unmarshal([]byte(compBackupMapString), &compBackupMap)
	for k := range compBackupMap {
		if cluster.Spec.GetComponentByName(k) == nil {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeNotFound, "restore: not found componentSpecs[*].name %s", k)
		}
	}
	return compBackupMap, err
}

// removeStsInitContainerForRestore removes the statefulSet's init container which restores data from backup.
func (c *StatefulComponentBase) removeStsInitContainerForRestore(backupName string) error {
	sts := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
	initContainers := sts.Spec.Template.Spec.InitContainers
	restoreInitContainerName := component.GetRestoredInitContainerName(backupName)
	restoreInitContainerIndex, _ := intctrlutil.GetContainerByName(initContainers, restoreInitContainerName)
	if restoreInitContainerIndex == -1 {
		return nil
	}

	initContainers = append(initContainers[:restoreInitContainerIndex], initContainers[restoreInitContainerIndex+1:]...)
	sts.Spec.Template.Spec.InitContainers = initContainers
	if *c.WorkloadVertex.Action != ictrltypes.UPDATE {
		if *c.WorkloadVertex.Action != ictrltypes.CREATE && *c.WorkloadVertex.Action != ictrltypes.DELETE {
			c.WorkloadVertex.Action = ictrltypes.ActionUpdatePtr()
		}
	}
	// TODO: it seems not reasonable to reset component phase back to Creating.
	//// if need to remove init container, reset component to Creating.
	// c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, "Remove init container for restore")
	return nil
}
