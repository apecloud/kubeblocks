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

package model

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

type GraphWriter interface {
	// Root setups the given obj as root vertex of the underlying DAG.
	Root(dag *graph.DAG, objOld, objNew client.Object, action *Action)

	// Create saves the object obj in the underlying DAG.
	Create(dag *graph.DAG, obj client.Object, opts ...GraphOption)

	// Delete deletes the given obj from the underlying DAG.
	Delete(dag *graph.DAG, obj client.Object, opts ...GraphOption)

	// Update updates the given obj in the underlying DAG.
	Update(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption)

	// Patch patches the given objOld by the new version objNew in the underlying DAG.
	Patch(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption)

	// Status updates the given obj's status in the underlying DAG.
	Status(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption)

	// Noop means not to commit any change made to this obj in the execute phase.
	Noop(dag *graph.DAG, obj client.Object, opts ...GraphOption)

	// Do does 'action' to 'objOld' and 'objNew' and return the vertex created.
	// this method creates a vertex directly even if the given object already exists in the underlying DAG.
	// WARN: this is a rather low-level API, will be refactored out in near future, avoid to use it.
	Do(dag *graph.DAG, objOld, objNew client.Object, action *Action, parent *ObjectVertex, opts ...GraphOption) *ObjectVertex

	// IsAction tells whether the action of the vertex of this obj is same as 'action'.
	IsAction(dag *graph.DAG, obj client.Object, action *Action) bool

	// DependOn setups dependencies between 'object' and 'dependencies',
	// which will guarantee the Write Order to the K8s cluster of these objects.
	// if multiple vertices exist(which can occur when ForceCreatingVertexOption being used), the one with the largest depth will be used.
	DependOn(dag *graph.DAG, object client.Object, dependencies ...client.Object)

	// FindAll finds all objects that have same type with obj in the underlying DAG.
	// obey the GraphOption if provided.
	FindAll(dag *graph.DAG, obj interface{}, opts ...GraphOption) []client.Object
}

type GraphClient interface {
	client.Reader
	GraphWriter
}

// TODO(free6om): make DAG a member of realGraphClient
type realGraphClient struct {
	client.Client
}

func (r *realGraphClient) Root(dag *graph.DAG, objOld, objNew client.Object, action *Action) {
	var root *ObjectVertex
	// find root vertex if already exists
	if len(dag.Vertices()) > 0 {
		if vertex := r.findMatchedVertex(dag, objNew); vertex != nil {
			root, _ = vertex.(*ObjectVertex)
		}
	}
	// create one if root vertex not found
	if root == nil {
		root = &ObjectVertex{}
		dag.AddVertex(root)
	}
	root.Obj, root.OriObj, root.Action = objNew, objOld, action
	// setup dependencies
	for _, vertex := range dag.Vertices() {
		if vertex != root {
			dag.Connect(root, vertex)
		}
	}
}

func (r *realGraphClient) Create(dag *graph.DAG, obj client.Object, opts ...GraphOption) {
	r.doWrite(dag, nil, obj, ActionCreatePtr(), opts...)
}

func (r *realGraphClient) Update(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption) {
	r.doWrite(dag, objOld, objNew, ActionUpdatePtr(), opts...)
}

func (r *realGraphClient) Patch(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption) {
	r.doWrite(dag, objOld, objNew, ActionPatchPtr(), opts...)
}

func (r *realGraphClient) Delete(dag *graph.DAG, obj client.Object, opts ...GraphOption) {
	r.doWrite(dag, nil, obj, ActionDeletePtr(), opts...)
}

func (r *realGraphClient) Status(dag *graph.DAG, objOld, objNew client.Object, opts ...GraphOption) {
	r.doWrite(dag, objOld, objNew, ActionStatusPtr(), opts...)
}

func (r *realGraphClient) Noop(dag *graph.DAG, obj client.Object, opts ...GraphOption) {
	r.doWrite(dag, nil, obj, ActionNoopPtr(), opts...)
}

