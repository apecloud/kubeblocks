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
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// ComponentWorkloadTransformer handles component rsm workload generation
type ComponentWorkloadTransformer struct{}

var _ graph.Transformer = &ComponentWorkloadTransformer{}

func (t *ComponentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	// TODO: build or update rsm workload
	transCtx, _ := ctx.(*ComponentTransformContext)
	cli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	compOrig := transCtx.ComponentOrig

	if model.IsObjectDeleting(compOrig) {
		return nil
	}

	cluster := transCtx.Cluster
	synthesizeComp := transCtx.SynthesizeComponent

	// build rsm workload
	// TODO(xingran): BuildRSM relies on the deprecated fields of the component, for example component.WorkloadType, which should be removed in the future
	rsm, err := factory.BuildRSM(cluster, synthesizeComp)
	if err != nil {
		return err
	}
	objects := []client.Object{rsm}

	// read cache snapshot
	ml := constant.GetComponentWellKnownLabels(cluster.Name, comp.Name)
	oldSnapshot, err := model.ReadCacheSnapshot(ctx, comp, ml, ownedWorkloadKinds()...)
	if err != nil {
		return err
	}

	// compute create/update/delete set
	newSnapshot := make(map[model.GVKNObjKey]client.Object)
	for _, object := range objects {
		name, err := model.GetGVKName(object)
		if err != nil {
			return err
		}
		newSnapshot[*name] = object
	}

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newSnapshot)

	createSet := newNameSet.Difference(oldNameSet)
	// updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	createNewObjects := func() {
		for name := range createSet {
			cli.Create(dag, newSnapshot[name])
		}
	}
	updateObjects := func() {
		// for name := range updateSet {
		// oldObj := oldSnapshot[name]
		// newObj := copyAndMergeRSM(oldObj, newSnapshot[name])
		// scli.Update(dag, oldObj, newObj)
		// }
	}
	deleteOrphanObjects := func() {
		for name := range deleteSet {
			cli.Delete(dag, oldSnapshot[name])
		}
	}

	// objects to be created
	createNewObjects()
	// objects to be updated
	updateObjects()
	// objects to be deleted
	deleteOrphanObjects()

	return nil
}

func ownedWorkloadKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.ReplicatedStateMachineList{},
	}
}
