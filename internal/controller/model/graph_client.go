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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type GraphWriter interface {
	// Root setups the given obj as root vertex of the underlying DAG.
	// this func should be called once before any others.
	Root(dag *graph.DAG, objOld, objNew client.Object)

	// Create saves the object obj in the underlying DAG.
	Create(dag *graph.DAG, obj client.Object)

	// Delete deletes the given obj from the underlying DAG.
	Delete(dag *graph.DAG, obj client.Object)

	// Update updates the given obj in the underlying DAG.
	Update(dag *graph.DAG, objOld, objNew client.Object)

	// Status updates the given obj's status in the underlying DAG.
	Status(dag *graph.DAG, objOld, objNew client.Object)

	// DependOn setups dependencies between 'object' and 'dependency',
	// which will guarantee the Write Order to the K8s cluster of these objects.
	DependOn(dag *graph.DAG, object client.Object, dependency ...client.Object)
}

type GraphClient interface {
	client.Reader
	GraphWriter
}

// TODO(free6om): make DAG a member of realGraphClient
type realGraphClient struct {
	client.Client
}

func (r *realGraphClient) Root(dag *graph.DAG, objOld, objNew client.Object) {
	vertex := &ObjectVertex{
		Obj:    objNew,
		OriObj: objOld,
		Action: ActionPtr(STATUS),
	}
	dag.AddVertex(vertex)
}

func (r *realGraphClient) Create(dag *graph.DAG, obj client.Object) {
	r.doWrite(dag, nil, obj, ActionPtr(CREATE))
}

func (r *realGraphClient) Update(dag *graph.DAG, objOld, objNew client.Object) {
	r.doWrite(dag, objOld, objNew, ActionPtr(UPDATE))
}

func (r *realGraphClient) Delete(dag *graph.DAG, obj client.Object) {
	r.doWrite(dag, nil, obj, ActionPtr(DELETE))
}

func (r *realGraphClient) Status(dag *graph.DAG, objOld, objNew client.Object) {
	r.doWrite(dag, objOld, objNew, ActionPtr(STATUS))
}

func (r *realGraphClient) DependOn(dag *graph.DAG, object client.Object, dependency ...client.Object) {
	objectVertex := r.findMatchedVertex(dag, object)
	if objectVertex == nil {
		return
	}
	for _, d := range dependency {
		v := r.findMatchedVertex(dag, d)
		if v != nil {
			dag.Connect(objectVertex, v)
		}
	}
}

func (r *realGraphClient) doWrite(dag *graph.DAG, objOld, objNew client.Object, action *Action) {
	vertex := r.findMatchedVertex(dag, objNew)
	switch {
	case vertex != nil:
		objVertex, _ := vertex.(*ObjectVertex)
		objVertex.Action = action
	default:
		vertex = &ObjectVertex{
			Obj:    objNew,
			OriObj: objOld,
			Action: action,
		}
		dag.AddConnectRoot(vertex)
	}
}

func (r *realGraphClient)findMatchedVertex(dag *graph.DAG, object client.Object) graph.Vertex {
	keyLookfor, err := GetGVKName(object)
	if err != nil {
		return nil
	}
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		key, err := GetGVKName(v.Obj)
		if err != nil {
			return nil
		}
		if *keyLookfor == *key {
			return vertex
		}
	}
	return nil
}

var _ GraphClient = &realGraphClient{}

func NewGraphClient(cli client.Client) GraphClient {
	return &realGraphClient{
		Client: cli,
	}
}
