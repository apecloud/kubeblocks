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

package replication

import (
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

func NewReplicationComponent(cli client.Client,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	synthesizedComponent *component.SynthesizedComponent,
	dag *graph.DAG) *replicationComponent {
	comp := &replicationComponent{
		StatefulComponentBase: internal.StatefulComponentBase{
			ComponentBase: internal.ComponentBase{
				Client:         cli,
				Cluster:        cluster,
				ClusterVersion: clusterVersion,
				Component:      synthesizedComponent,
				ComponentSet: &ReplicationSet{
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
				Dag:            dag,
				WorkloadVertex: nil,
			},
		},
	}
	comp.ComponentSet.SetComponent(comp)
	return comp
}

type replicationComponent struct {
	internal.StatefulComponentBase
}

var _ types.Component = &replicationComponent{}

func (c *replicationComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *ictrltypes.LifecycleAction) internal.ComponentWorkloadBuilder {
	builder := &replicationComponentWorkloadBuilder{
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

func (c *replicationComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return appsv1alpha1.Replication
}

func (c *replicationComponent) GetBuiltObjects(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]client.Object, error) {
	return c.StatefulComponentBase.GetBuiltObjects(c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *replicationComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.StatefulComponentBase.Create(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *replicationComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.StatefulComponentBase.Update(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}

func (c *replicationComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.StatefulComponentBase.Status(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionNoopPtr()))
}
