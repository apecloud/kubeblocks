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
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var disableAutoTransfer = viper.GetBool("DISABLE_AUTO_TRANSFER")

const (
	k8sResourceAttr   = "k8sResource"
	attrQuantityValue = "quantity"
)

type CueWalkVisitor interface {
	Visit(val cue.Value)
}

type cueTypeExtractor struct {
	data       interface{}
	context    *cue.Context
	fieldTypes map[string]CueType
}

func (c *cueTypeExtractor) Visit(val cue.Value) {
	if c.fieldTypes == nil {
		c.fieldTypes = make(map[string]CueType)
	}
	c.visitStruct(val)
}

func (c *cueTypeExtractor) visitValue(x cue.Value, path string) {
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
		attr := x.Attribute(k8sResourceAttr)
		v, err := attr.String(0)
		if err == nil && v == attrQuantityValue {
			c.addFieldType(path, K8SQuantityType)
		} else {
			c.addFieldType(path, IntType)
		}
	case k&cue.ListKind == cue.ListKind:
		c.addFieldType(path, ListType)
		c.visitList(x, path)
	case k&cue.StructKind == cue.StructKind:
		c.addFieldType(path, StructType)
		c.visitStruct(x)
	default:
		log.Log.Info(fmt.Sprintf("cannot convert value of type %s", k.String()))
	}
}

func (c *cueTypeExtractor) visitStruct(v cue.Value) {
	switch op, v := v.Expr(); op {
	// SelectorOp refer of other struct type
	case cue.NoOp, cue.SelectorOp:
		// pass
	default:
		log.Log.Info(fmt.Sprintf("unsupported op %v for object type (%v)", op, v))
		return
	}

	for itr, _ := v.Fields(cue.Optional(true), cue.Definitions(true)); itr.Next(); {
		name := itr.Label()
		c.visitValue(itr.Value(), name)
	}
}

func (c *cueTypeExtractor) visitList(v cue.Value, path string) {
	switch op, _ := v.Expr(); op {
	case cue.NoOp, cue.SelectorOp:
		// pass
	default:
		log.Log.Info(fmt.Sprintf("unsupported op %v for object type (%v)", op, v))
	}

	count := 0
	for i, _ := v.List(); i.Next(); count++ {
		c.visitValue(i.Value(), fmt.Sprintf("%s_%d", path, count))
	}
}

func (c *cueTypeExtractor) addFieldType(fieldName string, cueType CueType) {
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
	case K8SQuantityType:
		return processTypeTrans[int64](obj, func(s string) (int64, error) {
			quantity, err := resource.ParseQuantity(s)
			if err != nil {
				return 0, err
			}
			return quantity.Value(), nil
		}, fn)
	default:
		// pass
	}
	return nil
}

func processTypeTrans[T int | int64 | float64 | float32 | bool](obj reflect.Value, transFn func(s string) (T, error), updateFn UpdateFn) error {
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

func processCfgNotStringParam(data interface{}, context *cue.Context, tpl cue.Value) error {
	if disableAutoTransfer {
		return nil
	}
	typeTransformer := &cueTypeExtractor{
		data:    data,
		context: context,
	}
	typeTransformer.Visit(tpl)
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
