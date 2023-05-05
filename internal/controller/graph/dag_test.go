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

	walkFunc := func(v Vertex, dag DAG) error {
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
