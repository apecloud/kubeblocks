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
	"fmt"
	"testing"
)

func TestWalkTopoOrder(t *testing.T) {
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

	expected := []int{4, 5, 1, 10, 12, 11, 9, 6, 0, 3, 2, 7, 8}
	walkOrder := make([]int, 0, len(expected))

	walkFunc := func(v Vertex) error {
		walkOrder = append(walkOrder, v.(int))
		fmt.Printf("%v ", v)
		return nil
	}
	if err := dag.WalkReverseTopoOrder(walkFunc); err != nil {
		t.Error(err)
	}
	for i := range expected {
		if walkOrder[i] != expected[i] {
			t.Errorf("unexpected order, index %d\n expected: %v\nactual: %v\n", i, expected, walkOrder)
		}
	}
	fmt.Println("")

	expected = []int{8, 7, 2, 3, 0, 6, 9, 11, 12, 10, 1, 5, 4}
	walkOrder = make([]int, 0, len(expected))
	if err := dag.WalkTopoOrder(walkFunc); err != nil {
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
	for i := 0; i < 4; i++ {
		dag.AddVertex(i)
	}
	dag.Connect(0, 1)
	dag.Connect(1, 2)
	dag.Connect(2, 3)
	dag.Connect(3, 1)
	err := dag.validate()
	if err == nil {
		t.Error("cycle not found")
	}
	if err.Error() != "cycle found" {
		t.Error("error not expected")
	}
}

func TestRemoveVertex(t *testing.T) {
	dag := NewDAG()
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

func TestEquals(t *testing.T) {
	d1 := NewDAG()
	d2 := NewDAG()
	for i := 0; i < 13; i++ {
		d1.AddVertex(i)
		d2.AddVertex(12 - i)
	}
	d1.Connect(2, 3)
	d1.Connect(0, 6)
	d1.Connect(0, 1)
	d1.Connect(2, 0)
	d1.Connect(11, 12)
	d1.Connect(9, 12)
	d1.Connect(9, 10)
	d1.Connect(9, 11)
	d1.Connect(3, 5)
	d1.Connect(8, 7)
	d1.Connect(5, 4)
	d1.Connect(0, 5)
	d1.Connect(6, 4)
	d1.Connect(6, 9)
	d1.Connect(7, 6)
	d1.Connect(7, 2)
	d1.Connect(3, 0)
	d1.Connect(12, 10)
	d1.Connect(10, 1)
	d1.Connect(1, 5)

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

	less := func(v1, v2 Vertex) bool {
		val1, _ := v1.(int)
		val2, _ := v2.(int)
		return val1 < val2
	}
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
