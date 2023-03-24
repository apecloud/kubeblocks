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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type consensusComponent struct {
	componentBase
}

type consensusComponentBuilder struct {
	componentBuilderBase
	workload *appsv1.StatefulSet
}

func (b *consensusComponentBuilder) mutableWorkload(_ int32) client.Object {
	return b.workload
}

func (b *consensusComponentBuilder) mutablePodSpec(_ int32) *corev1.PodSpec {
	return &b.workload.Spec.Template.Spec
}

func (b *consensusComponentBuilder) buildService() componentBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0, len(svcList))
		leader := b.Comp.GetSynthesizedComponent().ConsensusSpec.Leader
		for _, svc := range svcList {
			if len(leader.Name) > 0 {
				svc.Spec.Selector[constant.RoleLabelKey] = leader.Name
			}
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.buildWrapper(buildfn)
}

func (b *consensusComponentBuilder) buildWorkload(_ int32) componentBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		component := b.Comp.GetSynthesizedComponent()
		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), component, b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.workload = sts

		// build PDB object
		if component.MaxUnavailable != nil {
			pdb, err := builder.BuildPDBLow(b.Comp.GetCluster(), component)
			if err != nil {
				return nil, err
			}
			return []client.Object{pdb}, err // don't return sts here, and it will not add to resource queue now
		}
		return nil, nil
	}
	return b.buildWrapper(buildfn)
}

func (c *consensusComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *Action) error {
	synthesizedComp, err := component.BuildSynthesizedComponent(reqCtx, cli, *c.Cluster, *c.Definition, *c.CompDef, *c.CompSpec, c.CompVer)
	if err != nil {
		return err
	}
	c.Component = synthesizedComp

	builder := &consensusComponentBuilder{
		componentBuilderBase: componentBuilderBase{
			ReqCtx:        reqCtx,
			Client:        cli,
			Comp:          c,
			defaultAction: action,
			Error:         nil,
			EnvConfig:     nil,
		},
		workload: nil,
	}
	builder.concreteBuilder = builder

	// runtime, config, script, env, volume, service, monitor, probe
	return builder.buildEnv(). // TODO: workload related, scaling related
					buildWorkload(0). // build workload here since other objects depend on it.
					buildHeadlessService().
					buildConfig(0).
					buildTLSVolume(0).
					buildVolumeMount(0).
					buildService().
					buildTLSCert().
					complete()
}

func (c *consensusComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Consensus
}

func (c *consensusComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster.Namespace, c.Cluster.Name, c.Component.Name); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *consensusComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, actionPtr(CREATE)); err != nil {
		return err
	}
	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is already exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}
	return nil
}

func (c *consensusComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component owned resources
	return nil
}

func (c *consensusComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, nil); err != nil {
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

	return c.updateUnderlayResources(reqCtx, cli)
}

func (c *consensusComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

		// TODO:
		//   1. check that cann't decrease the storage size.
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

func (c *consensusComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	ret, err := c.horizontalScaling(reqCtx, cli)
	if err != nil {
		return err
	} else if ret == 0 {
		return nil
	}

	if ret < 0 {
		if err := c.scaleIn(reqCtx, cli); err != nil {
			return err
		}
	} else {
		if err := c.scaleOut(reqCtx, cli); err != nil {
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

func (c *consensusComponent) runningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.StatefulSet, error) {
	stsList, err := listStsOwnedByComponent(reqCtx, cli, c.Cluster.Namespace, c.Cluster.Name, c.Component.Name)
	if err != nil {
		return nil, err
	}

	cnt := len(stsList)
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the consensus component, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the consensus component, cluster: %s, component: %s, cnt: %d",
			c.Cluster.Name, c.CompSpec.Name, cnt)
	}

	sts := stsList[0]
	if sts.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the consensus component has no replica, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	return sts, nil
}

// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *consensusComponent) horizontalScaling(reqCtx intctrlutil.RequestCtx, cli client.Client) (int, error) {
	sts, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return 0, err
	}
	return int(c.Component.Replicas - *sts.Spec.Replicas), nil
}

func (c *consensusComponent) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

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

func (c *consensusComponent) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

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
		stsProto := c.workloadVertexs[0].obj.(*appsv1.StatefulSet)
		objs, err := doBackup(reqCtx, cli, c.Cluster, c.Component, snapshotKey, stsProto, stsObj)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			c.createResource(obj, nil)
		}
		c.workloadVertexs[0].immutable = true
		return nil
	}

	// check all pvc bound, requeue if not all ready
	allPVCBounded, err := checkAllPVCBoundIfNeeded()
	if err != nil {
		return err
	}
	if !allPVCBounded {
		c.workloadVertexs[0].immutable = true
		return nil
	}
	// clean backup resources.
	// there will not be any backup resources other than scale out.
	if err := cleanBackupResourcesIfNeeded(); err != nil {
		return err
	}

	// pvcs are ready, stateful_set.replicas should be updated
	c.workloadVertexs[0].immutable = false

	return nil
}

func (c *consensusComponent) updateUnderlayResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.updateWorkload(reqCtx, cli); err != nil {
		return err
	}
	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}
	return nil
}

func (c *consensusComponent) updateWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	return c.updateStatefulSetWorkload(stsObj, 0)
}
