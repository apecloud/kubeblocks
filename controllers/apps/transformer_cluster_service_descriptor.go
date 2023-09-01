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

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	ictrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// ClusterServiceDescriptorTransformer creates a service descriptor for the cluster.
type ClusterServiceDescriptorTransformer struct {
	client.Client
}

var _ graph.Transformer = &ClusterServiceDescriptorTransformer{}

func (t *ClusterServiceDescriptorTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
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

	// TODO: check every service reference declaration in the clusterDefinition has a mapping service reference binding in the cluster.spec.componentSpecs[*].serviceRefs
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
				// if ServiceDescriptor is not empty, it means that the service reference is from external service, and we do not need to create a service descriptor for it
				if serviceRef.ServiceDescriptor != "" {
					continue
				}
				// if service reference is another KubeBlocks Cluster, we need to create a service descriptor object for the referenced cluster
				serviceDescriptor := serviceRefsMap[serviceRef.Name]
				if serviceDescriptor != nil {
					ictrltypes.LifecycleObjectCreate(dag, serviceDescriptor, root)
				}
			}
		}
	}

	return nil
}
