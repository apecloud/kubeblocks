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
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// clusterValidationTransformer validates the cluster spec.
type clusterValidationTransformer struct{}

var _ graph.Transformer = &clusterValidationTransformer{}

func (t *clusterValidationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	if err = validateComponentDefNComponentDefRef(cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
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
