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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	ictrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ClusterServiceReferenceTransformer handles the cluster service reference
type ClusterServiceReferenceTransformer struct {
	client.Client
}

var _ graph.Transformer = &ClusterServiceReferenceTransformer{}

func (t *ClusterServiceReferenceTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster

	clusterDef := transCtx.ClusterDef
	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	root, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	compSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	compDefMap := make(map[string]*appsv1alpha1.ClusterComponentDefinition)
	for _, spec := range cluster.Spec.ComponentSpecs {
		compSpecMap[spec.Name] = &spec
	}
	for _, compDef := range clusterDef.Spec.ComponentDefs {
		compDefMap[compDef.Name] = &compDef
	}

	for _, compDef := range clusterDef.Spec.ComponentDefs {
		for _, compSpec := range cluster.Spec.ComponentSpecs {
			if compDef.Name != compSpec.ComponentDefRef {
				continue
			}
			serviceRefsMap, err := plan.GenServiceReferences(reqCtx, t.Client, cluster, clusterDef, &compDef, &compSpec)
			if err != nil {
				return err
			}
			for _, serviceRef := range compSpec.ServiceRefs {
				if serviceRef.ConnectionCredential != "" {
					continue
				}
				// if service reference is another KubeBlocks Cluster, we need to create a service connection credential with the default connection credential of the referenced cluster
				serviceConnectionCredential := serviceRefsMap[serviceRef.Name]
				if serviceConnectionCredential != nil {
					ictrltypes.LifecycleObjectCreate(dag, serviceConnectionCredential, root)
				}
			}
		}
	}

	return nil
}
