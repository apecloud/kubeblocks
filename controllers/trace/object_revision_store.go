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

package trace

import (
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// ObjectRevisionStore defines an object store which can get the history revision.
// WARN: This store is designed only for Reconciliation Trace Controller,
// it's not thread-safe, it doesn't do a deep copy before returning the object.
// Don't use it in other place.
type ObjectRevisionStore interface {
	Insert(object, reference client.Object) error
	Get(objectRef *model.GVKNObjKey, revision int64) (client.Object, error)
	List(gvk *schema.GroupVersionKind) map[types.NamespacedName]map[int64]client.Object
	Delete(objectRef *model.GVKNObjKey, reference client.Object, revision int64)
}

type objectRevisionStore struct {
	store     map[schema.GroupVersionKind]map[types.NamespacedName]map[int64]client.Object
	storeLock sync.RWMutex

	referenceCounter map[revisionObjectRef]sets.Set[types.UID]
	counterLock      sync.Mutex

	scheme *runtime.Scheme
}

type revisionObjectRef struct {
	model.GVKNObjKey
	revision int64
}

func (s *objectRevisionStore) Insert(object, reference client.Object) error {
	// insert into store
	s.storeLock.Lock()
	defer s.storeLock.Unlock()

	objectRef, err := getObjectRef(object, s.scheme)
	if err != nil {
		return err
	}
	objectMap, ok := s.store[objectRef.GroupVersionKind]
	if !ok {
		objectMap = make(map[types.NamespacedName]map[int64]client.Object)
		s.store[objectRef.GroupVersionKind] = objectMap
	}
	revisionMap, ok := objectMap[objectRef.ObjectKey]
	if !ok {
		revisionMap = make(map[int64]client.Object)
		objectMap[objectRef.ObjectKey] = revisionMap
	}
	revision := parseRevision(object.GetResourceVersion())
	revisionMap[revision] = object

	// update reference counter
	s.counterLock.Lock()
	defer s.counterLock.Unlock()

	revObjectRef := revisionObjectRef{
		GVKNObjKey: *objectRef,
		revision:   revision,
	}
	referenceMap, ok := s.referenceCounter[revObjectRef]
	if !ok {
		referenceMap = sets.New[types.UID]()
	}
	referenceMap.Insert(reference.GetUID())
	s.referenceCounter[revObjectRef] = referenceMap

	return nil
}

func (s *objectRevisionStore) Get(objectRef *model.GVKNObjKey, revision int64) (client.Object, error) {
	s.storeLock.RLock()
	defer s.storeLock.RUnlock()

	objectMap, ok := s.store[objectRef.GroupVersionKind]
	if !ok {
		return nil, apierrors.NewNotFound(objectRef.GroupVersion().WithResource(strings.ToLower(objectRef.Kind)).GroupResource(), objectRef.Name)
	}
	revisionMap, ok := objectMap[objectRef.ObjectKey]
	if !ok {
		return nil, apierrors.NewNotFound(objectRef.GroupVersion().WithResource(strings.ToLower(objectRef.Kind)).GroupResource(), objectRef.Name)
	}
	object, ok := revisionMap[revision]
	if !ok {
		return nil, apierrors.NewNotFound(objectRef.GroupVersion().WithResource(strings.ToLower(objectRef.Kind)).GroupResource(), objectRef.Name)
	}
	object = object.DeepCopyObject().(client.Object)
	return object, nil
}

func (s *objectRevisionStore) List(gvk *schema.GroupVersionKind) map[types.NamespacedName]map[int64]client.Object {
	s.storeLock.RLock()
	defer s.storeLock.RUnlock()

	objectMap, ok := s.store[*gvk]
	if !ok {
		return nil
	}
	objectMapCopy := make(map[types.NamespacedName]map[int64]client.Object, len(objectMap))
	for name, revisionMap := range objectMap {
		revisionMapCopy := make(map[int64]client.Object, len(revisionMap))
		objectMapCopy[name] = revisionMapCopy
		for revision, object := range revisionMap {
			objectCopy := object.DeepCopyObject().(client.Object)
			revisionMapCopy[revision] = objectCopy
		}
	}
	return objectMapCopy
}

func (s *objectRevisionStore) Delete(objectRef *model.GVKNObjKey, reference client.Object, revision int64) {
	s.storeLock.Lock()
	defer s.storeLock.Unlock()
	s.counterLock.Lock()
	defer s.counterLock.Unlock()

	// decrease reference counter
	revObjectRef := revisionObjectRef{
		GVKNObjKey: *objectRef,
		revision:   revision,
	}
	referenceMap, ok := s.referenceCounter[revObjectRef]
	if ok {
		referenceMap.Delete(reference.GetUID())
	}
	if len(referenceMap) > 0 {
		return
	}

	// delete object
	objectMap, ok := s.store[objectRef.GroupVersionKind]
	if !ok {
		return
	}
	revisionMap, ok := objectMap[objectRef.ObjectKey]
	if !ok {
		return
	}
	delete(revisionMap, revision)
	if len(referenceMap) == 0 {
		delete(objectMap, objectRef.ObjectKey)
	}
	if len(objectMap) == 0 {
		delete(s.store, objectRef.GroupVersionKind)
	}
}

func NewObjectStore(scheme *runtime.Scheme) ObjectRevisionStore {
	return &objectRevisionStore{
		store:            make(map[schema.GroupVersionKind]map[types.NamespacedName]map[int64]client.Object),
		referenceCounter: make(map[revisionObjectRef]sets.Set[types.UID]),
		scheme:           scheme,
	}
}

var _ ObjectRevisionStore = &objectRevisionStore{}
