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

package lifecycle

import (
	"fmt"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// As a base class for single stateful-set based component (stateful & consensus)
type statefulsetComponentBase struct {
	componentBase
}

func (c *statefulsetComponentBase) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listStsOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels()); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *statefulsetComponentBase) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.composeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}

	return builder.buildEnv().
		buildWorkload(0).
		buildHeadlessService().
		buildConfig(0).
		buildTLSVolume(0).
		buildVolumeMount(0).
		buildService().
		buildTLSCert().
		complete()
}

func (c *statefulsetComponentBase) create(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder); err != nil {
		return err
	}

	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is already exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	if err := c.validateObjectsAction(); err != nil {
		return err
	}

	c.setStatusPhase(appsv1alpha1.CreatingClusterCompPhase)

	return nil
}

func (c *statefulsetComponentBase) update(reqCtx intctrlutil.RequestCtx, cli client.Client, builder componentWorkloadBuilder) error {
	if err := c.init(reqCtx, cli, builder); err != nil {
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

	if err := c.updateUnderlyingResources(reqCtx, cli); err != nil {
		return err
	}

	return c.resolveObjectsAction(reqCtx, cli)
}

func (c *statefulsetComponentBase) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}

func (c *statefulsetComponentBase) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.composeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}
	sts, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		// TODO(refactor): fix me
		if strings.Contains(err.Error(), "no workload found for the component") {
			return nil
		}
		return err
	}
	if err = c.status(reqCtx, cli, []client.Object{sts}); err != nil {
		return err
	}
	return c.handleGarbageOfRestoreBeforeRunning()
}

func (c *statefulsetComponentBase) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

	for _, vct := range stsObj.Spec.VolumeClaimTemplates {
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

		// TODO(refactor):
		//   1. check that can't decrease the storage size.
		//   2. since we can't update the storage size of stateful set, so we can't use it to determine the expansion.
		if vct.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Resources.Requests[corev1.ResourceStorage] {
			continue
		}

		for i := *stsObj.Spec.Replicas - 1; i >= 0; i-- {
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			if err := cli.Get(reqCtx.Ctx, pvcKey, pvc); err != nil {
				return err
			}
			pvc.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Resources.Requests[corev1.ResourceStorage]
			c.updateResource(pvc, c.workloadVertexs[0])
		}
	}
	return nil
}

func (c *statefulsetComponentBase) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	sts, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

	ret := c.horizontalScaling(sts)
	if ret == 0 {
		return nil
	} else if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, sts); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli, sts); err != nil {
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

func (c *statefulsetComponentBase) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	sts, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	return restartPod(&sts.Spec.Template)
}

func (c *statefulsetComponentBase) Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil // TODO(refactor): impl
}

func (c *statefulsetComponentBase) runningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.StatefulSet, error) {
	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}

	cnt := len(stsList)
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the component, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the component, cluster: %s, component: %s, cnt: %d",
			c.Cluster.Name, c.CompSpec.Name, cnt)
	}

	sts := stsList[0]
	if sts.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the component has no replica, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	return sts, nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *statefulsetComponentBase) horizontalScaling(sts *appsv1.StatefulSet) int {
	return int(c.Component.Replicas - *sts.Spec.Replicas)
}

func (c *statefulsetComponentBase) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// if scale in to 0, do not delete pvc
	// TODO: why check the volume claims of current stateful set?
	if c.CompSpec.Replicas == 0 || len(stsObj.Spec.VolumeClaimTemplates) == 0 {
		return nil // TODO: should reject to scale-in to zero explicitly
	}

	for i := c.CompSpec.Replicas; i < *stsObj.Spec.Replicas; i++ {
		for _, vct := range stsObj.Spec.VolumeClaimTemplates {
			pvcKey := types.NamespacedName{
				Namespace: stsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
			}
			// create cronjob to delete pvc after 30 minutes
			if obj, err := checkedCreateDeletePVCCronJob(reqCtx, cli, pvcKey, stsObj, c.Cluster); err != nil {
				return err
			} else if obj != nil {
				c.createResource(obj, nil)
			}
		}
	}
	return nil
}

func (c *statefulsetComponentBase) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
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
		for i := *stsObj.Spec.Replicas; i < c.CompSpec.Replicas; i++ {
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
				c.deleteResource(cronJob, c.workloadVertexs[0])
			}
		}
		return nil
	}

	checkAllPVCsExist := func() (bool, error) {
		for i := *stsObj.Spec.Replicas; i < c.CompSpec.Replicas; i++ {
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
		objs, err := deleteSnapshot(cli, reqCtx, snapshotKey, c.Cluster, c.CompSpec.Name)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.deleteResource(obj, nil)
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
		// do backup according to component's horizontal scale policy
		stsProto := c.workloadVertexs[0].Obj.(*appsv1.StatefulSet)
		objs, err := doBackup(reqCtx, cli, c.Cluster, c.Component, snapshotKey, stsProto, stsObj)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.createResource(obj, nil)
		}
		c.workloadVertexs[0].Immutable = true
		return nil
	}

	// check all pvc bound, requeue if not all ready
	allPVCBounded, err := checkAllPVCBoundIfNeeded()
	if err != nil {
		return err
	}
	if !allPVCBounded {
		c.workloadVertexs[0].Immutable = true
		return nil
	}
	// clean backup resources.
	// there will not be any backup resources other than scale out.
	if err := cleanBackupResourcesIfNeeded(); err != nil {
		return err
	}

	// pvcs are ready, stateful_set.replicas should be updated
	c.workloadVertexs[0].Immutable = false

	return nil
}

func (c *statefulsetComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

	c.updateWorkload(stsObj, 0)

	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}

	// to work around that the scaled PVC will be deleted at object action.
	if err := c.updatePVC(reqCtx, cli, stsObj); err != nil {
		return err
	}

	return nil
}

func (c *statefulsetComponentBase) updateWorkload(stsObj *appsv1.StatefulSet, idx int32) {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.workloadVertexs[idx].Obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	mergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.workloadVertexs[idx].Obj = stsObjCopy
		c.workloadVertexs[idx].Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		c.setStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase)
	}
}

func (c *statefulsetComponentBase) updatePVC(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	// PVCs which have been added to the dag because of volume expansion.
	pvcNameSet := sets.New[string]()
	for _, v := range findAll[*corev1.PersistentVolumeClaim](c.dag) {
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
			c.noopResource(pvc, c.workloadVertexs[0])
		}
	}
	return nil
}
