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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type LifecycleAction string

const (
	CREATE = LifecycleAction("CREATE")
	DELETE = LifecycleAction("DELETE")
	UPDATE = LifecycleAction("UPDATE")
	PATCH  = LifecycleAction("PATCH")
	STATUS = LifecycleAction("STATUS")
	NOOP   = LifecycleAction("NOOP")
)

// LifecycleVertex describes expected object spec and how to reach it
// obj always represents the expected part: new object in Create/Update action and old object in Delete action
// oriObj is set in Update action
// all transformers doing their object manipulation works on obj.spec
// the root vertex(i.e. the cluster vertex) will be treated specially:
// as all its meta, spec and status can be updated in one reconciliation loop
// Update is ignored when immutable=true
// orphan object will be force deleted when action is DELETE
type LifecycleVertex struct {
	Obj       client.Object
	ObjCopy   client.Object
	Immutable bool
	Orphan    bool
	Action    *LifecycleAction
}

func (v LifecycleVertex) String() string {
	if v.Action == nil {
		return fmt.Sprintf("{obj:%T, name: %s, immutable: %v, orphan: %v, action: nil}",
			v.Obj, v.Obj.GetName(), v.Immutable, v.Orphan)
	}
	return fmt.Sprintf("{obj:%T, name: %s, immutable: %v, orphan: %v, action: %v}",
		v.Obj, v.Obj.GetName(), v.Immutable, v.Orphan, *v.Action)
}

func ActionPtr(action LifecycleAction) *LifecycleAction {
	return &action
}

func ActionCreatePtr() *LifecycleAction {
	return ActionPtr(CREATE)
}

func ActionUpdatePtr() *LifecycleAction {
	return ActionPtr(UPDATE)
}

func ActionDeletePtr() *LifecycleAction {
	return ActionPtr(DELETE)
}

func ActionStatusPtr() *LifecycleAction {
	return ActionPtr(STATUS)
}

func ActionPatchPtr() *LifecycleAction {
	return ActionPtr(PATCH)
}

func ActionNoopPtr() *LifecycleAction {
	return ActionPtr(NOOP)
}

func LifecycleObjectCreate(dag *graph.DAG, obj client.Object, parent *LifecycleVertex) *LifecycleVertex {
	return addObject(dag, obj, ActionCreatePtr(), parent)
}

func LifecycleObjectDelete(dag *graph.DAG, obj client.Object, parent *LifecycleVertex) *LifecycleVertex {
	vertex := addObject(dag, obj, ActionDeletePtr(), parent)
	vertex.Orphan = true
	return vertex
}

func LifecycleObjectUpdate(dag *graph.DAG, obj client.Object, parent *LifecycleVertex) *LifecycleVertex {
	return addObject(dag, obj, ActionUpdatePtr(), parent)
}

func LifecycleObjectPatch(dag *graph.DAG, obj, objCopy client.Object, parent *LifecycleVertex) *LifecycleVertex {
	vertex := addObject(dag, obj, ActionPatchPtr(), parent)
	vertex.ObjCopy = objCopy
	return vertex
}

func LifecycleObjectNoop(dag *graph.DAG, obj client.Object, parent *LifecycleVertex) *LifecycleVertex {
	return addObject(dag, obj, ActionNoopPtr(), parent)
}

func addObject(dag *graph.DAG, obj client.Object, action *LifecycleAction, parent *LifecycleVertex) *LifecycleVertex {
	if obj == nil {
		panic("try to add nil object")
	}
	vertex := &LifecycleVertex{
		Obj:    obj,
		Action: action,
	}
	dag.AddVertex(vertex)

	if parent != nil {
		dag.Connect(parent, vertex)
	}
	return vertex
}

func FindMatchedVertex[T interface{}](dag *graph.DAG, objectKey client.ObjectKey) graph.Vertex {
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*LifecycleVertex)
		if _, ok := v.Obj.(T); ok {
			if client.ObjectKeyFromObject(v.Obj) == objectKey {
				return vertex
			}
		}
	}
	return nil
}

func FindAll[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*LifecycleVertex)
		if _, ok := v.Obj.(T); ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func FindAllNot[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*LifecycleVertex)
		if _, ok := v.Obj.(T); !ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func FindRootVertex(dag *graph.DAG) (*LifecycleVertex, error) {
	root := dag.Root()
	if root == nil {
		return nil, fmt.Errorf("root vertex not found: %v", dag)
	}
	rootVertex, _ := root.(*LifecycleVertex)
	return rootVertex, nil
}
