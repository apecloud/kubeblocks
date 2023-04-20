/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, "Create a new component")

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

	// TODO(impl): check the operation result of @Restart, @ExpandVolume, @HorizontalScale, and update component status if needed.
	//   @Restart - whether pods are available, covered by @BuildLatestStatus
	//   @ExpandVolume - whether PVCs have been expand finished
	//   @HorizontalScale - whether replicas to added or deleted have been done, and the cron job to delete PVCs have finished
	//  With these changes, we can remove the manipulation to cluster and component status phase in ops controller.

	if err := c.statusHorizontalScale(reqCtx, cli); err != nil {
		return err
	}

	// TODO(impl): restart pod if needed, move it to @Update and restart pod directly.
	if vertexes, err := c.ComponentSet.HandleRestart(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	if vertexes, err := c.ComponentSet.HandleRoleChange(reqCtx.Ctx, c.runningWorkload); err != nil {
		return err
	} else {
		for v := range vertexes {
			c.Dag.AddVertex(v)
		}
	}

	if err := c.BuildLatestStatus(reqCtx, cli, c.runningWorkload); err != nil {
		return err
	}

	if err := c.handleGarbageOfRestoreBeforeRunning(); err != nil {
		return err
	}
	c.updateWorkload(c.runningWorkload)
	return nil
}

func (c *StatefulComponentBase) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if c.runningWorkload == nil {
		return nil
	}
	return util.RestartPod(&c.runningWorkload.Spec.Template)
}

func (c *StatefulComponentBase) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if c.runningWorkload == nil {
		return nil
	}
	for _, vct := range c.runningWorkload.Spec.VolumeClaimTemplates {
		var vctProto *corev1.PersistentVolumeClaimSpec
		for _, v := range c.Component.VolumeClaimTemplates {
			if v.Name == vct.Name {
				vctProto = &v.Spec
				break
			}
		}

		// REVIEW: how could VCT proto is nil?
		if vctProto == nil {
			continue
		}

		// TODO(fix):
		//   1. check that can't decrease the storage size.
		//   2. since we can't update the storage size of stateful set, so we can't use it to determine the expansion.
		if vct.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Resources.Requests[corev1.ResourceStorage] {
			continue
		}

		for i := *c.runningWorkload.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: c.runningWorkload.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, c.runningWorkload.Name, i),
			}
			if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
				return err
			}
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Resources.Requests[corev1.ResourceStorage]
			c.UpdateResource(pvc, c.WorkloadVertex)
		}
	}
	return nil
}

func (c *StatefulComponentBase) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if c.runningWorkload == nil {
		return nil
	}
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

func (c *StatefulComponentBase) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// if scale in to 0, do not delete pvc
	if c.Component.Replicas == 0 || len(stsObj.Spec.VolumeClaimTemplates) == 0 {
		return nil // TODO: should reject to scale-in to zero explicitly
	}

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

func (c *StatefulComponentBase) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	key := client.ObjectKey{
		Namespace: stsObj.Namespace,
		Name:      stsObj.Name,
	}
	snapshotKey := types.NamespacedName{
		Namespace: stsObj.Namespace,
		Name:      stsObj.Name + "-scaling",
	}

	horizontalScalePolicy := c.Component.HorizontalScalePolicy

	cleanCronJobs := func() error {
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

	checkAllPVCsExist := func() (bool, error) {
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

	checkAllPVCBoundIfNeeded := func() (bool, error) {
		if horizontalScalePolicy == nil ||
			horizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
			!isSnapshotAvailable(cli, reqCtx.Ctx) {
			return true, nil
		}
		return isAllPVCBound(cli, reqCtx.Ctx, stsObj)
	}

	cleanBackupResourcesIfNeeded := func() error {
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
		stsProto := c.WorkloadVertex.Obj.(*appsv1.StatefulSet)
		objs, err := doBackup(reqCtx, cli, c.Cluster, c.Component, snapshotKey, stsProto, stsObj)
		if err != nil {
			return err
		}
		if objs != nil {
			for _, obj := range objs {
				c.CreateResource(obj, nil)
			}
			c.WorkloadVertex.Immutable = true
		}
		return nil
	}

	// check all pvc bound, requeue if not all ready
	allPVCBounded, err := checkAllPVCBoundIfNeeded()
	if err != nil {
		return err
	}
	if !allPVCBounded {
		c.WorkloadVertex.Immutable = true
		return nil
	}
	// clean backup resources.
	// there will not be any backup resources other than scale out.
	if err := cleanBackupResourcesIfNeeded(); err != nil {
		return err
	}

	// pvcs are ready, stateful_set.replicas should be updated
	c.WorkloadVertex.Immutable = false

	return nil
}

func (c *StatefulComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	if stsObj == nil {
		c.createWorkload()
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, "Component workload created")
	} else {
		if c.updateWorkload(stsObj) {
			c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, "Component workload updated")
		}
		// to work around that the scaled PVC will be deleted at object action.
		if err := c.updatePVC(reqCtx, cli, stsObj); err != nil {
			return err
		}
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
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.WorkloadVertex.Obj = stsObjCopy
		c.WorkloadVertex.Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		return true
	}
	return false
}

func (c *StatefulComponentBase) updatePVC(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, v := range ictrltypes.FindAll[*corev1.PersistentVolumeClaim](c.Dag) {
		pvcNameSet.Insert(v.(*ictrltypes.LifecycleVertex).Obj.GetName())
	}

	for _, vct := range c.Component.VolumeClaimTemplates {
		for i := c.Component.Replicas - 1; i >= 0; i-- {
			pvcName := fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i)
			if pvcNameSet.Has(pvcName) {
				continue
			}

			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      pvcName,
			}
			if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			c.NoopResource(pvc, c.WorkloadVertex)
		}
	}
	return nil
}

func (c *StatefulComponentBase) statusHorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	ret := c.horizontalScaling(c.runningWorkload)
	if ret == 0 {
		return c.statusPVCDeletionJobFailed(reqCtx, cli)
	}
	if ret > 0 {
		// forward the h-scaling progress.
		if err := c.scaleOut(reqCtx, cli, c.runningWorkload); err != nil {
			return err
		}
	}
	return nil
}

func (c *StatefulComponentBase) statusPVCDeletionJobFailed(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	hasJobFailed := func(reqCtx intctrlutil.RequestCtx, cli client.Client) (*batchv1.Job, string, error) {
		jobs, err := util.ListObjWithLabelsInNamespace(reqCtx.Ctx, cli, generics.JobSignature, c.GetNamespace(), c.GetMatchingLabels())
		if err != nil {
			return nil, "", err
		}
		for _, job := range jobs {
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
		c.setStatusPhaseWithMsg(appsv1alpha1.AbnormalClusterCompPhase, msgKey, msg, "PVC deletion job failed")
	}
	return nil
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
