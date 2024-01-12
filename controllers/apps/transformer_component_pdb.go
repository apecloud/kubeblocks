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
	"reflect"

	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentPDBTransformer handles the component PDB
type componentPDBTransformer struct{}

var _ graph.Transformer = &componentPDBTransformer{}

func (t *componentPDBTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create pdb related objects", "component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	synthesizedComp := transCtx.SynthesizeComponent
	obj, err := t.runningPDBObject(ctx, synthesizedComp)
	if err != nil {
		return err
	}

	// build PDB for backward compatibility
	// MinAvailable is used to determine whether to create a PDB (Pod Disruption Budget) object. However, the functionality of PDB should be implemented within the RSM.
	// Therefore, PDB objects are no longer needed in the new API, and the MinAvailable field should be deprecated.
	// The old MinAvailable field, which value is determined based on the deprecated "workloadType" field, is also no longer applicable in the new API.
	// TODO(xingran): which should be removed when workloadType and ClusterCompDefName are removed
	var pdb *policyv1.PodDisruptionBudget
	if synthesizedComp.MinAvailable != nil {
		pdb = factory.BuildPDB(synthesizedComp)
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if obj == nil {
		if pdb == nil {
			// do nothing
		} else {
			graphCli.Create(dag, pdb)
		}
	} else {
		if pdb == nil {
			graphCli.Delete(dag, obj)
		} else {
			t.handleUpdate(graphCli, dag, obj, pdb)
		}
	}
	return nil
}

func (t *componentPDBTransformer) runningPDBObject(ctx graph.TransformContext, synthesizedComp *component.SynthesizedComponent) (*policyv1.PodDisruptionBudget, error) {
	pdbs := &policyv1.PodDisruptionBudgetList{}
	inNS := client.InNamespace(synthesizedComp.Namespace)
	ml := client.MatchingLabels(constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name))
	if err := ctx.GetClient().List(ctx.GetContext(), pdbs, inNS, ml); err != nil {
		return nil, err
	}
	if len(pdbs.Items) == 0 {
		return nil, nil
	}
	return &pdbs.Items[0], nil
}

func (t *componentPDBTransformer) handleUpdate(cli model.GraphClient, dag *graph.DAG, obj, pdb *policyv1.PodDisruptionBudget) {
	objCopy := obj.DeepCopy()
	if pdb.Annotations != nil {
		for k, v := range objCopy.Annotations {
			if _, ok := pdb.Annotations[k]; !ok {
				pdb.Annotations[k] = v
			}
		}
		objCopy.Annotations = pdb.Annotations
	}
	objCopy.Spec = pdb.Spec
	if !reflect.DeepEqual(obj, objCopy) {
		cli.Update(dag, obj, objCopy)
	}
}
