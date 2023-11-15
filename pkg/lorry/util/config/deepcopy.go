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

package config

import (
	"errors"
	"reflect"
)

func Clone(s any) (any, error) {
	sValue := reflect.Indirect(reflect.ValueOf(s))
	sType := sValue.Type()
	d := reflect.New(sType).Interface()
	err := DeepCopy(s, d)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// DeepCopy make a compele copy for a struct value
func DeepCopy(s, d any) error {
	sValue := reflect.Indirect(reflect.ValueOf(s))
	sType := sValue.Type()

	dType := reflect.TypeOf(d)
	dValue := reflect.Indirect(reflect.ValueOf(d))
	if dType.Kind() != reflect.Pointer {
		return errors.New("dest object must be an Pointer")
	}
	dType = dType.Elem()

	if sType != dType {
		return errors.New("source and dest object type is not match")
	}

	if sType.Kind() != reflect.Struct {
		return errors.New("object type is not struct")
	}

	return deepCopy(sValue, dValue)
}

func deepCopy(s, d reflect.Value) error {
	kind := d.Kind()
	var err error
	switch kind {
	case reflect.Struct:
		err = deepCopyStruct(s, d)
	case reflect.Slice:
		err = deepCopySlice(s, d)
	case reflect.Map:
		err = deepCopyMap(s, d)
	case reflect.String:
		err = deepCopyString(s, d)
	case reflect.Pointer:
		err = deepCopyPointer(s, d)
	// case reflect.Func:
	//	break
	default:
		d.Set(s)
	}

	return err
}

func deepCopyStruct(s, d reflect.Value) error {
	// 	var structs = make([]any, 1, 5)
	// 	structs[0] = d
	// 	type field struct {
	// 		field reflect.StructField
	// 		val   reflect.Value
	// 	}
	//
	// 	fields := []field{}
	//
	// for len(structs) > 0 {
	// 	structData := structs[0]
	// 	structs = structs[1:]
	dValue := reflect.Indirect(d)
	sValue := reflect.Indirect(s)
	dType := dValue.Type()

	for i := 0; i < dType.NumField(); i++ {
		dfieldType := dType.Field(i)
		if !dfieldType.IsExported() {
			continue
		}

		dfieldValue := dValue.Field(i)
		sfieldValue := sValue.Field(i)
		err := deepCopy(sfieldValue, dfieldValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func deepCopyString(s, d reflect.Value) error {
	d.SetString(s.String())
	return nil
}

func deepCopyPointer(s, d reflect.Value) error {
	if s.IsNil() {
		return nil
	}
	sValue := reflect.Indirect(s)
	dType := sValue.Type()
	newValue := reflect.New(dType)

	err := deepCopy(sValue, reflect.Indirect(newValue))
	if err != nil {
		return err
	}
	d.Set(newValue)

	return nil
}

func deepCopySlice(s, d reflect.Value) error {
	dType := d.Type()
	valElemType := dType.Elem()
	sliceType := reflect.SliceOf(valElemType)
	sliceVar := reflect.MakeSlice(sliceType, s.Len(), s.Len())
	for i := 0; i < s.Len(); i++ {
		err := deepCopy(s.Index(i), sliceVar.Index(i))
		if err != nil {
			return err
		}
	}
	d.Set(sliceVar)
	return nil
}

func deepCopyMap(s, d reflect.Value) error {
	valType := d.Type()
	valKeyType := valType.Key()
	valElemType := valType.Elem()
	mapType := reflect.MapOf(valKeyType, valElemType)
	valMap := reflect.MakeMap(mapType)
	for _, k := range s.MapKeys() {
		currentKey := reflect.Indirect(reflect.New(valKeyType))
		err := deepCopy(k, currentKey)
		if err != nil {
			return err
		}
		currentElem := reflect.Indirect(reflect.New(valElemType))
		err = deepCopy(s.MapIndex(k), currentElem)
		if err != nil {
			return err
		}
		valMap.SetMapIndex(currentKey, currentElem)
	}
	d.Set(valMap)
	return nil
}
