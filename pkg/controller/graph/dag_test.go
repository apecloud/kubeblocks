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
	"strings"
	"testing"
)

func TestAddVertex(t *testing.T) {
	dag := NewDAG()
	added := dag.AddVertex(nil)
	if added {
		t.Error("should return false if add nil vertex")
	}
	v := 6
	added = dag.AddVertex(v)
	if !added {
		t.Error("should return true if add none nil vertex")
	}
	if len(dag.Vertices()) != 1 {
		t.Error("vertex not added")
	}
	if dag.Vertices()[0] != v {
		t.Error("vertex not added")
	}
}

func TestRemoveVertex(t *testing.T) {
	dag := NewDAG()
	removed := dag.RemoveVertex(nil)
	if !removed {
		t.Error("should return true if removing nil vertex")
	}
	for i := 0; i < 4; i++ {
		dag.AddVertex(i)
	}
	dag.Connect(0, 1)
	dag.Connect(1, 2)
	dag.Connect(1, 3)
	if len(dag.vertices) != 4 {
		t.Error("unexpected vertices", len(dag.vertices))
	}
	if len(dag.edges) != 3 {
		t.Error("unexpected edges", len(dag.edges))
	}
	for i := 3; i >= 0; i-- {
		dag.RemoveVertex(i)
	}
	if len(dag.vertices) != 0 {
		t.Error("unexpected vertices", len(dag.vertices))
	}
	if len(dag.edges) != 0 {
		t.Error("unexpected edges", len(dag.edges))
	}
}

func TestAddNRemoveEdge(t *testing.T) {
	dag := NewDAG()
	added := dag.AddEdge(RealEdge(nil, nil))
	if added {
		t.Error("should return false if nil edge added")
	}
	v1, v2 := 3, 5
	e1 := RealEdge(v1, v2)
	e2 := RealEdge(v1, v2)
	added = dag.AddEdge(e1)
	if !added {
		t.Errorf("edge %v should be added", e1)
	}
	added = dag.AddEdge(e2)
	if !added {
		t.Errorf("edge %v should be added", e2)
	}
	if len(dag.edges) != 1 {
		t.Error("edge add failed")
	}
	if dag.edges[e1] != e1 {
		t.Error("edge add failed")
	}

	removed := dag.RemoveEdge(e2)
	if !removed {
		t.Errorf("remove edge %v failed", e2)
	}
	if len(dag.edges) != 0 {
		t.Errorf("remove edge %v failed", e2)
	}
}

func TestXConnect(t *testing.T) {
	dag := NewDAG()
	v1, v2 := 3, 5
	connected := dag.Connect(nil, v2)
	if connected {
		t.Error("connect nil vertex should return false")
	}
	connected = dag.Connect(v1, v2)
	if !connected {
		t.Errorf("connect %v to %v failed", v1, v2)
	}
	connected = dag.Connect(v1, v2)
	if !connected {
		t.Errorf("connect %v to %v failed", v1, v2)
	}
	if len(dag.edges) != 1 {
		t.Error("connect failed")
	}
	for edge := range dag.edges {
		if edge.From() != v1 || edge.To() != v2 {
			t.Errorf("edge in dag: %v, edge need: %v", edge, RealEdge(v1, v2))
		}
	}

	v3 := 7
	connected = dag.AddConnect(v1, nil)
	if connected {
		t.Error("AddConnect nil vertex should return false")
	}
	connected = dag.AddConnect(v1, v3)
	if !connected {
		t.Errorf("AddConnect %v to %v should succeed", v1, v3)
	}
	v4 := 9
	connected = dag.AddConnectRoot(v4)
	if connected {
		t.Errorf("AddConnectRoot to %v with nil root should failed", v4)
	}
	dag.AddVertex(v1)
	connected = dag.AddConnectRoot(v4)
	if !connected {
		t.Errorf("AddConnectRoot to %v should succeed", v4)
	}
}

func TestWalkTopoOrder(t *testing.T) {
	dag := newTestDAG()

	expected := []int{4, 5, 1, 10, 12, 11, 9, 6, 0, 3, 2, 7, 8}
	walkOrder := make([]int, 0, len(expected))

	walkFunc := func(v Vertex) error {
		walkOrder = append(walkOrder, v.(int))
		return nil
	}
	if err := dag.WalkReverseTopoOrder(walkFunc, nil); err != nil {
		t.Error(err)
	}
	for i := range expected {
		if walkOrder[i] != expected[i] {
			t.Errorf("unexpected order, index %d\n expected: %v\nactual: %v\n", i, expected, walkOrder)
		}
	}

	expected = []int{8, 7, 2, 3, 0, 6, 9, 11, 12, 10, 1, 5, 4}
	walkOrder = make([]int, 0, len(expected))
	if err := dag.WalkTopoOrder(walkFunc, nil); err != nil {
		t.Error(err)
	}
	for i := range expected {
		if walkOrder[i] != expected[i] {
			t.Errorf("unexpected order, index %d\n expected: %v\nactual: %v\n", i, expected, walkOrder)
		}
	}
}

func TestWalkBFS(t *testing.T) {
	dag := newTestDAG()

	expected := []int{8, 7, 2, 6, 0, 3, 4, 9, 1, 5, 10, 11, 12}
	walkOrder := make([]int, 0, len(expected))

	walkFunc := func(v Vertex) error {
		walkOrder = append(walkOrder, v.(int))
		return nil
	}
	if err := dag.bfs(walkFunc, less); err != nil {
		t.Error(err)
	}
	for i := range expected {
		if walkOrder[i] != expected[i] {
			t.Errorf("unexpected order, index %d\n expected: %v\nactual: %v\n", i, expected, walkOrder)
		}
	}
}

