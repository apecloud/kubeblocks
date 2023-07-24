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

package components

import (
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type rsmComponent struct {
	rsmComponentBase
}

var _ Component = &rsmComponent{}

const workloadType = "RSM"

func newRSMComponent(cli client.Client,
	recorder record.EventRecorder,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	synthesizedComponent *component.SynthesizedComponent,
	dag *graph.DAG) *rsmComponent {
	comp := &rsmComponent{
		rsmComponentBase: rsmComponentBase{
			componentBase: componentBase{
				Client:         cli,
				Recorder:       recorder,
				Cluster:        cluster,
				ClusterVersion: clusterVersion,
				Component:      synthesizedComponent,
				ComponentSet: &RSM{
					componentSetBase: componentSetBase{
						Cli:                  cli,
						Cluster:              cluster,
						SynthesizedComponent: synthesizedComponent,
						ComponentSpec:        nil,
						ComponentDef:         nil,
					},
				},
				Dag:            dag,
				WorkloadVertex: nil,
			},
		},
	}
	return comp
}

func (c *rsmComponent) newBuilder(reqCtx intctrlutil.RequestCtx, cli client.Client,
	action *ictrltypes.LifecycleAction) componentWorkloadBuilder {
	builder := &rsmComponentWorkloadBuilder{
		componentWorkloadBuilderBase: componentWorkloadBuilderBase{
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

func (c *rsmComponent) GetWorkloadType() appsv1alpha1.WorkloadType {
	return workloadType
}

func (c *rsmComponent) GetBuiltObjects(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]client.Object, error) {
	return c.rsmComponentBase.GetBuiltObjects(c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *rsmComponent) Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.rsmComponentBase.Create(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionCreatePtr()))
}

func (c *rsmComponent) Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.rsmComponentBase.Update(reqCtx, cli, c.newBuilder(reqCtx, cli, nil))
}

func (c *rsmComponent) Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error {
	return c.rsmComponentBase.Status(reqCtx, cli, c.newBuilder(reqCtx, cli, ictrltypes.ActionNoopPtr()))
}
