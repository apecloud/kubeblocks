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

package validate

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
	// SelectorOp refers to other struct type
	case cue.NoOp, cue.SelectorOp:
		// pass
		// cue.NoOp: the value is an underlying field.
		// cue.SelectorOp: the value is a type reference field.
	default:
		// not support op, e.g. cue.Or, cue.And.
		log.Log.V(1).Info(fmt.Sprintf("cue type extractor does not support op %v for object type (%v)", op, v))
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
		log.Log.Info(fmt.Sprintf("not supported op %v for object type (%v)", op, v))
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

func transNumberOrBoolType(t CueType, obj reflect.Value, fn util.UpdateFn, expand string, trimString bool) error {
	switch t {
	case IntType:
		return processTypeTrans[int](obj, strconv.Atoi, fn, trimString, false)
	case BoolType:
		return processTypeTrans[bool](obj, strconv.ParseBool, fn, trimString, false)
	case FloatType:
		return processTypeTrans[float64](obj, func(s string) (float64, error) {
			return strconv.ParseFloat(s, 64)
		}, fn, trimString, false)
	case K8SQuantityType:
		return processTypeTrans[int64](obj, handleK8sQuantityType, fn, trimString, true)
	case ClassicStorageType:
		return processTypeTrans[int64](obj, handleClassicStorageType(expand), fn, trimString, true)
	case ClassicTimeDurationType:
		return processTypeTrans[int64](obj, handleClassicTimeDurationType(expand), fn, trimString, true)
	case StringType:
		if trimString {
			trimStringQuotes(obj, fn)
		}
	default:
		// pass
	}
	return nil
}

func trimStringQuotes(obj reflect.Value, fn util.UpdateFn) {
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

func processTypeTrans[T int | int64 | float64 | float32 | bool](obj reflect.Value, transFn func(s string) (T, error), updateFn util.UpdateFn, trimString bool, enableEmpty bool) error {
	switch obj.Type().Kind() {
	case reflect.String:
		str := obj.String()
		if trimString {
			str = strings.Trim(str, "'\"")
		}
		if !enableEmpty && str == "" {
			updateFn(nil)
			return nil
		}
		v, err := transFn(str)
		if err != nil {
			return err
		}
		updateFn(v)
	case reflect.Array, reflect.Slice, reflect.Struct:
		return core.MakeError("not supported type[%s] trans.", obj.Type().Kind())
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
	return util.UnstructuredObjectWalk(typeTransformer.data,
		func(parent, cur string, obj reflect.Value, fn util.UpdateFn) error {
			if fn == nil || cur == "" || !obj.IsValid() {
				return nil
			}
			fieldPath, exist := typeTransformer.hasFieldType(parent, cur)
			if !exist {
				return nil
			}
			err := transNumberOrBoolType(typeTransformer.fieldTypes[fieldPath], obj, fn, typeTransformer.fieldUnits[fieldPath], trimString)
			if err != nil {
				return core.WrapError(err, "failed to parse field %s", fieldPath)
			}
			return nil
		}, false)
}
