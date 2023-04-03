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
	batchv1 "k8s.io/api/batch/v1"
	"reflect"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

// TODO(refactor):
//  1. status management
//  2. component workload

type Component interface {
	GetName() string
	GetNamespace() string
	GetClusterName() string
	GetWorkloadType() appsv1alpha1.WorkloadType

	GetDefinition() *appsv1alpha1.ClusterDefinition
	GetVersion() *appsv1alpha1.ClusterVersion
	GetCluster() *appsv1alpha1.Cluster
	GetSynthesizedComponent() *component.SynthesizedComponent

	// GetPhase() appsv1alpha1.ClusterComponentPhase
	// GetStatus() appsv1alpha1.ClusterComponentStatus

	GetMatchingLabels() client.MatchingLabels

	// Exist checks whether the component exists in cluster, we say that a component exists iff the main workloads
	// exist in cluster, such as stateful set for consensus/replication/stateful and deployment for stateless.
	Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error)

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	// TODO(refactor): impl-related, will replace it with component workload
	addResource(obj client.Object, action *Action, parent *lifecycleVertex) *lifecycleVertex
	addWorkload(obj client.Object, action *Action, parent *lifecycleVertex)
}

func NewComponent(definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition is not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	}

	if compSpec == nil || compDef == nil {
		// TODO(refactor): fix me
		return nil, fmt.Errorf("NotSupported")
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return newReplicationComponent(definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Consensus:
		return newConsensusComponent(definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateful:
		return newStatefulComponent(definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateless:
		return newStatelessComponent(definition, cluster, compDef, compVer, compSpec, dag), nil
	}
	return nil, fmt.Errorf("unknown workload type: %s, cluster: %s, component: %s, component definition ref: %s",
		compDef.WorkloadType, cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
}

type componentBase struct {
	Definition *appsv1alpha1.ClusterDefinition
	Version    *appsv1alpha1.ClusterVersion
	Cluster    *appsv1alpha1.Cluster

	// TODO(refactor): should remove those members in future.
	CompDef  *appsv1alpha1.ClusterComponentDefinition
	CompVer  *appsv1alpha1.ClusterComponentVersion
	CompSpec *appsv1alpha1.ClusterComponentSpec

	// built synthesized component
	Component *component.SynthesizedComponent

	// DAG vertexes of main workload object(s)
	workloadVertexs []*lifecycleVertex

	dag *graph.DAG
}

func (c *componentBase) GetName() string {
	return c.CompSpec.Name
}

func (c *componentBase) GetNamespace() string {
	return c.Cluster.Namespace
}

func (c *componentBase) GetClusterName() string {
	return c.Cluster.Name
}

func (c *componentBase) GetDefinition() *appsv1alpha1.ClusterDefinition {
	return c.Definition
}

func (c *componentBase) GetVersion() *appsv1alpha1.ClusterVersion {
	return c.Version
}

func (c *componentBase) GetCluster() *appsv1alpha1.Cluster {
	return c.Cluster
}

func (c *componentBase) GetSynthesizedComponent() *component.SynthesizedComponent {
	return c.Component
}

// func (c *componentBase) GetPhase() appsv1alpha1.ClusterComponentPhase {
//	return c.GetStatus().Phase // TODO: impl
// }
//
// func (c *componentBase) GetStatus() appsv1alpha1.ClusterComponentStatus {
//	return appsv1alpha1.ClusterComponentStatus{} // TODO: impl
// }

func (c *componentBase) GetMatchingLabels() client.MatchingLabels {
	return client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
}

func (c *componentBase) addResource(obj client.Object, action *Action, parent *lifecycleVertex) *lifecycleVertex {
	if obj == nil {
		panic("try to add nil object")
	}
	vertex := &lifecycleVertex{
		obj:    obj,
		action: action,
	}
	c.dag.AddVertex(vertex)

	if parent != nil {
		c.dag.Connect(parent, vertex)
	}
	return vertex
}

func (c *componentBase) addWorkload(obj client.Object, action *Action, parent *lifecycleVertex) {
	c.workloadVertexs = append(c.workloadVertexs, c.addResource(obj, action, parent))
}

func (c *componentBase) createResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, actionPtr(CREATE), parent)
}

func (c *componentBase) deleteResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	vertex := c.addResource(obj, actionPtr(DELETE), parent)
	vertex.isOrphan = true
	return vertex
}

