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

package stateless

import (
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func NewStatelessComponent(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	synthesizedComponent *component.SynthesizedComponent,
	dag *graph.DAG) *statelessComponent {
	comp := &statelessComponent{
		ComponentBase: internal.ComponentBase{
			Client:         cli,
			Cluster:        cluster,
			ClusterVersion: clusterVersion,
			Component:      synthesizedComponent,
			ComponentSet: &Stateless{
				ComponentSetBase: types.ComponentSetBase{
					Cli:           cli,
					Cluster:       cluster,
					ComponentSpec: nil,
					ComponentDef:  nil,
					Component:     nil,
				},
			},
			Dag:            dag,
			WorkloadVertex: nil,
		},
	}
	comp.ComponentSet.SetComponent(comp)
	return comp
}

type statelessComponent struct {
	internal.ComponentBase
	// runningWorkload can be nil, and the replicas of workload can be nil (zero)
	runningWorkload *appsv1.Deployment
}

var _ types.Component = &statelessComponent{}

func (c *statelessComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *ictrltypes.LifecycleAction) internal.ComponentWorkloadBuilder {
	builder := &statelessComponentWorkloadBuilder{
		ComponentWorkloadBuilderBase: internal.ComponentWorkloadBuilderBase{
			ReqCtx:        reqCtx,
			Client:        cli,
			Comp:          c,
			DefaultAction: action,
			Error:         nil,
			EnvConfig:     nil,
			Workload:      nil,
		},
	}
	builder.ConcreteBuilder = builder
	return builder
}

func (c *statelessComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, builder internal.ComponentWorkloadBuilder, load bool) error {
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

func (c *statelessComponent) loadRunningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.Deployment, error) {
	deployList, err := util.ListDeployOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}
	cnt := len(deployList)
	if cnt == 1 {
		return deployList[0], nil
	}
	if cnt == 0 {
		return nil, nil
	} else {
		return nil, fmt.Errorf("more than one workloads found for the stateless component, cluster: %s, component: %s, cnt: %d",
			c.GetClusterName(), c.GetName(), cnt)
	}
}

func (c *statelessComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateless
}

func (c *statelessComponent) GetBuiltObjects(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]client.Object, error) {
	dag := c.Dag
	defer func() {
		c.Dag = dag
	}()

	c.Dag = graph.NewDAG()
	if err := c.init(intctrlutil.RequestCtx{}, nil, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()), false); err != nil {
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

func (c *statelessComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()), false); err != nil {
		return err
	}

	if err := c.ValidateObjectsAction(); err != nil {
		return err
	}

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, nil, "Create a new component")

	return nil
}

func (c *statelessComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(impl): delete component owned resources
	return nil
}

func (c *statelessComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, c.newBuilder(reqCtx, cli, nil), true); err != nil {
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

func (c *statelessComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionNoopPtr()), true); err != nil {
		return err
	}
	if c.runningWorkload == nil {
		return nil
	}
	return c.ComponentBase.StatusWorkload(reqCtx, cli, c.runningWorkload, nil)
}

func (c *statelessComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *statelessComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if c.runningWorkload.Spec.Replicas == nil && c.Component.Replicas > 0 {
		reqCtx.Recorder.Eventf(c.Cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			c.GetName(), c.GetClusterName(), 0, c.Component.Replicas)
	} else if c.runningWorkload.Spec.Replicas != nil && *c.runningWorkload.Spec.Replicas != c.Component.Replicas {
		reqCtx.Recorder.Eventf(c.Cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			c.GetName(), c.GetClusterName(), *c.runningWorkload.Spec.Replicas, c.Component.Replicas)
	}
	return nil
}

func (c *statelessComponent) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return util.RestartPod(&c.runningWorkload.Spec.Template)
}

func (c *statelessComponent) Reconfigure(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil // TODO(impl)
}

func (c *statelessComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client, deployObj *appsv1.Deployment) error {
	if deployObj == nil {
		c.createWorkload()
	} else {
		c.updateWorkload(deployObj)
	}

	if err := c.UpdateService(reqCtx, cli); err != nil {
		return err
	}

	return nil
}

func (c *statelessComponent) createWorkload() {
	deployProto := c.WorkloadVertex.Obj.(*appsv1.Deployment)
	c.WorkloadVertex.Obj = deployProto
	c.WorkloadVertex.Action = ictrltypes.ActionCreatePtr()
	c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, nil, "Component workload created")
}

func (c *statelessComponent) updateWorkload(deployObj *appsv1.Deployment) {
	deployObjCopy := deployObj.DeepCopy()
	deployProto := c.WorkloadVertex.Obj.(*appsv1.Deployment)

	util.MergeAnnotations(deployObj.Spec.Template.Annotations, &deployProto.Spec.Template.Annotations)
	deployObjCopy.Spec = deployProto.Spec
	if !reflect.DeepEqual(&deployObj.Spec, &deployObjCopy.Spec) {
		c.WorkloadVertex.Obj = deployObjCopy
		c.WorkloadVertex.Action = ictrltypes.ActionUpdatePtr()
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, nil, "Component workload updated")
	}
}
