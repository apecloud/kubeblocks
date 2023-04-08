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

package replication

import (
	"fmt"
	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func NewReplicationComponent(cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *replicationComponent {
	return &replicationComponent{
		ComponentBase: types.ComponentBase{
			Client:     cli,
			Definition: definition,
			Cluster:    cluster,
			CompDef:    compDef,
			CompVer:    compVer,
			CompSpec:   compSpec,
			Component:  nil,
			ComponentSet: &replicationset.ReplicationSet{
				Cli:          cli,
				Cluster:      cluster,
				Component:    compSpec,
				ComponentDef: compDef,
			},
			Dag:             dag,
			WorkloadVertexs: make([]*ictrltypes.LifecycleVertex, 0),
		},
	}
}

type replicationComponent struct {
	types.ComponentBase
}

func (c *replicationComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *ictrltypes.LifecycleAction) error {
	if err := c.ComposeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}

	builder := &replicationComponentWorkloadBuilder{
		ComponentWorkloadBuilderBase: types.ComponentWorkloadBuilderBase{
			ReqCtx:        reqCtx,
			Client:        cli,
			Comp:          c,
			DefaultAction: action,
			Error:         nil,
			EnvConfig:     nil,
		},
		workloads: make([]*appsv1.StatefulSet, 0),
	}
	builder.ConcreteBuilder = builder

	// env and headless service are component level resources
	builder.BuildEnv().BuildHeadlessService()
	for i := int32(0); i < c.Component.Replicas; i++ {
		builder.BuildWorkload(i).
			BuildVolume(i).
			BuildConfig(i).
			BuildTLSVolume(i).
			BuildVolumeMount(i)
		if builder.Error != nil {
			return builder.Error
		}
	}
	return builder.BuildService().BuildTLSCert().Complete()
}

func (c *replicationComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (c *replicationComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := util.ListStsOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels()); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *replicationComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, ictrltypes.ActionCreatePtr()); err != nil {
		return err
	}

	if exist, err := c.Exist(reqCtx, cli); err != nil || exist {
		if err != nil {
			return err
		}
		return fmt.Errorf("component to be created is already exist, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	if err := c.ValidateObjectsAction(); err != nil {
		return err
	}

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase)

	return nil
}

func (c *replicationComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, nil); err != nil {
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

	return c.ResolveObjectsAction(reqCtx, cli)
}

func (c *replicationComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}

func (c *replicationComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.ComposeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}
	workloads, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		// TODO(refactor): fix me
		if strings.Contains(err.Error(), "no workload found for the component") {
			return nil
		}
		return err
	}
	objs := make([]client.Object, 0)
	for _, w := range workloads {
		objs = append(objs, w)
	}
	if err = c.StatusImpl(reqCtx, cli, objs); err != nil {
		return err
	}
	return c.HandleGarbageOfRestoreBeforeRunning()
}

func (c *replicationComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	workloads, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for _, sts := range workloads {
		for _, vct := range c.Component.VolumeClaimTemplates {
			pvc := &corev1.PersistentVolumeClaim{}
			key := client.ObjectKey{
				Namespace: sts.GetNamespace(),
				Name:      replicationset.GetPersistentVolumeClaimName(sts, &vct, 0),
			}
			if err = cli.Get(reqCtx.Ctx, key, pvc); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if apierrors.IsNotFound(err) {
				continue // new added pvc?
			}

			if pvc.Spec.Resources.Requests[corev1.ResourceStorage] == vct.Spec.Resources.Requests[corev1.ResourceStorage] {
				continue
			}

			if vertex := ictrltypes.FindMatchedVertex[*corev1.PersistentVolumeClaim](c.Dag, key); vertex == nil {
				return fmt.Errorf("cann't find PVC object when to update it, cluster: %s, component: %s, pvc: %s",
					c.Cluster.Name, c.Component.Name, key)
			} else {
				vertex.(*ictrltypes.LifecycleVertex).Action = ictrltypes.ActionUpdatePtr()
			}
		}
	}
	return nil
}

func (c *replicationComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsList, err := util.ListStsOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return err
	}

	ret := c.horizontalScaling(stsList)
	if ret == 0 {
		return nil
	} else if ret < 0 {
		if err := c.scaleIn(reqCtx, cli, stsList); err != nil {
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

func (c *replicationComponent) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsList, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for _, sts := range stsList {
		if err := util.RestartPod(&sts.Spec.Template); err != nil {
			return err
		}
	}
	return nil
}

func (c *replicationComponent) Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil // TODO(refactor): impl
}

func (c *replicationComponent) runningWorkloads(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]*appsv1.StatefulSet, error) {
	stsList, err := util.ListStsOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}
	if len(stsList) == 0 {
		return nil, fmt.Errorf("no workload found for the component, cluster: %s, component: %s",
			c.Cluster.Name, c.Component.Name)
	}
	return stsList, nil
}

// TODO: fix stale cache problem
// TODO: if sts created in last reconcile-loop not present in cache, hasReplicationSetHScaling return false positive
// < 0 for scale in, > 0 for scale out, and == 0 for nothing
func (c *replicationComponent) horizontalScaling(stsList []*appsv1.StatefulSet) int {
	// TODO(refactor): should use a more stable status
	return int(c.Component.Replicas) - len(stsList)
}

func (c *replicationComponent) scaleIn(reqCtx intctrlutil.RequestCtx, cli client.Client, stsList []*appsv1.StatefulSet) error {
	stsToDelete, err := replicationset.HandleComponentHorizontalScaleIn(reqCtx.Ctx, cli, c.Cluster, c.GetSynthesizedComponent(), stsList)
	if err != nil {
		return err
	}
	for _, sts := range stsToDelete {
		c.DeleteResource(sts, nil)
	}

	return nil
}

func (c *replicationComponent) scaleOut(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *replicationComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	stsObjList, err := c.runningWorkloads(reqCtx, cli)
	if err != nil {
		return err
	}

	for i, stsObj := range stsObjList {
		c.updateWorkload(stsObj, int32(i))
	}

	if err := c.UpdateService(reqCtx, cli); err != nil {
		return err
	}

	return nil
}

func (c *replicationComponent) updateWorkload(stsObj *appsv1.StatefulSet, idx int32) {
	stsObjCopy := stsObj.DeepCopy()
	stsProto := c.WorkloadVertexs[idx].Obj.(*appsv1.StatefulSet)

	// keep the original template annotations.
	// if annotations exist and are replaced, the statefulSet will be updated.
	util.MergeAnnotations(stsObjCopy.Spec.Template.Annotations, &stsProto.Spec.Template.Annotations)
	stsObjCopy.Spec.Template = stsProto.Spec.Template
	stsObjCopy.Spec.Replicas = stsProto.Spec.Replicas
	stsObjCopy.Spec.UpdateStrategy = stsProto.Spec.UpdateStrategy
	if !reflect.DeepEqual(&stsObj.Spec, &stsObjCopy.Spec) {
		c.WorkloadVertexs[idx].Obj = stsObjCopy
		c.WorkloadVertexs[idx].Action = ictrltypes.ActionPtr(ictrltypes.UPDATE)
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase)
	}
}
