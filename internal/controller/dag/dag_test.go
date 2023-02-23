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

import (
	"fmt"
	"testing"
)

func TestWalkDepthFirst(t *testing.T) {
	dag := New()
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
	dag.Connect(7, 8)
	dag.Connect(5, 4)
	dag.Connect(0, 5)
	dag.Connect(6, 4)
	dag.Connect(6, 9)
	dag.Connect(6, 7)
	walkFunc := func(v Vertex) error {
		fmt.Printf("%v ", v)
		return nil
	}
	if err := dag.WalkDepthFirst(walkFunc); err != nil {
		t.Error(err)
	}
}