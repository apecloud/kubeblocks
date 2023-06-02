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

func ActionDeletePtr() *LifecycleAction {
	return ActionPtr(DELETE)
}

func ActionUpdatePtr() *LifecycleAction {
	return ActionPtr(UPDATE)
}

func ActionPatchPtr() *LifecycleAction {
	return ActionPtr(PATCH)
}

func ActionStatusPtr() *LifecycleAction {
	return ActionPtr(STATUS)
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

func LifecycleObjectPatch(dag *graph.DAG, obj client.Object, objCopy client.Object, parent *LifecycleVertex) *LifecycleVertex {
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
