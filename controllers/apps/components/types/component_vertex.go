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

package types

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type ComponentVertex struct {
	Obj       client.Object
	Copy      client.Object
	Immutable bool
	Action    func(context.Context, client.Client) error
}

func (v *ComponentVertex) String() string {
	return ""
}

func (v *ComponentVertex) patch(ctx context.Context, cli client.Client) error {
	return nil
}

func (v *ComponentVertex) update(ctx context.Context, cli client.Client) error {
	return nil
}

func (v *ComponentVertex) status(ctx context.Context, cli client.Client) error {
	patch := client.MergeFrom(v.Copy)
	if err := cli.Status().Patch(ctx, v.Obj, patch); err != nil {
		return err
	}
	return nil
}

func (v *ComponentVertex) delete(ctx context.Context, cli client.Client) error {
	err := cli.Delete(ctx, v.Obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (v *ComponentVertex) noop(_ context.Context, _ client.Client) error {
	return nil
}

func ExecuteComponentVertex(vertex graph.Vertex) error {
	v, ok := vertex.(*ComponentVertex)
	if !ok {
		return fmt.Errorf("unknown vertex type: %T", vertex)
	}
	if v.Action == nil {
		return fmt.Errorf("unexpected empty vertex action: %s", v)
	}
	return v.Action(nil, nil)
}

func AddVertex4Patch(dag *graph.DAG, obj client.Object, copy client.Object) *ComponentVertex {
	v := &ComponentVertex{
		Obj:  obj,
		Copy: copy,
	}
	v.Action = v.patch
	dag.AddVertex(v)
	return v
}

func AddVertex4Update(dag *graph.DAG, obj client.Object) *ComponentVertex {
	v := &ComponentVertex{
		Obj: obj,
	}
	v.Action = v.update
	dag.AddVertex(v)
	return v
}

func AddVertex4Delete(dag *graph.DAG, obj client.Object) *ComponentVertex {
	v := &ComponentVertex{
		Obj: obj,
	}
	v.Action = v.delete
	dag.AddVertex(v)
	return v
}

func AddVertex4Status(dag *graph.DAG, obj client.Object, copy client.Object) *ComponentVertex {
	v := &ComponentVertex{
		Obj:  obj,
		Copy: copy,
	}
	v.Action = v.status
	dag.AddVertex(v)
	return v
}

func AddVertex4Noop(dag *graph.DAG, obj client.Object) *ComponentVertex {
	v := &ComponentVertex{
		Obj: obj,
	}
	v.Action = v.noop
	dag.AddVertex(v)
	return v
}
