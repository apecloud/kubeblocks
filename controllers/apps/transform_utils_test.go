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

package apps

import (
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReflect(t *testing.T) {
	var list client.ObjectList
	sts := v1.StatefulSet{}
	sts.SetName("hello")
	list = &v1.StatefulSetList{Items: []v1.StatefulSet{sts}}
	v := reflect.ValueOf(list).Elem().FieldByName("Items")
	if v.Kind() != reflect.Slice {
		t.Error("not slice")
	}
	c := v.Len()
	objects := make([]client.Object, c)
	for i := 0; i < c; i++ {
		var st = v.Index(i).Addr().Interface()
		objects[i] = st.(client.Object)
	}
	for _, e := range objects {
		fmt.Println(e)
	}

	var o client.Object = &sts
	ptr := reflect.ValueOf(o)
	v = ptr.Elem().FieldByName("Spec")
	fmt.Println(v)
}
