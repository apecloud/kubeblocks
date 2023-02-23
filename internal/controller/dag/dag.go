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

package dag

import "errors"

type DAG struct {
	vertices map[Vertex]Vertex
	edges    map[Edge]Edge
}

type Vertex interface {}

type Edge interface {
	From() Vertex
	To() Vertex
}

type realEdge struct {
	F, T Vertex
}

type WalkFunc func(v Vertex) error

func (d *DAG) AddVertex(v Vertex) bool {
	if v == nil {
		return false
	}
	d.vertices[v] = v
	return true
}

func (d *DAG) RemoveVertex(v Vertex) bool {
	if v == 0 {
		return true
	}
	for k := range d.edges {
		if k.From() == v || k.To() == v {
			delete(d.edges, k)
		}
	}
	return true
}

func (d *DAG) AddEdge(e Edge) bool {
	if e.From() == nil || e.To() == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			return true
		}
	}
	d.edges[e]= e
	return true
}

func (d *DAG) RemoveEdge(e Edge) bool {
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			delete(d.edges, k)
		}
	}
	return true
}

func (d *DAG) Connect(from, to Vertex) bool {
	if from == nil || to == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == from && k.To() == to {
			return true
		}
	}
	edge :=RealEdge(from, to)
	d.edges[edge] = edge
	return true
}

func (d *DAG) WalkDepthFirst(walkFunc WalkFunc) error {
	root := d.root()
	if root == nil {
		return errors.New("can't find single root")
	}
	return d.dfs([]Vertex{root}, walkFunc)
}

func (d *DAG) root() Vertex {
	roots := make([]Vertex, 0)
	for n := range d.vertices {
		if d.hasNoInbound(n) {
			roots = append(roots, n)
		}
	}
	if len(roots) != 1 {
		return nil
	}
	return roots[0]
}

func (d *DAG) hasNoInbound(v Vertex) bool  {
	for e := range d.edges {
		if e.To() == v {
			return false
		}
	}
	return true
}

func (d *DAG) dfs(start []Vertex, walkFunc WalkFunc) error {
	for _, v := range start {
		adjacent := d.adjacent(v)
		if err := d.dfs(adjacent, walkFunc); err != nil {
			return err
		}
		if err := walkFunc(v); err != nil {
			return err
		}
	}
	return nil
}

func (d *DAG) adjacent(v Vertex) []Vertex {
	vertices := make([]Vertex, 0)
	for e := range d.edges {
		if e.From() == v {
			vertices = append(vertices, e.To())
		}
	}
	return vertices
}

func (r *realEdge) From() Vertex {
	return r.F
}

func (r *realEdge) To() Vertex {
	return r.T
}

func New() *DAG {
	dag := &DAG{
		vertices: make(map[Vertex]Vertex),
		edges:    make(map[Edge]Edge),
	}
	return dag
}

func RealEdge(from, to Vertex) Edge {
	return &realEdge{F: from, T: to}
}