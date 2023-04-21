/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
		return fmt.Errorf("invalid data type: %T", data)
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
		return accessor.visitStruct(v, joinFieldPath(parent, cur))
	case reflect.Map:
		return accessor.visitMap(v, t, joinFieldPath(parent, cur))
	case reflect.Slice:
		return accessor.visitArray(v, t.Elem(), parent, cur)
	case reflect.Array:
		return accessor.visitArray(v, t.Elem(), parent, cur)
	case reflect.Pointer:
		return accessor.visitValueType(v, t.Elem(), parent, cur, updateFn)
	default:
		return fmt.Errorf("not support type: %s", k)
	}
}

func (accessor *unstructuredAccessor) visitArray(v reflect.Value, t reflect.Type, parent, cur string) error {
	n := v.Len()
	for i := 0; i < n; i++ {
		// index := fmt.Sprintf("%s_%d", parent, i)
		if err := accessor.visitValueType(v.Index(i), t, parent, cur, nil); err != nil {
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
		return fmt.Errorf("not support key type: %s", k)
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

func joinFieldPath(parent, cur string) string {
	if parent == "" {
		return cur
	}

	if cur == "" {
		return parent
	}

	return parent + "." + cur
}

func (accessor *unstructuredAccessor) visitStruct(v reflect.Value, parent string) error {
	return fmt.Errorf("not support struct")
}
