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

package lifecycle

import (
	"fmt"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

func findAll[T interface{}](dag *graph.DAG) ([]graph.Vertex, error) {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, ok := vertex.(*lifecycleVertex)
		if !ok {
			return nil, fmt.Errorf("wrong type, expect lifecycleVertex, actual: %v", vertex)
		}
		if _, ok := v.obj.(T); ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices, nil
}

func findAllNot[T interface{}](dag *graph.DAG) ([]graph.Vertex, error) {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, ok := vertex.(*lifecycleVertex)
		if !ok {
			return nil, fmt.Errorf("wrong type, expect lifecycleVertex, actual: %v", vertex)
		}
		if _, ok := v.obj.(T); !ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices, nil
}