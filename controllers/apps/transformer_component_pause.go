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
	"fmt"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

// componentDeletionTransformer handles component deletion
type componentPauseTransformer struct {
}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentPauseTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	// if paused
	if checkPaused(comp) {
		// get instanceSet and set paused
		oldInstanceSet, err := t.getInstanceSet(transCtx, comp)
		newInstanceSet := oldInstanceSet.DeepCopy()
		if err != nil {
			return err
		}
		newInstanceSet.Spec.Paused = true
		// update in dag
		graphCli.Update(dag, oldInstanceSet, newInstanceSet)
		return graph.ErrPrematureStop
	} else {
		// get instanceSet and cancel paused
		oldInstanceSet, err := t.getInstanceSet(transCtx, comp)
		if model.IsReconciliationPaused(oldInstanceSet) {
			newInstanceSet := oldInstanceSet.DeepCopy()
			if err != nil {
				return err
			}
			newInstanceSet.Spec.Paused = false
			// update in dag
			graphCli.Update(dag, oldInstanceSet, newInstanceSet)
			return nil
		}
		return nil
	}
}

func (t *componentPauseTransformer) getInstanceSet(transCtx *componentTransformContext, comp *appsv1alpha1.Component) (*workloads.InstanceSet, error) {
	instanceName := comp.Name
	instanceSet := &workloads.InstanceSet{}
	err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: instanceName, Namespace: comp.Namespace}, instanceSet)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get instanceSet %s: %v", instanceName, err))
	}
	return instanceSet, nil
}
