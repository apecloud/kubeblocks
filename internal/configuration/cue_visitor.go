/*
Copyright ApeCloud Inc.

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

	"cuelang.org/go/cue"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var disableAutoTransfer = viper.GetBool("DISABLE_AUTO_TRANSFER")

type WalkVisitor interface {
	Visit(val cue.Value) error
}

type CueTypeExtractor struct {
	data       interface{}
	context    *cue.Context
	fieldTypes map[string]CueType
}

func (c *CueTypeExtractor) Visit(val cue.Value) error {
	if c.fieldTypes == nil {
		c.fieldTypes = make(map[string]CueType)
	}
	itr, err := val.Fields(cue.Definitions(true))
	if err != nil {
		return err
	}

	for itr.Next() {
		if !itr.IsDefinition() {
			continue
		}
		v := itr.Value()
		label := itr.Label()
		c.visitValue(v, label)
	}
	return nil
}

func (c *CueTypeExtractor) visitValue(x cue.Value, path string) {
	k := x.IncompleteKind()
	switch {
	case k&cue.NullKind == cue.NullKind:
		c.addFieldType(path, NullableType)
	case k&cue.BytesKind == cue.BytesKind:
		c.addFieldType(path, StringType)
	case k&cue.StringKind == cue.StringKind:
		c.addFieldType(path, StringType)
	case k&cue.FloatKind == cue.FloatKind:
		c.addFieldType(path, FloatType)
	case k&cue.IntKind == cue.IntKind:
		c.addFieldType(path, IntType)
	case k&cue.ListKind == cue.ListKind:
		c.addFieldType(path, ListType)
		c.visitList(x, path)
	case k&cue.StructKind == cue.StructKind:
		c.addFieldType(path, StructType)
		c.visitStruct(x)
	default:
		logrus.Warnf("cannot convert value of type %s", k.String())
	}
}

func (c *CueTypeExtractor) visitStruct(v cue.Value) {
	switch op, v := v.Expr(); op {
	// SelectorOp refer of other struct type
	case cue.NoOp, cue.SelectorOp:
		// pass
	default:
		logrus.Warnf("unsupported op %v for object type (%v)", op, v)
		return
	}

	for itr, _ := v.Fields(cue.Optional(true), cue.Definitions(true)); itr.Next(); {
		name := itr.Label()
		c.visitValue(itr.Value(), name)
	}
}

func (c *CueTypeExtractor) visitList(v cue.Value, path string) {
	switch op, _ := v.Expr(); op {
	case cue.NoOp, cue.SelectorOp:
		// pass
	default:
		logrus.Warnf("unsupported op %v for object type (%v)", op, v)
	}

	count := 0
	for i, _ := v.List(); i.Next(); count++ {
		c.visitValue(i.Value(), fmt.Sprintf("%s_%d", path, count))
	}
}

func (c *CueTypeExtractor) addFieldType(fieldName string, cueType CueType) {
	c.fieldTypes[fieldName] = cueType
}

func transNumberOrBoolType(t CueType, obj reflect.Value, fn UpdateFn) error {
	switch t {
	case IntType:
		return processTypeTrans[int](obj, strconv.Atoi, fn)
	case BoolType:
		return processTypeTrans[bool](obj, strconv.ParseBool, fn)
	case FloatType:
		return processTypeTrans[float64](obj, func(s string) (float64, error) {
			return strconv.ParseFloat(s, 64)
		}, fn)
	default:
		// pass
	}
	return nil
}

func processTypeTrans[T int | float64 | float32 | bool](obj reflect.Value, transFn func(s string) (T, error), updateFn UpdateFn) error {
	switch obj.Type().Kind() {
	case reflect.String:
		v, err := transFn(obj.String())
		if err != nil {
			return err
		}
		updateFn(v)
	case reflect.Array, reflect.Slice, reflect.Struct:
		return MakeError("not support type[%s] trans.", obj.Type().Kind())
	}

	return nil
}

func ProcessCfgNotStringParam(data interface{}, context *cue.Context, tpl cue.Value) error {
	if disableAutoTransfer {
		return nil
	}
	typeTransformer := &CueTypeExtractor{
		data:    data,
		context: context,
	}
	if err := typeTransformer.Visit(tpl); err != nil {
		return err
	}

	return UnstructuredObjectWalk(typeTransformer.data,
		func(parent, cur string, obj reflect.Value, fn UpdateFn) error {
			if fn == nil || cur == "" || !obj.IsValid() {
				return nil
			}
			if t, exist := typeTransformer.fieldTypes[cur]; exist {
				return transNumberOrBoolType(t, obj, fn)
			}
			return nil
		}, false)
}
