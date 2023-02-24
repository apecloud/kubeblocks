package lifecycle

import (
	"fmt"
	v1 "k8s.io/api/apps/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
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
		var st interface{}
		st = v.Index(i).Addr().Interface()
		objects[i] = st.(client.Object)
	}
	for _, e := range objects {
		fmt.Println(e)
	}
}
