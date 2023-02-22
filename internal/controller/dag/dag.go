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

type DAG struct {
	Nodes map[string]*Node
	Edges []*Edge
}

type Node struct {
	Obj interface{}
	Immutable bool
	Action *Action
}

type Action string
const (
	CREATE = "CREATE"
	UPDATE = "UPDATE"
	DELETE = "DELETE"
)

type Edge struct {
	From string
	To string
}

type WalkFunc func(node Node) error

func (d *DAG) AddNode(key string, obj interface{}) bool {
	if len(key) == 0 || obj == nil {
		return false
	}
	node := &Node{
		Obj: obj,
		Immutable: false,
	}
	d.Nodes[key] = node
	return true
}

func (d *DAG) RemoveNode(key string) bool {
	if len(key) == 0 {
		return true
	}
	edges := make([]*Edge, 0)
	for i := range d.Edges {
		if d.Edges[i].From != key && d.Edges[i].To != key {
			edges = append(edges, d.Edges[i])
		}
	}
	d.Edges = edges
	return true
}

func (d *DAG) AddEdge(from, to string) bool {
	if len(from) == 0 || len(to) == 0 {
		return false
	}
	if d.Nodes[from] == nil || d.Nodes[to] == nil {
		return false
	}
	edge := &Edge{
		From: from,
		To: to,
	}
	d.Edges = append(d.Edges, edge)
	return true
}

func (d *DAG) RemoveEdge(from, to string) bool {
	if len(from) == 0 || len(to) == 0 {
		return true
	}
	edges := make([]*Edge, 0)
	for i := range d.Edges {
		if d.Edges[i].From == from && d.Edges[i].To == to {
			continue
		}
		edges = append(edges, d.Edges[i])
	}
	return true
}

func (d *DAG) WalkDepthFirst(walkFunc WalkFunc) error {
	return nil
}

func (d *DAG) WalkBreadthFirst(walkFunc WalkFunc) error {
	return nil
}

func New() *DAG {
	dag := &DAG{
		Nodes: make(map[string]*Node, 0),
		Edges: make([]*Edge, 0),
	}
	return dag
}