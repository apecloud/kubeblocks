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

package stateful

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func NewStatefulComponent(cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	cluster *appsv1alpha1.Cluster,
	compDef *appsv1alpha1.ClusterComponentDefinition,
	compVer *appsv1alpha1.ClusterComponentVersion,
	compSpec *appsv1alpha1.ClusterComponentSpec,
	dag *graph.DAG) *statefulComponent {
	return &statefulComponent{
		StatefulsetComponentBase: types.StatefulsetComponentBase{
			ComponentBase: types.ComponentBase{
				Client:     cli,
				Definition: definition,
				Cluster:    cluster,
				CompDef:    compDef,
				CompVer:    compVer,
				CompSpec:   compSpec,
				Component:  nil,
				ComponentSet: &Stateful{
					Cli:          cli,
					Cluster:      cluster,
					Component:    compSpec,
					ComponentDef: compDef,
				},
				Dag:             dag,
				WorkloadVertexs: make([]*ictrltypes.LifecycleVertex, 0),
			},
		},
	}
}

type statefulComponent struct {
	types.StatefulsetComponentBase
}

func (c *statefulComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *ictrltypes.LifecycleAction) types.ComponentWorkloadBuilder {
	builder := &statefulComponentWorkloadBuilder{
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
	return builder
}

func (c *statefulComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Stateful
}

func (c *statefulComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.CreateImpl(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *statefulComponent) Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	// TODO(refactor): delete component owned resources
	return nil
}

func (c *statefulComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.UpdateImpl(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}
