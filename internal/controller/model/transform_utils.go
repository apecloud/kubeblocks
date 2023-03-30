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

package model

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

func FindMatchedVertex[T interface{}](dag *graph.DAG, objectKey client.ObjectKey) graph.Vertex {
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		if _, ok := v.Obj.(T); ok {
			if client.ObjectKeyFromObject(v.Obj) == objectKey {
				return vertex
			}
		}
	}
	return nil
}

func FindAll[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		if _, ok := v.Obj.(T); ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func FindAllNot[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		if _, ok := v.Obj.(T); !ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func FindRootVertex(dag *graph.DAG) (*ObjectVertex, error) {
	root := dag.Root()
	if root == nil {
		return nil, fmt.Errorf("root vertex not found: %v", dag)
	}
	rootVertex, _ := root.(*ObjectVertex)
	return rootVertex, nil
}

func GetGVKName(object client.Object, scheme *runtime.Scheme) (*GVKName, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &GVKName{
		gvk:  gvk,
		ns:   object.GetNamespace(),
		name: object.GetName(),
	}, nil
}

func IsOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) bool {
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

func ActionPtr(action Action) *Action {
	return &action
}

func NewRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

func isObjectDeleting(object client.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func isObjectUpdating(object client.Object) bool {
	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	status := value.FieldByName("Status")
	if !status.IsValid() {
		return false
	}
	observedGeneration := status.FieldByName("ObservedGeneration")
	if !observedGeneration.IsValid() {
		return false
	}
	generation := value.FieldByName("Generation")
	if !generation.IsValid() {
		return false
	}
	return observedGeneration.Interface() == generation.Interface()
}

func isObjectStatusUpdating(object client.Object) bool {
	return !isObjectDeleting(object) && !isObjectUpdating(object)
}
