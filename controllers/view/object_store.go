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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjectStore interface {
	Insert(object client.Object) error
	Get(objKey *corev1.ObjectReference) client.Object
	Delete(objKey *corev1.ObjectReference)
}

type objectStore struct {

}

func (o *objectStore) Insert(object client.Object) error {
	//TODO implement me
	panic("implement me")
}

func (o *objectStore) Get(objKey *corev1.ObjectReference) client.Object {
	//TODO implement me
	panic("implement me")
}

func (o *objectStore) Delete(objKey *corev1.ObjectReference) {
	//TODO implement me
	panic("implement me")
	// TODO(free6om): decrease reference counter as one object may in many views
}

func NewObjectStore() ObjectStore {
	return &objectStore{}
}

var _ ObjectStore = &objectStore{}