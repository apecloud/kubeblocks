/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package util

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockReader struct {
	Objects []client.Object
}

func (r *MockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	for _, o := range r.Objects {
		// ignore the GVK check
		if client.ObjectKeyFromObject(o) == key {
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(o).Elem())
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func (r *MockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	if !items.IsValid() {
		return fmt.Errorf("ObjectList has no Items field: %s", list.GetObjectKind().GroupVersionKind().String())
	}

	objs := reflect.MakeSlice(items.Type(), 0, 0)
	if len(r.Objects) > 0 {
		listOpts := &client.ListOptions{}
		for _, opt := range opts {
			opt.ApplyToList(listOpts)
		}

		for i, o := range r.Objects {
			if reflect.TypeOf(r.Objects[i]).Elem().AssignableTo(items.Type().Elem()) {
				if listOpts.LabelSelector == nil || listOpts.LabelSelector.Matches(labels.Set(o.GetLabels())) {
					objs = reflect.Append(objs, reflect.ValueOf(r.Objects[i]).Elem())
				}
			}
		}
	}
	items.Set(objs)

	return nil
}
