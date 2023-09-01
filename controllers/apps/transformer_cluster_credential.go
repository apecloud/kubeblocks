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
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	component2 "github.com/apecloud/kubeblocks/pkg/controller/component"
	graph2 "github.com/apecloud/kubeblocks/pkg/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/pkg/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ClusterCredentialTransformer creates the connection credential secret
type ClusterCredentialTransformer struct{}

var _ graph2.Transformer = &ClusterCredentialTransformer{}

func (c *ClusterCredentialTransformer) Transform(ctx graph2.TransformContext, dag *graph2.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	var synthesizedComponent *component2.SynthesizedComponent
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
			synthesizedComponent = &component2.SynthesizedComponent{
				Name: comps[0].Name,
			}
		} else {
			synthesizedComponent, err = component2.BuildComponent(reqCtx, nil, cluster, transCtx.ClusterDef, &compDef, nil)
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
		secret, err := builder.BuildConnCredential(transCtx.ClusterDef, cluster, synthesizedComponent)
		if err != nil {
			return err
		}
		if secret != nil {
			ictrltypes.LifecycleObjectCreate(dag, secret, root)
		}
	}
	return nil
}
