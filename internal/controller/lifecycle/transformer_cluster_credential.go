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

package lifecycle

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
)

// ClusterCredentialTransformer creates the connection credential secret
type ClusterCredentialTransformer struct{}

var _ graph.Transformer = &ClusterCredentialTransformer{}

func (c *ClusterCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster
	if cluster.IsDeleting() {
		return nil
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	var synthesizedComponent *component.SynthesizedComponent
	compSpecMap := cluster.Spec.GetDefNameMappingComponents()
	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.Service == nil {
			continue
		}
		comps := compSpecMap[compDef.Name]
		if len(comps) > 0 {
			synthesizedComponent = &component.SynthesizedComponent{
				Name: comps[0].Name,
				Services: []corev1.Service{
					{Spec: compDef.Service.ToSVCSpec()},
				},
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
