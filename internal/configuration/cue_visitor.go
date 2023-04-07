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
	"strings"

	"cuelang.org/go/cue"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var disableAutoTransfer = viper.GetBool("DISABLE_AUTO_TRANSFER")

type CueWalkVisitor interface {
	Visit(val cue.Value)
}

type cueTypeExtractor struct {
	data       interface{}
	context    *cue.Context
	fieldTypes map[string]CueType
	fieldUnits map[string]string
}

func (c *cueTypeExtractor) Visit(val cue.Value) {
	if c.fieldTypes == nil {
		c.fieldTypes = make(map[string]CueType)
		c.fieldUnits = make(map[string]string)
	}
	c.visitStruct(val, "")
}

func (c *cueTypeExtractor) visitValue(x cue.Value, path string) {
	k := x.IncompleteKind()
	switch {
	case k&cue.NullKind == cue.NullKind:
		c.addFieldType(path, NullableType)
	case k&cue.BytesKind == cue.BytesKind:
		c.addFieldType(path, StringType)
	case k&cue.BoolKind == cue.BoolKind:
		c.addFieldType(path, BoolType)
	case k&cue.StringKind == cue.StringKind:
		c.addFieldType(path, StringType)
	case k&cue.FloatKind == cue.FloatKind:
		c.addFieldType(path, FloatType)
	case k&cue.IntKind == cue.IntKind:
		t, base := processCueIntegerExpansion(x)
		c.addFieldUnits(path, t, base)
	case k&cue.ListKind == cue.ListKind:
		c.addFieldType(path, ListType)
		c.visitList(x, path)
	case k&cue.StructKind == cue.StructKind:
		c.addFieldType(path, StructType)
		c.visitStruct(x, path)
	default:
		log.Log.Info(fmt.Sprintf("cannot convert value of type %s", k.String()))
	}
}

func (c *cueTypeExtractor) visitStruct(v cue.Value, parentPath string) {
	joinFieldPath := func(path string, name string) string {
		if path == "" || strings.HasPrefix(path, "#") {
			return name
		}
		return path + "." + name
	}

	switch op, v := v.Expr(); op {
	// SelectorOp refer of other struct type
	case cue.NoOp, cue.SelectorOp:
		// pass
		// cue.NoOp describes the value is an underlying field.
		// cue.SelectorOp describes the value is a type reference field.
	default:
		// not support op, e.g. cue.Or, cue.And.
		log.Log.V(1).Info(fmt.Sprintf("cue type extractor unsupported op %v for object type (%v)", op, v))
		return
	}

	for itr, _ := v.Fields(cue.Optional(true), cue.Definitions(true)); itr.Next(); {
		name := itr.Label()
		c.visitValue(itr.Value(), joinFieldPath(parentPath, name))
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
		c.visitValue(i.Value(), path)
	}
}

func (c *cueTypeExtractor) addFieldType(fieldName string, cueType CueType) {
	c.fieldTypes[fieldName] = cueType
}

func (c *cueTypeExtractor) addFieldUnits(path string, t CueType, base string) {
	c.addFieldType(path, t)
	if t != IntType && base != "" {
		c.fieldUnits[path] = base
	}
}

func (c *cueTypeExtractor) hasFieldType(parent string, cur string) (string, bool) {
	fieldRef := cur
	if parent != "" {
		fieldRef = parent + "." + cur
	}
	if _, exist := c.fieldTypes[fieldRef]; exist {
		return fieldRef, true
	}
	if _, exist := c.fieldTypes[cur]; exist {
		return cur, true
	}
	return "", false
}

func transNumberOrBoolType(t CueType, obj reflect.Value, fn UpdateFn, expand string, trimString bool) error {
	switch t {
	case IntType:
		return processTypeTrans[int](obj, strconv.Atoi, fn, trimString)
	case BoolType:
		return processTypeTrans[bool](obj, strconv.ParseBool, fn, trimString)
	case FloatType:
		return processTypeTrans[float64](obj, func(s string) (float64, error) {
			return strconv.ParseFloat(s, 64)
		}, fn, trimString)
	case K8SQuantityType:
		return processTypeTrans[int64](obj, handleK8sQuantityType, fn, trimString)
	case ClassicStorageType:
		return processTypeTrans[int64](obj, handleClassicStorageType(expand), fn, trimString)
	case ClassicTimeDurationType:
		return processTypeTrans[int64](obj, handleClassicTimeDurationType(expand), fn, trimString)
	case StringType:
		if trimString {
			trimStringQuotes(obj, fn)
		}
	default:
		// pass
	}
	return nil
}

func trimStringQuotes(obj reflect.Value, fn UpdateFn) {
	if obj.Type().Kind() != reflect.String {
		return
	}
	str := obj.String()
	if !isQuotesString(str) {
		return
	}

	trimStr := strings.Trim(str, "'\"")
	if str != trimStr {
		fn(trimStr)
	}
}

func processTypeTrans[T int | int64 | float64 | float32 | bool](obj reflect.Value, transFn func(s string) (T, error), updateFn UpdateFn, trimString bool) error {
	switch obj.Type().Kind() {
	case reflect.String:
		str := obj.String()
		if trimString {
			str = strings.Trim(str, "'\"")
		}
		v, err := transFn(str)
		if err != nil {
			return err
		}
		updateFn(v)
	case reflect.Array, reflect.Slice, reflect.Struct:
		return MakeError("not support type[%s] trans.", obj.Type().Kind())
	}

	return nil
}

func processCfgNotStringParam(data interface{}, context *cue.Context, tpl cue.Value, trimString bool) error {
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
			fieldPath, exist := typeTransformer.hasFieldType(parent, cur)
			if !exist {
				return nil
			}
			return transNumberOrBoolType(typeTransformer.fieldTypes[fieldPath], obj, fn, typeTransformer.fieldUnits[fieldPath], trimString)
		}, false)
}
