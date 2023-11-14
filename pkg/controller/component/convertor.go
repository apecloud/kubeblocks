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

package component

import (
	"reflect"
	"strings"
)

// convertor is the interface for converting a value.
type convertor interface {
	convert(...any) (any, error)
}

// covertObject converts the fields of an object with the given convertors.
func covertObject(convertors map[string]convertor, obj any, args ...any) error {
	tp := typeofObject(obj)
	for i := 0; i < tp.NumField(); i++ {
		fieldName := tp.Field(i).Name
		c, ok := convertors[strings.ToLower(fieldName)]
		if !ok || c == nil {
			continue // leave the origin (default) value
		}

		val, err := c.convert(args...)
		if err != nil {
			return err
		}

		fieldValue := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
		switch {
		case reflect.TypeOf(val) == nil || reflect.ValueOf(val).IsZero():
			fieldValue.Set(reflect.Zero(fieldValue.Type()))
		case fieldValue.IsValid() && fieldValue.Type().AssignableTo(reflect.TypeOf(val)):
			fieldValue.Set(reflect.ValueOf(val))
		default:
			panic("not assignable")
		}
	}
	return nil
}

// typeofObject returns the typeof an object.
func typeofObject(obj any) reflect.Type {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		return reflect.TypeOf(obj).Elem()
	}
	if val.Kind() != reflect.Struct {
		panic("not a struct")
	}
	return reflect.TypeOf(obj)
}
