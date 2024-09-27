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

package view

import (
	"strconv"
	"sync/atomic"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
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
	GetChanges() []viewv1.ObjectChange
}

type changeCaptureStore struct {
	scheme        *runtime.Scheme
	i18nResources *corev1.ConfigMap
	defaultLocale *string
	locale        *string
	store         map[model.GVKNObjKey]client.Object
	clock         atomic.Int64
	changes       []viewv1.ObjectChange
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
	objectRef, err := getObjectRef(object, s.scheme)
	if err != nil {
		return err
	}
	object.SetResourceVersion(s.applyRevision())
	s.store[*objectRef] = object

	s.captureCreation(objectRef, object)

	return nil
}

func (s *changeCaptureStore) Update(object client.Object) error {
	objectRef, err := getObjectRef(object, s.scheme)
	if err != nil {
		return err
	}
	oldObj, _ := s.store[*objectRef]
	object.SetResourceVersion(s.applyRevision())
	s.store[*objectRef] = object

	s.captureUpdate(objectRef, oldObj, object)
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
	if obj.GetDeletionTimestamp() == nil {
		ts := metav1.Now()
		obj.SetDeletionTimestamp(&ts)
	} else {
		delete(s.store, *objectRef)
	}

	obj.SetResourceVersion(s.applyRevision())
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

func (s *changeCaptureStore) GetChanges() []viewv1.ObjectChange {
	slices.SortStableFunc(s.changes, func(a, b viewv1.ObjectChange) bool {
		return a.Revision < b.Revision
	})
	return s.changes
}

func newChangeCaptureStore(scheme *runtime.Scheme, resource *corev1.ConfigMap) ChangeCaptureStore {
	return &changeCaptureStore{
		scheme:        scheme,
		i18nResources: resource,
		store:         make(map[model.GVKNObjKey]client.Object),
	}
}

func (s *changeCaptureStore) applyRevision() string {
	return strconv.FormatInt(s.clock.Add(1), 10)
}

func (s *changeCaptureStore) captureCreation(objectRef *model.GVKNObjKey, object client.Object) {
	changes := buildChanges(sets.New(*objectRef),
		nil, map[model.GVKNObjKey]client.Object{*objectRef: object},
		viewv1.ObjectCreationType, s.i18nResources, s.defaultLocale, s.locale)
	s.changes = append(s.changes, changes...)
}

func (s *changeCaptureStore) captureUpdate(objectRef *model.GVKNObjKey, obj client.Object, object client.Object) {
	changes := buildChanges(sets.New(*objectRef),
		map[model.GVKNObjKey]client.Object{*objectRef: obj},
		map[model.GVKNObjKey]client.Object{*objectRef: object},
		viewv1.ObjectUpdateType, s.i18nResources, s.defaultLocale, s.locale)
	s.changes = append(s.changes, changes...)
}

func (s *changeCaptureStore) captureDeletion(objectRef *model.GVKNObjKey, object client.Object) {
	changes := buildChanges(sets.New(*objectRef),
		map[model.GVKNObjKey]client.Object{*objectRef: object}, nil,
		viewv1.ObjectDeletionType, s.i18nResources, s.defaultLocale, s.locale)
	s.changes = append(s.changes, changes...)
}

var _ ChangeCaptureStore = &changeCaptureStore{}
