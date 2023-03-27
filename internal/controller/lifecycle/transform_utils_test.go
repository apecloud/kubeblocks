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

	var o client.Object
	o = &sts
	ptr := reflect.ValueOf(o)
	v = ptr.Elem().FieldByName("Spec")
	fmt.Println(v)
}
