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

package component

import (
	"fmt"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewStatelessComponent(definition appsv1alpha1.ClusterDefinition,
	version appsv1alpha1.ClusterVersion,
	cluster appsv1alpha1.Cluster,
	compSpec appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *statelessComponent {
	return &statelessComponent{
		componentBase: componentBase{
			Definition: definition,
			Cluster:    cluster,
			CompDef:    *(&definition).GetComponentDefByName(compSpec.ComponentDefRef),
			CompVer:    version.GetDefNameMappingComponents()[compSpec.ComponentDefRef],
			CompSpec:   compSpec,
			Dag:        dag,
		},
	}
}

type statelessComponent struct {
	componentBase
}

func (c *statelessComponent) GetName() string {
	return c.CompSpec.Name
}

func (c *statelessComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateless
}

func (c *statelessComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is aready exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	synthesizedComp := BuildComponent(reqCtx, c.Cluster, c.Definition, c.CompDef, c.CompSpec, c.CompVer)

	if err := buildRestoreInfoFromBackup(reqCtx, cli, c.Cluster, synthesizedComp); err != nil {
		return err
	}

	task := intctrltypes.ReconcileTask{
		Cluster:   &c.Cluster,
		Component: synthesizedComp,
		Resources: c.Resources,
	}
	return plan.PrepareStatelessComponentResources(reqCtx, cli, &task)
}

func (c *statelessComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO: delete component managed resources
	return nil
}

func (c *statelessComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *statelessComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := listDeployOwnedByComponent(reqCtx, cli, c.Cluster, c.CompSpec); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *statelessComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

			if vct.Spec.Resources.Requests[corev1.ResourceStorage] == vctProto.Spec.Resources.Requests[corev1.ResourceStorage] {
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
				obj := pvc.DeepCopy()
				obj.Spec.Resources.Requests[corev1.ResourceStorage] = vctProto.Spec.Resources.Requests[corev1.ResourceStorage]
				v := &lifecycleVertex{
					obj:    obj,
					oriObj: pvc,
					action: actionPtr(UPDATE),
				}
				dag.AddVertex(v)
				dag.Connect(vertex, v)
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

func (c *statelessComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	handleHorizontalScaling := func(vertex *lifecycleVertex) error {
		stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
		stsProto, _ := vertex.obj.(*appsv1.StatefulSet)
		if *stsObj.Spec.Replicas == *stsProto.Spec.Replicas {
			return nil
		}

		key := client.ObjectKey{
			Namespace: stsProto.GetNamespace(),
			Name:      stsProto.GetName(),
		}
		snapshotKey := types.NamespacedName{
			Namespace: stsObj.Namespace,
			Name:      stsObj.Name + "-scaling",
		}
		// find component of current statefulset
		componentName := stsObj.Labels[constant.KBAppComponentLabelKey]
		components := mergeComponentsList(s.ctx,
			*cluster,
			s.cr.cd,
			s.cr.cd.Spec.ComponentDefs,
			cluster.Spec.ComponentSpecs)
		comp := getComponent(components, componentName)
		if comp == nil {
			s.ctx.Recorder.Eventf(cluster,
				corev1.EventTypeWarning,
				"HorizontalScaleFailed",
				"component %s not found",
				componentName)
			return nil
		}
		cleanCronJobs := func() error {
			for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// delete deletion cronjob if exists
					cronJobKey := pvcKey
					cronJobKey.Name = "delete-pvc-" + pvcKey.Name
					cronJob := &batchv1.CronJob{}
					if err := s.cli.Get(s.ctx.Ctx, cronJobKey, cronJob); err != nil {
						return client.IgnoreNotFound(err)
					}
					v := &lifecycleVertex{
						obj:    cronJob,
						action: actionPtr(DELETE),
					}
					dag.AddVertex(v)
					dag.Connect(vertex, v)
				}
			}
			return nil
		}

		checkAllPVCsExist := func() (bool, error) {
			for i := *stsObj.Spec.Replicas; i < *stsProto.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// check pvc existence
					pvcExists, err := isPVCExists(s.cli, s.ctx.Ctx, pvcKey)
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
			if comp.HorizontalScalePolicy == nil ||
				comp.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(s.cli, s.ctx.Ctx) {
				return true, nil
			}
			return isAllPVCBound(s.cli, s.ctx.Ctx, stsObj)
		}

		cleanBackupResourcesIfNeeded := func() error {
			if comp.HorizontalScalePolicy == nil ||
				comp.HorizontalScalePolicy.Type != appsv1alpha1.HScaleDataClonePolicyFromSnapshot ||
				!isSnapshotAvailable(s.cli, s.ctx.Ctx) {
				return nil
			}
			// if all pvc bounded, clean backup resources
			return deleteSnapshot(s.cli, s.ctx, snapshotKey, cluster, comp, dag, rootVertex)
		}

		scaleOut := func() error {
			if err := cleanCronJobs(); err != nil {
				return err
			}
			allPVCsExist, err := checkAllPVCsExist()
			if err != nil {
				return err
			}
			if !allPVCsExist {
				// do backup according to component's horizontal scale policy
				if err := doBackup(s.ctx, s.cli, comp, snapshotKey, dag, rootVertex, vertex); err != nil {
					return err
				}
				vertex.immutable = true
				return nil
			}
			// check all pvc bound, requeue if not all ready
			allPVCBounded, err := checkAllPVCBoundIfNeeded()
			if err != nil {
				return err
			}
			if !allPVCBounded {
				vertex.immutable = true
				return nil
			}
			// clean backup resources.
			// there will not be any backup resources other than scale out.
			if err := cleanBackupResourcesIfNeeded(); err != nil {
				return err
			}

			// pvcs are ready, stateful_set.replicas should be updated
			vertex.immutable = false

			return nil

		}

		scaleIn := func() error {
			// scale in, if scale in to 0, do not delete pvc
			if *stsProto.Spec.Replicas == 0 || len(stsObj.Spec.VolumeClaimTemplates) == 0 {
				return nil
			}
			for i := *stsProto.Spec.Replicas; i < *stsObj.Spec.Replicas; i++ {
				for _, vct := range stsObj.Spec.VolumeClaimTemplates {
					pvcKey := types.NamespacedName{
						Namespace: key.Namespace,
						Name:      fmt.Sprintf("%s-%s-%d", vct.Name, stsObj.Name, i),
					}
					// create cronjob to delete pvc after 30 minutes
					if err := checkedCreateDeletePVCCronJob(s.cli, s.ctx, pvcKey, stsObj, cluster, dag, rootVertex); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// when horizontal scaling up, sometimes db needs backup to sync data from master,
		// log is not reliable enough since it can be recycled
		var err error
		switch {
		// scale out
		case *stsObj.Spec.Replicas < *stsProto.Spec.Replicas:
			err = scaleOut()
		case *stsObj.Spec.Replicas > *stsProto.Spec.Replicas:
			err = scaleIn()
		}
		if err != nil {
			return err
		}

		if *stsObj.Spec.Replicas != *stsProto.Spec.Replicas {
			s.ctx.Recorder.Eventf(cluster,
				corev1.EventTypeNormal,
				"HorizontalScale",
				"Start horizontal scale component %s from %d to %d",
				comp.Name,
				*stsObj.Spec.Replicas,
				*stsProto.Spec.Replicas)
		}

		return nil
	}

	vertices := findAll[*appsv1.StatefulSet](dag)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if v.obj == nil || v.oriObj == nil {
			continue
		}
		if err := handleHorizontalScaling(v); err != nil {
			return err
		}
	}
	return nil
}