func (r *realGraphClient) Do(dag *graph.DAG, objOld, objNew client.Object, action *Action, parent *ObjectVertex, opts ...GraphOption) *ObjectVertex {
	if dag.Root() == nil {
		panic(fmt.Sprintf("root vertex not found. obj: %T, name: %s", objNew, objNew.GetName()))
	}

	graphOpts := &GraphOptions{}
	for _, opt := range opts {
		opt.ApplyTo(graphOpts)
	}

	vertex := &ObjectVertex{
		OriObj:    objOld,
		Obj:       objNew,
		Action:    action,
		ClientOpt: graphOpts.clientOpt,
	}
	switch {
	case parent == nil:
		dag.AddConnectRoot(vertex)
	default:
		dag.AddConnect(parent, vertex)
	}
	return vertex
}

func (r *realGraphClient) IsAction(dag *graph.DAG, obj client.Object, action *Action) bool {
	vertex := r.findMatchedVertex(dag, obj)
	if vertex == nil {
		return false
	}
	v, _ := vertex.(*ObjectVertex)
	if action == nil {
		return v.Action == nil
	}
	if v.Action == nil {
		return false
	}
	return *v.Action == *action
}

func (r *realGraphClient) DependOn(dag *graph.DAG, object client.Object, dependency ...client.Object) {
	objectVertex := r.findMatchedVertex(dag, object)
	if objectVertex == nil {
		return
	}
	for _, d := range dependency {
		if d == nil {
			continue
		}
		v := r.findMatchedVertex(dag, d)
		if v != nil {
			dag.Connect(objectVertex, v)
		}
	}
}

func (r *realGraphClient) FindAll(dag *graph.DAG, obj interface{}, opts ...GraphOption) []client.Object {
	graphOpts := &GraphOptions{}
	for _, opt := range opts {
		opt.ApplyTo(graphOpts)
	}
	hasSameType := func() bool {
		return !graphOpts.haveDifferentTypeWith
	}()
	assignableTo := func(src, dst reflect.Type) bool {
		if dst == nil {
			return src == nil
		}
		return src.AssignableTo(dst)
	}
	objType := reflect.TypeOf(obj)
	objects := make([]client.Object, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		vertexType := reflect.TypeOf(v.Obj)
		if assignableTo(vertexType, objType) == hasSameType {
			objects = append(objects, v.Obj)
		}
	}
	return objects
}

func (r *realGraphClient) doWrite(dag *graph.DAG, objOld, objNew client.Object, action *Action, opts ...GraphOption) {
	graphOpts := &GraphOptions{}
	for _, opt := range opts {
		opt.ApplyTo(graphOpts)
	}

	vertex := r.findMatchedVertex(dag, objNew)
	switch {
	case vertex != nil:
		objVertex, _ := vertex.(*ObjectVertex)
		objVertex.Action = action
		if graphOpts.replaceIfExisting {
			objVertex.Obj = objNew
			objVertex.OriObj = objOld
		}
	default:
		vertex = &ObjectVertex{
			Obj:       objNew,
			OriObj:    objOld,
			Action:    action,
			ClientOpt: graphOpts.clientOpt,
		}
		dag.AddConnectRoot(vertex)
	}
}

func (r *realGraphClient) findMatchedVertex(dag *graph.DAG, object client.Object) graph.Vertex {
	keyLookFor, err := GetGVKName(object)
	if err != nil {
		panic(fmt.Sprintf("parse gvk name failed, obj: %T, name: %s, err: %v", object, object.GetName(), err))
	}
	var found graph.Vertex
	findVertex := func(v graph.Vertex) error {
		if found != nil {
			return nil
		}
		ov, _ := v.(*ObjectVertex)
		key, err := GetGVKName(ov.Obj)
		if err != nil {
			panic(fmt.Sprintf("parse gvk name failed, obj: %T, name: %s, err: %v", ov.Obj, ov.Obj.GetName(), err))
		}
		if *keyLookFor == *key {
			found = v
		}
		return nil
	}
	err = dag.WalkReverseTopoOrder(findVertex, nil)
	if err != nil {
		panic(fmt.Sprintf("walk DAG failed, err: %v", err))
	}
	return found
}

var _ GraphClient = &realGraphClient{}

func NewGraphClient(cli client.Client) GraphClient {
	return &realGraphClient{
		Client: cli,
	}
}