func TestValidate(t *testing.T) {
	dag := NewDAG()
	err := dag.validate()
	if err == nil {
		t.Error("nil root not found")
	}
	if !strings.Contains(err.Error(), "no single Root found") {
		t.Error("nil root not found")
	}
	for i := 0; i < 4; i++ {
		dag.AddVertex(i)
	}
	dag.Connect(0, 1)
	dag.Connect(1, 2)
	dag.Connect(2, 3)
	dag.Connect(3, 1)
	err = dag.validate()
	if err == nil {
		t.Error("cycle not found")
	}
	if err.Error() != "cycle found" {
		t.Error("error not as expected")
	}
	dag.Connect(1, 1)
	err = dag.validate()
	if err == nil {
		t.Error("self-cycle not found")
	}
	if err.Error() != "self-cycle found: 1" {
		t.Error("error not as expected")
	}
}

func TestEquals(t *testing.T) {
	d1 := newTestDAG()
	equal := d1.Equals(nil, less)
	if equal {
		t.Error("should return false if nil other")
	}
	d2 := NewDAG()
	equal = d1.Equals(d2, nil)
	if equal {
		t.Error("should return false if nil less func")
	}
	for i := 0; i < 13; i++ {
		d2.AddVertex(12 - i)
	}

	// add edges in reverse order
	d2.Connect(1, 5)
	d2.Connect(10, 1)
	d2.Connect(12, 10)
	d2.Connect(3, 0)
	d2.Connect(7, 2)
	d2.Connect(7, 6)
	d2.Connect(6, 9)
	d2.Connect(6, 4)
	d2.Connect(0, 5)
	d2.Connect(5, 4)
	d2.Connect(8, 7)
	d2.Connect(3, 5)
	d2.Connect(9, 11)
	d2.Connect(9, 10)
	d2.Connect(9, 12)
	d2.Connect(11, 12)
	d2.Connect(2, 0)
	d2.Connect(0, 1)
	d2.Connect(0, 6)
	d2.Connect(2, 3)

	if !d1.Equals(d2, less) {
		t.Error("equals test failed")
	}

	d1 = NewDAG()
	d2 = NewDAG()

	d1.AddVertex(0)
	d1.AddVertex(1)
	d1.AddVertex(2)
	d1.AddVertex(3)
	d1.AddVertex(4)
	d2.AddVertex(0)
	d2.AddVertex(2)
	d2.AddVertex(3)
	d2.AddVertex(1)
	d2.AddVertex(4)

	d1.Connect(0, 1)
	d1.Connect(0, 2)
	d1.Connect(0, 3)
	d1.Connect(0, 4)
	d2.Connect(0, 2)
	d2.Connect(0, 3)
	d2.Connect(0, 4)
	d2.Connect(0, 1)

	if !d1.Equals(d2, less) {
		t.Error("equals test failed")
	}
}

func TestMerge(t *testing.T) {
	dag1 := NewDAG()
	dag2 := NewDAG()
	v1, v2, v3, v4, v5, v6 := 1, 2, 3, 4, 5, 6
	dag1.AddVertex(v1)
	dag1.AddVertex(v2)
	dag1.Connect(v1, v2)
	dag2.AddVertex(v3)
	dag2.AddVertex(v4)
	dag2.AddVertex(v5)
	dag2.AddVertex(v6)
	dag2.Connect(v2, v3)
	dag2.Connect(v2, v4)
	dag2.Connect(v3, v5)
	dag2.Connect(v4, v5)
	dag2.Connect(v6, v4)
	dag2.Connect(v6, v5)

	dagExpected := NewDAG()
	dagExpected.AddVertex(v1)
	dagExpected.AddVertex(v2)
	dagExpected.AddVertex(v3)
	dagExpected.AddVertex(v4)
	dagExpected.AddVertex(v5)
	dagExpected.AddVertex(v6)
	dagExpected.Connect(v1, v2)
	dagExpected.Connect(v1, v6)
	dagExpected.Connect(v2, v3)
	dagExpected.Connect(v2, v4)
	dagExpected.Connect(v3, v5)
	dagExpected.Connect(v4, v5)
	dagExpected.Connect(v6, v4)
	dagExpected.Connect(v6, v5)

	dag1.Merge(dag2)
	if !dag1.Equals(dagExpected, less) {
		t.Errorf("dag merge error, expected: %v, actual: %v", dagExpected, dag1)
	}
}

func TestString(t *testing.T) {
	dag := newTestDAG()
	str := dag.String()
	expectedOrder := []string{"|", "4", "5", "1", "10", "12", "11", "9", "6", "0", "3", "2", "7", "8"}
	expectedStr := strings.Join(expectedOrder, "->")
	if str != expectedStr {
		t.Errorf("dag string error, expected: %s, actual: %s", expectedStr, str)
	}
}

func less(v1, v2 Vertex) bool {
	val1, _ := v1.(int)
	val2, _ := v2.(int)
	return val1 < val2
}

func newTestDAG() *DAG {
	dag := NewDAG()
	for i := 0; i < 13; i++ {
		dag.AddVertex(i)
	}
	dag.Connect(2, 3)
	dag.Connect(0, 6)
	dag.Connect(0, 1)
	dag.Connect(2, 0)
	dag.Connect(11, 12)
	dag.Connect(9, 12)
	dag.Connect(9, 10)
	dag.Connect(9, 11)
	dag.Connect(3, 5)
	dag.Connect(8, 7)
	dag.Connect(5, 4)
	dag.Connect(0, 5)
	dag.Connect(6, 4)
	dag.Connect(6, 9)
	dag.Connect(7, 6)
	dag.Connect(7, 2)
	dag.Connect(3, 0)
	dag.Connect(12, 10)
	dag.Connect(10, 1)
	dag.Connect(1, 5)
	return dag
}
