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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type gvkName struct {
	kind, ns, name string
}

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

func getGVKName(object client.Object) gvkName {
	return gvkName{
		kind: object.GetObjectKind().GroupVersionKind().Kind,
		ns: object.GetNamespace(),
		name: object.GetName(),
	}
}

func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) bool {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return false
	}
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return false
	}
	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}
	owners := obj.GetOwnerReferences()
	referSameObject := func(a, b metav1.OwnerReference) bool {
		aGV, err := schema.ParseGroupVersion(a.APIVersion)
		if err != nil {
			return false
		}

		bGV, err := schema.ParseGroupVersion(b.APIVersion)
		if err != nil {
			return false
		}

		return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
	}
	for _, ownerRef := range owners {
		if referSameObject(ownerRef, ref) {
			return true
		}
	}
	return false
}

func actionPtr(action Action) *Action {
	return &action
}