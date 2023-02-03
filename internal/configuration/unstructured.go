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

package configuration

import (
	"fmt"
	"reflect"
	"strconv"
)

type UpdateFn func(v interface{})
type NodeProcessFn func(parent, cur string, v reflect.Value, fn UpdateFn) error

func UnstructuredObjectWalk(data interface{}, fn NodeProcessFn, onlyAccessNode bool) error {
	if data == nil {
		return nil
	}

	visitor := unstructuredAccessor{
		isUpdate: !onlyAccessNode,
		fn:       fn,
	}

	return visitor.Visit(data)
}

type unstructuredAccessor struct {
	isUpdate bool
	fn       NodeProcessFn
}

func (accessor *unstructuredAccessor) Visit(data interface{}) error {
	v := reflect.ValueOf(data)
	if !v.IsValid() {
		return MakeError("invalid data type: %T", data)
	}
	return accessor.visitValueType(v, v.Type(), "", "", nil)
}

func (accessor *unstructuredAccessor) visitValueType(v reflect.Value, t reflect.Type, parent, cur string, updateFn UpdateFn) error {
	switch k := t.Kind(); k {
	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.String:
		return accessor.fn(parent, cur, v, updateFn)
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}

		implValue := v.Elem()
		return accessor.visitValueType(implValue, implValue.Type(), parent, cur, updateFn)
	case reflect.Struct:
		return accessor.visitStruct(v, cur)
	case reflect.Map:
		return accessor.visitMap(v, t, cur)
	case reflect.Slice:
		return accessor.visitArray(v, t.Elem(), cur)
	case reflect.Array:
		return accessor.visitArray(v, t.Elem(), cur)
	case reflect.Pointer:
		return accessor.visitValueType(v, t.Elem(), parent, cur, updateFn)
	default:
		return MakeError("not support type: %s", k)
	}
}

func (accessor *unstructuredAccessor) visitArray(v reflect.Value, t reflect.Type, parent string) error {
	n := v.Len()
	for i := 0; i < n; i++ {
		index := fmt.Sprintf("%s_%d", parent, i)
		if err := accessor.visitValueType(v.Index(i), t, parent, index, nil); err != nil {
			return err
		}
	}
	return nil
}

func (accessor *unstructuredAccessor) visitMap(v reflect.Value, t reflect.Type, parent string) error {
	// return if empty
	if v.Len() == 0 {
		return nil
	}

	switch k := t.Key().Kind(); k {
	case reflect.String:
	default:
		return MakeError("not support key type: %s", k)
	}

	t = t.Elem()
	var updateFn = createMapUpdateFn[string](v, accessor.isUpdate)
	mi := v.MapRange()
	for i := 0; mi.Next(); i++ {
		keyType := mi.Key()
		key := toString(keyType, keyType.Kind())
		if err := accessor.visitValueType(mi.Value(), t, parent, key, func(newObj interface{}) {
			if updateFn != nil {
				updateFn(key, newObj)
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func createMapUpdateFn[T comparable](v reflect.Value, isUpdate bool) func(key T, val interface{}) {
	if !isUpdate {
		return nil
	}

	obj, _ := v.Interface().(map[T]any)
	return func(key T, val interface{}) {
		if val != nil {
			obj[key] = val
		}
	}
}

func toString(key reflect.Value, kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return key.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(key.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(key.Uint(), 10)
	default:
		return ""
	}
}

func (accessor *unstructuredAccessor) visitStruct(v reflect.Value, parent string) error {
	return MakeError("not support struct.")
}