func (c *componentBase) updateResource(obj client.Object, parent *lifecycleVertex) *lifecycleVertex {
	return c.addResource(obj, actionPtr(UPDATE), parent)
}

// validateObjectsAction validates the action of all objects in dag has been determined
func (c *componentBase) validateObjectsAction() error {
	for _, v := range c.dag.Vertices() {
		if node, ok := v.(*lifecycleVertex); !ok {
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		} else if node.action == nil {
			return fmt.Errorf("unexpected nil vertex action, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		}
	}
	return nil
}

// resolveObjectsAction resolves the action of objects in dag to guarantee that all object actions will be determined
func (c *componentBase) resolveObjectsAction(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	snapshot, err := readCacheSnapshot(reqCtx, cli, c.GetCluster())
	if err != nil {
		return err
	}
	for _, v := range c.dag.Vertices() {
		if node, ok := v.(*lifecycleVertex); !ok {
			fmt.Printf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
			return fmt.Errorf("unexpected vertex type, cluster: %s, component: %s, vertex: %T",
				c.GetClusterName(), c.GetName(), v)
		} else if node.action == nil {
			if action, err := resolveObjectAction(snapshot, node); err != nil {
				return err
			} else {
				node.action = action
			}
		}
	}
	return c.validateObjectsAction()
}

func (c *componentBase) updateService(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    c.GetClusterName(),
		constant.KBAppComponentLabelKey: c.GetName(),
	}
	svcObjList, err := listObjWithLabelsInNamespace(reqCtx, cli, generics.ServiceSignature, c.GetNamespace(), labels)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	svcProtoList := findAll[*corev1.Service](c.dag)

	// create new services or update existed services
	for _, vertex := range svcProtoList {
		node, _ := vertex.(*lifecycleVertex)
		svcProto, _ := node.obj.(*corev1.Service)

		if pos := slices.IndexFunc(svcObjList, func(svc *corev1.Service) bool {
			return svc.GetName() == svcProto.GetName()
		}); pos < 0 {
			node.action = actionPtr(CREATE)
		} else {
			svcProto.Annotations = mergeServiceAnnotations(svcObjList[pos].Annotations, svcProto.Annotations)
			node.action = actionPtr(UPDATE)
		}
	}

	// delete useless services
	for _, svc := range svcObjList {
		if pos := slices.IndexFunc(svcProtoList, func(vertex graph.Vertex) bool {
			node, _ := vertex.(*lifecycleVertex)
			svcProto, _ := node.obj.(*corev1.Service)
			return svcProto.GetName() == svc.GetName()
		}); pos < 0 {
			c.deleteResource(svc, nil)
		}
	}
	return nil
}

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

func (c *statefulsetComponentBase) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

	c.updateWorkload(stsObj, 0)

	if err := c.updateService(reqCtx, cli); err != nil {
		return err
	}

	// TODO(refactor): to workaround that the scaled PVC will be deleted at object action
	if err := c.updatePVC(reqCtx, cli, stsObj); err != nil {
		return err
	}

	return nil
}

func (c *statefulsetComponentBase) updateWorkload(stsObj *appsv1.StatefulSet, idx int32) {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.workloadVertexs[idx].obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	mergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.workloadVertexs[idx].obj = stsObjCopy
		c.workloadVertexs[idx].action = actionPtr(UPDATE)
		// sync component phase
		//updateComponentPhaseWithOperation2(c.GetCluster(), c.GetName())
	}
}

func (c *statefulsetComponentBase) updatePVC(reqCtx intctrlutil.RequestCtx, cli client.Client, stsObj *appsv1.StatefulSet) error {
	pvcNameSet := sets.New[string]()
	for _, v := range findAll[*corev1.PersistentVolumeClaim](c.dag) {
		pvcNameSet.Insert(v.(*lifecycleVertex).obj.GetName())
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
			c.updateResource(pvc, c.workloadVertexs[0]).immutable = true
		}
	}
	return nil
}
