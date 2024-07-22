/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentPauseTransformer handles component pause and resume
type componentPauseTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentPauseTransformer{}

func (t *componentPauseTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	instanceSet := getInstanceSet(transCtx)
	configuration := getConfiguration(transCtx)
	configMapList := listConfigMaps(transCtx)

	if model.IsReconciliationPaused(comp) {
		if !model.IsReconciliationPaused(instanceSet) {
			instanceSet.Spec.Paused = true
			graphCli.Update(dag, nil, instanceSet)
		}
		if configuration, needUpdate := SetPauseAnnotation(configuration); needUpdate {
			graphCli.Update(dag, nil, configuration)
		}
		for i := range configMapList.Items {
			if configMap, needUpdate := SetPauseAnnotation(&configMapList.Items[i]); needUpdate {
				graphCli.Update(dag, nil, configMap)
			}
		}
		// pause reconciliation now
		return graph.ErrPrematureStop
	}
	allResumed := true
	if model.IsReconciliationPaused(instanceSet) {
		instanceSet.Spec.Paused = false
		graphCli.Update(dag, nil, instanceSet)
		allResumed = false
	}
	if configuration, needUpdate := RemovePauseAnnotation(configuration); needUpdate {
		graphCli.Update(dag, nil, configuration)
		allResumed = false
	}
	for i := range configMapList.Items {
		if configMap, needUpdate := RemovePauseAnnotation(&configMapList.Items[i]); needUpdate {
			graphCli.Update(dag, nil, configMap)
			allResumed = false
		}
	}
	if !allResumed {
		// Wait for the next reconciliation to actually update the resources in k8s before reconciling the component.
		return graph.ErrPrematureStop
	}
	return nil

}
