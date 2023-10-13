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

package graph

import (
	"errors"
	"fmt"
	"sort"
)

type DAG struct {
	vertices map[Vertex]Vertex
	edges    map[Edge]Edge
}

type Vertex interface{}

type Edge interface {
	From() Vertex
	To() Vertex
}

type realEdge struct {
	F, T Vertex
}

// WalkFunc defines the action should be taken when we walk through the DAG.
// the func is vertex basis
type WalkFunc func(v Vertex) error

var _ Edge = &realEdge{}

func (r *realEdge) From() Vertex {
	return r.F
}

func (r *realEdge) To() Vertex {
	return r.T
}

// AddVertex puts 'v' into 'd'
func (d *DAG) AddVertex(v Vertex) bool {
	if v == nil {
		return false
	}
	d.vertices[v] = v
	return true
}

// RemoveVertex deletes 'v' from 'd'
// the in&out edges are also deleted
func (d *DAG) RemoveVertex(v Vertex) bool {
	if v == nil {
		return true
	}
	for k := range d.edges {
		if k.From() == v || k.To() == v {
			delete(d.edges, k)
		}
	}
	delete(d.vertices, v)
	return true
}

// Vertices returns all vertices in 'd'
func (d *DAG) Vertices() []Vertex {
	vertices := make([]Vertex, 0)
	for v := range d.vertices {
		vertices = append(vertices, v)
	}
	return vertices
}

// AddEdge puts edge 'e' into 'd'
func (d *DAG) AddEdge(e Edge) bool {
	if e.From() == nil || e.To() == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			return true
		}
	}
	d.edges[e] = e
	return true
}

// RemoveEdge deletes edge 'e'
func (d *DAG) RemoveEdge(e Edge) bool {
	for k := range d.edges {
		if k.From() == e.From() && k.To() == e.To() {
			delete(d.edges, k)
		}
	}
	return true
}

// Connect vertex 'from' to 'to' by a new edge if not exist
func (d *DAG) Connect(from, to Vertex) bool {
	if from == nil || to == nil {
		return false
	}
	for k := range d.edges {
		if k.From() == from && k.To() == to {
			return true
		}
	}
	edge := RealEdge(from, to)
	d.edges[edge] = edge
	return true
}

// AddConnect add 'to' to the DAG 'd' and connect 'from' to 'to'
func (d *DAG) AddConnect(from, to Vertex) bool {
	if !d.AddVertex(to) {
		return false
	}
	return d.Connect(from, to)
}

// AddConnectRoot add 'v' to the DAG 'd' and connect root to 'v'
func (d *DAG) AddConnectRoot(v Vertex) bool {
	root := d.Root()
	if root == nil {
		return false
	}
	return d.AddConnect(root, v)
}

// WalkTopoOrder walks the DAG 'd' in topology order
func (d *DAG) WalkTopoOrder(walkFunc WalkFunc, less func(v1, v2 Vertex) bool) error {
	if err := d.validate(); err != nil {
		return err
	}
	orders := d.topologicalOrder(false, less)
	for _, v := range orders {
		if err := walkFunc(v); err != nil {
			return err
		}
	}
	return nil
}

// WalkReverseTopoOrder walks the DAG 'd' in reverse topology order
func (d *DAG) WalkReverseTopoOrder(walkFunc WalkFunc, less func(v1, v2 Vertex) bool) error {
	if err := d.validate(); err != nil {
		return err
	}
	orders := d.topologicalOrder(true, less)
	for _, v := range orders {
		if err := walkFunc(v); err != nil {
			return err
		}
	}
	return nil
}

// WalkBFS walks the DAG 'd' in breadth-first order
func (d *DAG) WalkBFS(walkFunc WalkFunc) error {
	return d.bfs(walkFunc, nil)
}

func (d *DAG) bfs(walkFunc WalkFunc, less func(v1, v2 Vertex) bool) error {
	if err := d.validate(); err != nil {
		return err
	}
	queue := make([]Vertex, 0)
	walked := make(map[Vertex]bool, len(d.Vertices()))

	root := d.Root()
	queue = append(queue, root)
	for len(queue) > 0 {
		var walkErr error
		for _, vertex := range queue {
			if err := walkFunc(vertex); err != nil {
				walkErr = err
			}
		}
		if walkErr != nil {
			return walkErr
		}

		nextStep := make([]Vertex, 0)
		for _, vertex := range queue {
			adjs := d.outAdj(vertex)
			if less != nil {
				sort.SliceStable(adjs, func(i, j int) bool {
					return less(adjs[i], adjs[j])
				})
			}
			for _, adj := range adjs {
				if !walked[adj] {
					nextStep = append(nextStep, adj)
					walked[adj] = true
				}
			}
		}
		queue = nextStep
	}

	return nil
}

