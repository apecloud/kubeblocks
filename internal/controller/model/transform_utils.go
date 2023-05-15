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

package model

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

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

func GetGVKName(object client.Object) (*GVKName, error) {
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

func AddScheme(addToScheme func(*runtime.Scheme) error) {
	utilruntime.Must(addToScheme(scheme))
}

func GetScheme() *runtime.Scheme {
	return scheme
}

func IsOwnerOf(owner, obj client.Object) bool {
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

func IsObjectDeleting(object client.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func IsObjectUpdating(object client.Object) bool {
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
	return observedGeneration.Interface() != generation.Interface()
}

func IsObjectStatusUpdating(object client.Object) bool {
	return !IsObjectDeleting(object) && !IsObjectUpdating(object)
}

// ReadCacheSnapshot reads all objects owned by our cluster
func ReadCacheSnapshot(transCtx graph.TransformContext, root client.Object, ml client.MatchingLabels, kinds ...client.ObjectList) (ObjectSnapshot, error) {
	// list what kinds of object cluster owns
	snapshot := make(ObjectSnapshot)
	inNS := client.InNamespace(root.GetNamespace())
	for _, list := range kinds {
		if err := transCtx.GetClient().List(transCtx.GetContext(), list, inNS, ml); err != nil {
			return nil, err
		}
		// reflect get list.Items
		items := reflect.ValueOf(list).Elem().FieldByName("Items")
		l := items.Len()
		for i := 0; i < l; i++ {
			// get the underlying object
			object := items.Index(i).Addr().Interface().(client.Object)
			name, err := GetGVKName(object)
			if err != nil {
				return nil, err
			}
			snapshot[*name] = object
		}
	}

	return snapshot, nil
}

func PrepareCreate(dag *graph.DAG, object client.Object) {
	vertex := &ObjectVertex{
		Obj:    object,
		Action: ActionPtr(CREATE),
	}
	dag.AddConnectRoot(vertex)
}

func PrepareUpdate(dag *graph.DAG, objectOld, objectNew client.Object) {
	vertex := &ObjectVertex{
		Obj:    objectNew,
		OriObj: objectOld,
		Action: ActionPtr(UPDATE),
	}
	dag.AddConnectRoot(vertex)
}

func PrepareDelete(dag *graph.DAG, object client.Object) {
	vertex := &ObjectVertex{
		Obj:    object,
		Action: ActionPtr(DELETE),
	}
	dag.AddConnectRoot(vertex)
}

func PrepareStatus(dag *graph.DAG, objectOld, objectNew client.Object) {
	vertex := &ObjectVertex{
		Obj:    objectNew,
		OriObj: objectOld,
		Action: ActionPtr(STATUS),
	}
	dag.AddVertex(vertex)
}

func PrepareRootDelete(dag *graph.DAG) error {
	root, err := FindRootVertex(dag)
	if err != nil {
		return err
	}
	root.Action = ActionPtr(DELETE)
	return nil
}

func PrepareRootStatus(dag *graph.DAG) error {
	root, err := FindRootVertex(dag)
	if err != nil {
		return err
	}
	root.Action = ActionPtr(STATUS)
	return nil
}

func DependOn(dag *graph.DAG, object client.Object, dependency ...client.Object) {
	objectVertex := findMatchedVertex(dag, object)
	if objectVertex == nil {
		return
	}
	for _, d := range dependency {
		v := findMatchedVertex(dag, d)
		if v != nil {
			dag.Connect(objectVertex, v)
		}
	}
}

func findMatchedVertex(dag *graph.DAG, object client.Object) graph.Vertex {
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*ObjectVertex)
		if v.Obj == object || v.OriObj == object {
			return vertex
		}
		// TODO(free6om): compare by type and objectKey
	}
	return nil
}
