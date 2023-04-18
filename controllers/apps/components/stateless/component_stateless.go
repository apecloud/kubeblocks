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

package stateless

import (
	"fmt"
	"reflect"
	"strings"

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
			Dag:             dag,
			WorkloadVertexs: make([]*ictrltypes.LifecycleVertex, 0),
		},
	}
	comp.ComponentSet.SetComponent(comp)
	return comp
}

type statelessComponent struct {
	internal.ComponentBase
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
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the component, cluster: %s, component: %s",
			c.GetClusterName(), c.GetName())
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the stateless component, cluster: %s, component: %s, cnt: %d",
			c.GetClusterName(), c.GetName(), cnt)
	}

	deploy := deployList[0]
	if deploy.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the stateless component has no replica, cluster: %s, component: %s",
			c.GetClusterName(), c.GetName())
	}

	return deploy, nil
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

	c.SetStatusPhase(appsv1alpha1.CreatingClusterCompPhase, "Create a new component")

	return nil
}

func (c *statelessComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, c.newBuilder(reqCtx, cli, nil), true); err != nil {
		return err
	}

	if err := c.Restart(reqCtx, cli); err != nil {
		return err
	}

	if err := c.Reconfigure(reqCtx, cli); err != nil {
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

func (c *statelessComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}

func (c *statelessComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.init(reqCtx, cli, nil, true); err != nil {
		// TODO(refactor): fix me
		if strings.Contains(err.Error(), "no workload found for the component") {
			return nil
		}
		return err
	}
	return c.ComponentBase.Status(reqCtx, cli, c.runningWorkload)
}

func (c *statelessComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *statelessComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if *c.runningWorkload.Spec.Replicas != c.Component.Replicas {
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
	c.updateWorkload(deployObj)

	if err := c.UpdateService(reqCtx, cli); err != nil {
		return err
	}

	return nil
}

func (c *statelessComponent) updateWorkload(deployObj *appsv1.Deployment) {
	deployObjCopy := deployObj.DeepCopy()
	deployProto := c.WorkloadVertexs[0].Obj.(*appsv1.Deployment)

	util.MergeAnnotations(deployObj.Spec.Template.Annotations, &deployProto.Spec.Template.Annotations)
	deployObjCopy.Spec = deployProto.Spec
	if !reflect.DeepEqual(&deployObj.Spec, &deployObjCopy.Spec) {
		c.WorkloadVertexs[0].Obj = deployObjCopy
		c.WorkloadVertexs[0].Action = ictrltypes.ActionUpdatePtr()
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase, "Component workload updated")
	}
}
