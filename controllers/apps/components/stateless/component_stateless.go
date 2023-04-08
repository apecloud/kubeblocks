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
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func NewStatelessComponent(cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *statelessComponent {
	return &statelessComponent{
		ComponentBase: types.ComponentBase{
			Client:     cli,
			Definition: definition,
			Cluster:    cluster,
			CompDef:    compDef,
			CompVer:    compVer,
			CompSpec:   compSpec,
			Component:  nil,
			ComponentSet: &Stateless{
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

type statelessComponent struct {
	types.ComponentBase
}

func (c *statelessComponent) init(reqCtx intctrlutil.RequestCtx, cli client.Client, action *ictrltypes.LifecycleAction) error {
	if err := c.ComposeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}
	builder := &statelessComponentWorkloadBuilder{
		ComponentWorkloadBuilderBase: types.ComponentWorkloadBuilderBase{
			ReqCtx:        reqCtx,
			Client:        cli,
			Comp:          c,
			DefaultAction: action,
			Error:         nil,
			EnvConfig:     nil,
		},
		workload: nil,
	}
	builder.ConcreteBuilder = builder

	return builder.BuildEnv().
		BuildWorkload(0).
		BuildHeadlessService().
		BuildConfig(0).
		BuildTLSVolume(0).
		BuildVolumeMount(0).
		BuildService().
		BuildTLSCert().
		Complete()
}

func (c *statelessComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateless
}

func (c *statelessComponent) Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error) {
	if stsList, err := util.ListDeployOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels()); err != nil {
		return false, err
	} else {
		return len(stsList) > 0, nil // component.replica can not be zero
	}
}

func (c *statelessComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *statelessComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
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

func (c *statelessComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}

func (c *statelessComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	if err := c.ComposeSynthesizedComponent(reqCtx, cli); err != nil {
		return err
	}
	deploy, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		// TODO(refactor): fix me
		if strings.Contains(err.Error(), "no workload found for the component") {
			return nil
		}
		return err
	}
	return c.StatusImpl(reqCtx, cli, []client.Object{deploy})
}

func (c *statelessComponent) ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil
}

func (c *statelessComponent) HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deploy, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	if *deploy.Spec.Replicas != c.Component.Replicas {
		reqCtx.Recorder.Eventf(c.Cluster,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			c.GetName(), c.GetClusterName(), *deploy.Spec.Replicas, c.Component.Replicas)
	}
	return nil
}

func (c *statelessComponent) Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deploy, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}
	return util.RestartPod(&deploy.Spec.Template)
}

func (c *statelessComponent) Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return nil // TODO: impl
}

func (c *statelessComponent) runningWorkload(reqCtx intctrlutil.RequestCtx, cli client.Client) (*appsv1.Deployment, error) {
	deployList, err := util.ListDeployOwnedByComponent(reqCtx.Ctx, cli, c.GetNamespace(), c.GetMatchingLabels())
	if err != nil {
		return nil, err
	}

	cnt := len(deployList)
	if cnt == 0 {
		return nil, fmt.Errorf("no workload found for the component, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	} else if cnt > 1 {
		return nil, fmt.Errorf("more than one workloads found for the stateless component, cluster: %s, component: %s, cnt: %d",
			c.Cluster.Name, c.CompSpec.Name, cnt)
	}

	deploy := deployList[0]
	if deploy.Spec.Replicas == nil {
		return nil, fmt.Errorf("running workload for the stateless component has no replica, cluster: %s, component: %s",
			c.Cluster.Name, c.CompSpec.Name)
	}

	return deploy, nil
}

func (c *statelessComponent) updateUnderlyingResources(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	deployObj, err := c.runningWorkload(reqCtx, cli)
	if err != nil {
		return err
	}

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
		c.SetStatusPhase(appsv1alpha1.SpecReconcilingClusterCompPhase)
	}
}
