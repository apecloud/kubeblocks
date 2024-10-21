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
	"sort"
	"strconv"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type ChangeCaptureStore interface {
	Load(objects ...client.Object) error
	Insert(object client.Object) error
	Update(object client.Object) error
	Delete(object client.Object) error
	Get(objectRef *model.GVKNObjKey) client.Object
	List(gvk *schema.GroupVersionKind) []client.Object
	GetAll() map[model.GVKNObjKey]client.Object
	GetChanges() []tracev1.ObjectChange
}

type changeCaptureStore struct {
	scheme    *runtime.Scheme
	formatter descriptionFormatter
	store     map[model.GVKNObjKey]client.Object
	clock     atomic.Int64
	changes   []tracev1.ObjectChange
}

func (s *changeCaptureStore) Load(objects ...client.Object) error {
	for _, object := range objects {
		// sync the clock
		revision := parseRevision(object.GetResourceVersion())
		if revision > s.clock.Load() {
			s.clock.Store(revision)
		}
		objectRef, err := getObjectRef(object, s.scheme)
		if err != nil {
			return err
		}
		s.store[*objectRef] = object

	}
	return nil
}

func (s *changeCaptureStore) Insert(object client.Object) error {
	var err error
	obj := object.DeepCopyObject().(client.Object)
	obj, err = normalize(obj)
	if err != nil {
		return err
	}
	objectRef, err := getObjectRef(obj, s.scheme)
	if err != nil {
		return err
	}
	obj.SetResourceVersion(s.applyRevision())
	object.SetResourceVersion(obj.GetResourceVersion())
	s.store[*objectRef] = obj

	s.captureCreation(objectRef, obj)

	return nil
}

func (s *changeCaptureStore) Update(object client.Object) error {
	var err error
	newObj := object.DeepCopyObject().(client.Object)
	newObj, err = normalize(newObj)
	if err != nil {
		return err
	}
	objectRef, err := getObjectRef(newObj, s.scheme)
	if err != nil {
		return err
	}
	oldObj := s.store[*objectRef]
	newObj.SetResourceVersion(s.applyRevision())
	object.SetResourceVersion(newObj.GetResourceVersion())
	s.store[*objectRef] = newObj

	s.captureUpdate(objectRef, oldObj, newObj)
	return nil
}

func (s *changeCaptureStore) Delete(object client.Object) error {
	objectRef, err := getObjectRef(object, s.scheme)
	if err != nil {
		return err
	}
	obj, ok := s.store[*objectRef]
	if !ok {
		return nil
	}
	delete(s.store, *objectRef)

	s.captureDeletion(objectRef, obj)
	return nil
}

func (s *changeCaptureStore) Get(objectRef *model.GVKNObjKey) client.Object {
	return s.store[*objectRef]
}

func (s *changeCaptureStore) List(gvk *schema.GroupVersionKind) []client.Object {
	var objects []client.Object
	for objectRef, object := range s.store {
		if objectRef.GroupVersionKind == *gvk {
			objects = append(objects, object)
		}
	}
	return objects
}

func (s *changeCaptureStore) GetAll() map[model.GVKNObjKey]client.Object {
	all := make(map[model.GVKNObjKey]client.Object, len(s.store))
	for key, object := range s.store {
		all[key] = object.DeepCopyObject().(client.Object)
	}
	return all
}

func (s *changeCaptureStore) GetChanges() []tracev1.ObjectChange {
	sort.SliceStable(s.changes, func(i, j int) bool {
		return s.changes[i].Revision < s.changes[j].Revision
	})
	return s.changes
}

func newChangeCaptureStore(scheme *runtime.Scheme, formatter descriptionFormatter) ChangeCaptureStore {
	return &changeCaptureStore{
		scheme:    scheme,
		store:     make(map[model.GVKNObjKey]client.Object),
		formatter: formatter,
	}
}

func (s *changeCaptureStore) applyRevision() string {
	return strconv.FormatInt(s.clock.Add(1), 10)
}

func (s *changeCaptureStore) captureCreation(objectRef *model.GVKNObjKey, object client.Object) {
	changes := buildChanges(
		make(map[model.GVKNObjKey]client.Object),
		map[model.GVKNObjKey]client.Object{*objectRef: object},
		s.formatter)
	s.changes = append(s.changes, changes...)
}

func (s *changeCaptureStore) captureUpdate(objectRef *model.GVKNObjKey, obj client.Object, object client.Object) {
	changes := buildChanges(
		map[model.GVKNObjKey]client.Object{*objectRef: obj},
		map[model.GVKNObjKey]client.Object{*objectRef: object},
		s.formatter)
	s.changes = append(s.changes, changes...)
}

func (s *changeCaptureStore) captureDeletion(objectRef *model.GVKNObjKey, object client.Object) {
	changes := buildChanges(
		map[model.GVKNObjKey]client.Object{*objectRef: object},
		make(map[model.GVKNObjKey]client.Object),
		s.formatter)
	s.changes = append(s.changes, changes...)
}

var _ ChangeCaptureStore = &changeCaptureStore{}
