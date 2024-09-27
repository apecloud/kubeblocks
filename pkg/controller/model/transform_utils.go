/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

func FindRootVertex(dag *graph.DAG) (*ObjectVertex, error) {
	root := dag.Root()
	if root == nil {
		return nil, fmt.Errorf("root vertex not found: %v", dag)
	}
	rootVertex, _ := root.(*ObjectVertex)
	return rootVertex, nil
}

func GetGVKName(object client.Object) (*GVKNObjKey, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &GVKNObjKey{
		GroupVersionKind: gvk,
		ObjectKey:        client.ObjectKeyFromObject(object),
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

func actionPtr(action Action) *Action {
	return &action
}

func ActionCreatePtr() *Action {
	return actionPtr(CREATE)
}

func ActionDeletePtr() *Action {
	return actionPtr(DELETE)
}

func ActionUpdatePtr() *Action {
	return actionPtr(UPDATE)
}

func ActionPatchPtr() *Action {
	return actionPtr(PATCH)
}

func ActionStatusPtr() *Action {
	return actionPtr(STATUS)
}

func ActionNoopPtr() *Action {
	return actionPtr(NOOP)
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

func IsReconciliationPaused(object client.Object) bool {
	value := reflect.ValueOf(object)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	spec := value.FieldByName("Spec")
	if !spec.IsValid() {
		return false
	}
	paused := spec.FieldByName("Paused")
	if !paused.IsValid() {
		return false
	}
	if paused.Kind() == reflect.Ptr {
		paused = paused.Elem()
	}
	if !paused.Type().AssignableTo(reflect.TypeOf(true)) {
		return false
	}
	return paused.Interface().(bool)
}

// ReadCacheSnapshot reads all objects owned by root object.
func ReadCacheSnapshot(transCtx graph.TransformContext, root client.Object, ml client.MatchingLabels, kinds ...client.ObjectList) (ObjectSnapshot, error) {
	snapshot := make(ObjectSnapshot)
	inNs := client.InNamespace(root.GetNamespace())
	for _, list := range kinds {
		if err := transCtx.GetClient().List(transCtx.GetContext(), list, inNs, ml); err != nil {
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

func DefaultLess(v1, v2 graph.Vertex) bool {
	o1, ok1 := v1.(*ObjectVertex)
	o2, ok2 := v2.(*ObjectVertex)
	if !ok1 || !ok2 {
		return false
	}
	return o1.String() < o2.String()
}
