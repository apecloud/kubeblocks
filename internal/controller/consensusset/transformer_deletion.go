/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consensusset

import (
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// CSSetDeletionTransformer handles ConsensusSet deletion
type CSSetDeletionTransformer struct{}

func (t *CSSetDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	obj := transCtx.CSSet
	if !model.IsObjectDeleting(obj) {
		return nil
	}

	// list all objects owned by this primary obj in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	snapshot, err := model.ReadCacheSnapshot(transCtx, obj, ownedKinds()...)
	if err != nil {
		return err
	}
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		vertex := &model.ObjectVertex{Obj: object, Action: model.ActionPtr(model.DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}
	root.Action = model.ActionPtr(model.DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrFastReturn
}

var _ graph.Transformer = &CSSetDeletionTransformer{}
