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

package consensus1

import (
	"github.com/apecloud/kubeblocks/controllers/apps/components/consensus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateful"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func NewConsensusComponent(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	synthesizedComponent *component.SynthesizedComponent,
	dag *graph.DAG) *consensusComponent {
	comp := &consensusComponent{
		StatefulComponentBase: internal.StatefulComponentBase{
			ComponentBase: internal.ComponentBase{
				Client:         cli,
				Cluster:        cluster,
				ClusterVersion: clusterVersion,
				Component:      synthesizedComponent,
				ComponentSet: &consensus.ConsensusSet{
					Stateful: stateful.Stateful{
						ComponentSetBase: types.ComponentSetBase{
							Cli:           cli,
							Cluster:       cluster,
							ComponentSpec: nil,
							ComponentDef:  nil,
							Component:     nil,
						},
					},
				},
				Dag:             dag,
				WorkloadVertexs: make([]*ictrltypes.LifecycleVertex, 0),
			},
		},
	}
	comp.ComponentSet.SetComponent(comp)
	return comp
}

type consensusComponent struct {
	internal.StatefulComponentBase
}

var _ types.Component = &consensusComponent{}

func (c *consensusComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *ictrltypes.LifecycleAction) internal.ComponentWorkloadBuilder {
	builder := &consensusComponentWorkloadBuilder{
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

func (c *consensusComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Consensus
}

func (c *consensusComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.StatefulComponentBase.Create(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *consensusComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.StatefulComponentBase.Update(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}
