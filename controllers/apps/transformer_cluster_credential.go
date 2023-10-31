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

package apps

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ClusterCredentialTransformer creates the connection credential secret
type ClusterCredentialTransformer struct{}

var _ graph.Transformer = &ClusterCredentialTransformer{}

func (c *ClusterCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	var (
		synthesizedComponent *component.SynthesizedComponent
		err                  error
	)
	compSpecMap := cluster.Spec.GetDefNameMappingComponents()
	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.Service == nil {
			continue
		}
		reqCtx := intctrlutil.RequestCtx{
			Ctx: transCtx.Context,
			Log: log.Log.WithName("cluster"),
		}
		comps := compSpecMap[compDef.Name]
		if len(comps) > 0 {
			synthesizedComponent = &component.SynthesizedComponent{
				Name: comps[0].Name,
			}
		} else {
			synthesizedComponent, err = component.BuildComponent(reqCtx, nil, cluster, transCtx.ClusterDef, &compDef, nil, nil)
			if err != nil {
				return err
			}
		}
		if synthesizedComponent != nil {
			synthesizedComponent.Services = []corev1.Service{
				{Spec: compDef.Service.ToSVCSpec()},
			}
			break
		}
	}
	if synthesizedComponent != nil {
		secret := factory.BuildConnCredential(transCtx.ClusterDef, cluster, synthesizedComponent)
		if secret != nil {
			graphCli.Create(dag, secret)
		}
	}
	return nil
}
