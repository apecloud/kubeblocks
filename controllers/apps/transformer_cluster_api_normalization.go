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
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ClusterAPINormalizationTransformer handles cluster and component API conversion.
type ClusterAPINormalizationTransformer struct{}

var _ graph.Transformer = &ClusterAPINormalizationTransformer{}

func (t *ClusterAPINormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	// build all component specs
	transCtx.ComponentSpecs = make([]*appsv1alpha1.ClusterComponentSpec, 0)
	cluster := transCtx.Cluster

	// validate componentDef and componentDefRef
	if err := validateComponentDefNComponentDefRef(cluster); err != nil {
		return err
	}

	for i := range cluster.Spec.ComponentSpecs {
		clusterComSpec := cluster.Spec.ComponentSpecs[i]
		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, &clusterComSpec)
	}
	if compSpec := apiconversion.HandleSimplifiedClusterAPI(transCtx.ClusterDef, cluster); compSpec != nil {
		transCtx.ComponentSpecs = append(transCtx.ComponentSpecs, compSpec)
	}

	// build all component definitions referenced
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}
	for i, compSpec := range transCtx.ComponentSpecs {
		if len(compSpec.ComponentDef) == 0 {
			compDef, err := component.BuildComponentDefinition(transCtx.ClusterDef, transCtx.ClusterVer, compSpec)
			if err != nil {
				return err
			}
			transCtx.ComponentDefs[compSpec.ComponentDefRef] = compDef
			transCtx.ComponentSpecs[i].ComponentDef = compSpec.ComponentDefRef
		} else {
			// should be loaded at load resources transformer
			if _, ok := transCtx.ComponentDefs[compSpec.ComponentDef]; !ok {
				panic(fmt.Sprintf("runtime error - expected component definition object not found: %s", compSpec.ComponentDef))
			}
		}
	}
	return nil
}

func validateComponentDefNComponentDefRef(cluster *appsv1alpha1.Cluster) error {
	if len(cluster.Spec.ComponentSpecs) == 0 {
		return nil
	}
	compDefRefMap := make(map[string]bool)
	compDefMap := make(map[string]bool)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if len(compSpec.ComponentDefRef) == 0 && len(compSpec.ComponentDef) == 0 {
			return fmt.Errorf("componentDef and componentDefRef cannot be both empty")
		}
		if compSpec.ComponentDefRef == compSpec.ComponentDef {
			return fmt.Errorf("componentDef and componentDefRef cannot be the same in ComponentSpec: %s", compSpec.Name)
		}
		if len(compSpec.ComponentDef) != 0 {
			if _, ok := compDefRefMap[compSpec.ComponentDef]; ok {
				return fmt.Errorf("componentDef and componentDefRef cannot be the same in different ComponentSpecs")
			}
			compDefMap[compSpec.ComponentDef] = true
		}
		if len(compSpec.ComponentDefRef) != 0 {
			if _, ok := compDefMap[compSpec.ComponentDefRef]; ok {
				return fmt.Errorf("componentDef and componentDefRef cannot be the same in different ComponentSpecs")
			}
			compDefRefMap[compSpec.ComponentDefRef] = true
		}
	}
	return nil
}