// Equals tells whether two DAGs are equal
// `less` tells whether vertex 'v1' is less than vertex 'v2'.
// `less` should return false if 'v1' equals to 'v2'.
func (d *DAG) Equals(other *DAG, less func(v1, v2 Vertex) bool) bool {
	if other == nil || less == nil {
		return false
	}
	// sort both DAGs in topology order.
	// a DAG may have more than one topology order, func 'less' is used to eliminate randomness
	// and hence only one deterministic order is generated.
	vertices1 := d.topologicalOrder(false, less)
	vertices2 := other.topologicalOrder(false, less)

	// compare them
	if len(vertices1) != len(vertices2) {
		return false
	}
	for i := range vertices1 {
		if less(vertices1[i], vertices2[i]) || less(vertices2[i], vertices1[i]) {
			return false
		}
	}
	return true
}

// Root returns root vertex that has no in adjacent.
// our DAG should have one and only one root vertex
func (d *DAG) Root() Vertex {
	roots := make([]Vertex, 0)
	for n := range d.vertices {
		if len(d.inAdj(n)) == 0 {
			roots = append(roots, n)
		}
	}
	if len(roots) != 1 {
		return nil
	}
	return roots[0]
}

func (d *DAG) Merge(subDag *DAG) {
	for v := range subDag.vertices {
		d.AddConnectRoot(v)
	}
	for e := range subDag.edges {
		d.AddEdge(e)
	}
}

// String returns a string representation of the DAG in topology order
func (d *DAG) String() string {
	str := "|"
	walkFunc := func(v Vertex) error {
		str += fmt.Sprintf("->%v", v)
		return nil
	}
	if err := d.WalkReverseTopoOrder(walkFunc, nil); err != nil {
		return "->err"
	}
	return str
}

// validate 'd' has single Root and has no cycles
func (d *DAG) validate() error {
	// single Root validation
	root := d.Root()
	if root == nil {
		return errors.New("no single Root found")
	}

	// self-cycle validation
	for e := range d.edges {
		if e.From() == e.To() {
			return fmt.Errorf("self-cycle found: %v", e.From())
		}
	}

	// cycle validation
	// use a DFS func to find cycles
	walked := make(map[Vertex]bool)
	marked := make(map[Vertex]bool)
	var walk func(v Vertex) error
	walk = func(v Vertex) error {
		if walked[v] {
			return nil
		}
		if marked[v] {
			return errors.New("cycle found")
		}

		marked[v] = true
		adjacent := d.outAdj(v)
		for _, vertex := range adjacent {
			if err := walk(vertex); err != nil {
				return err
			}
		}
		marked[v] = false
		walked[v] = true
		return nil
	}
	for v := range d.vertices {
		if err := walk(v); err != nil {
			return err
		}
	}
	return nil
}

// topologicalOrder returns a vertex list that is in topology order
// 'd' MUST be a legal DAG
func (d *DAG) topologicalOrder(reverse bool, less func(v1, v2 Vertex) bool) []Vertex {
	// orders is what we want, a (reverse) topological order of this DAG
	orders := make([]Vertex, 0)

	// walked marks vertex has been walked, to stop recursive func call
	walked := make(map[Vertex]bool)

	// walk is a DFS func
	var walk func(v Vertex)
	walk = func(v Vertex) {
		if walked[v] {
			return
		}
		var adjacent []Vertex
		if reverse {
			adjacent = d.outAdj(v)
		} else {
			adjacent = d.inAdj(v)
		}
		if less != nil {
			sort.SliceStable(adjacent, func(i, j int) bool {
				return less(adjacent[i], adjacent[j])
			})
		}
		for _, vertex := range adjacent {
			walk(vertex)
		}
		walked[v] = true
		orders = append(orders, v)
	}
	vertexLst := d.Vertices()
	if less != nil {
		sort.SliceStable(vertexLst, func(i, j int) bool {
			return less(vertexLst[i], vertexLst[j])
		})
	}
	for _, v := range vertexLst {
		walk(v)
	}
	return orders
}

// outAdj returns all adjacent vertices that v points to
func (d *DAG) outAdj(v Vertex) []Vertex {
	vertices := make([]Vertex, 0)
	for e := range d.edges {
		if e.From() == v {
			vertices = append(vertices, e.To())
		}
	}
	return vertices
}

// inAdj returns all adjacent vertices that point to v
func (d *DAG) inAdj(v Vertex) []Vertex {
	vertices := make([]Vertex, 0)
	for e := range d.edges {
		if e.To() == v {
			vertices = append(vertices, e.From())
		}
	}
	return vertices
}

// NewDAG news an empty DAG
func NewDAG() *DAG {
	dag := &DAG{
		vertices: make(map[Vertex]Vertex),
		edges:    make(map[Edge]Edge),
	}
	return dag
}

func RealEdge(from, to Vertex) Edge {
	return &realEdge{F: from, T: to}
}
