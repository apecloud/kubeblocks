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

package unstructured

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	"github.com/StudioSol/set"
	"github.com/spf13/cast"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/unstructured/redis"
)

type redisConfig struct {
	name string
	lex  *redis.Lexer

	// parameters map[string]string
}

func init() {
	CfgObjectRegistry().RegisterConfigCreator(appsv1alpha1.RedisCfg, func(name string) ConfigObject {
		return &redisConfig{name: name}
	})
}

func (r *redisConfig) Update(key string, value any) error {
	return r.setString(key, cast.ToString(value))
}

func (r *redisConfig) setString(key string, value string) error {
	keys := strings.Split(key, ".")
	v := r.GetItem(keys)
	lineNo := math.MaxInt32
	if v != nil {
		lineNo = v.LineNo
		r.lex.RemoveParameter(v)
	}
	keys = append(keys, value)
	t, err := r.lex.ParseParameter(strings.Join(keys, " "), lineNo)
	if err == nil {
		if v != nil {
			t.Comments = v.Comments
		}
		r.lex.AppendValidParameter(t, lineNo)
	}
	return err
}

func matchSubKeys(keys []string, it redis.Item) bool {
	if len(keys) > len(it.Values) {
		return false
	}

	for i, k := range keys {
		if it.Values[i] != k {
			return false
		}
	}
	return true
}

func (r *redisConfig) Get(key string) interface{} {
	v, _ := r.GetString(key)
	return v
}

func (r *redisConfig) GetItem(keys []string) *redis.Item {
	v := r.lex.GetItem(keys[0])
	if v == nil {
		return nil
	}

	if len(keys) == 1 && len(v) != 1 {
		return nil
	} else if len(keys) == 1 {
		return &v[0]
	}

	for i := range v {
		if matchSubKeys(keys, v[i]) {
			return &v[i]
		}
	}
	return nil
}

func (r *redisConfig) GetString(key string) (string, error) {
	keys := strings.Split(key, ".")
	item := r.GetItem(keys)
	if item == nil {
		return "", nil
	}

	res := make([]string, 0)
	for i := len(keys); i < len(item.Values); i++ {
		v := item.Values[i]
		if redis.ContainerEscapeString(v) {
			res = append(res, strconv.Quote(v))
		} else {
			res = append(res, v)
		}
	}
	return strings.Join(res, " "), nil
}

func (r *redisConfig) GetAllParameters() map[string]interface{} {
	params := make(map[string]interface{})
	for key, param := range r.lex.GetAllParams() {
		if len(param) == 0 {
			continue
		}
		if len(param) == 1 {
			params[key] = encodeStringValue(param[0], 1)
			continue
		}
		prefix := uniqKeysParameters(param)
		for _, i := range param {
			params[encodeMultiKeys(i, prefix)] = encodeStringValue(i, prefix)
		}
	}
	return params
}

func encodeMultiKeys(it redis.Item, prefix int) string {
	buffer := &bytes.Buffer{}
	for i := 0; i < prefix && i < len(it.Values); i++ {
		if i > 0 {
			buffer.WriteByte(' ')
		}
		buffer.WriteString(it.Values[i])
	}
	return buffer.String()
}

func encodeStringValue(param redis.Item, index int) string {
	buffer := &bytes.Buffer{}
	for i := index; i < len(param.Values); i++ {
		v := param.Values[i]
		if i > index {
			buffer.WriteByte(' ')
		}
		if v == "" || redis.ContainerEscapeString(v) {
			v = strconv.Quote(v)
		}
		buffer.WriteString(v)
	}
	return buffer.String()
}

func uniqKeysParameters(its []redis.Item) int {
	maxPrefix := len(its[0].Values)
	for i := 2; i < maxPrefix; i++ {
		if !hasPrefixKey(its, i) {
			return i
		}
	}
	return maxPrefix
}

func hasPrefixKey(its []redis.Item, prefix int) bool {
	keys := set.NewLinkedHashSetString()
	for _, i := range its {
		prefixKeys := strings.Join(i.Values[0:prefix], " ")
		if keys.InArray(prefixKeys) {
			return true
		}
		keys.Add(prefixKeys)
	}
	return false
}

func (r *redisConfig) SubConfig(key string) ConfigObject {
	return nil
}

func (r *redisConfig) Marshal() (string, error) {
	if r.lex.Empty() {
		return "", nil
	}
	if !r.lex.IsUpdated() {
		return r.lex.ToString(), nil
	}

	out := &bytes.Buffer{}
	for i, param := range r.lex.SortParameters() {
		if i > 0 {
			out.WriteByte('\n')
		}
		if err := encodeParamItem(param, out); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func encodeParamItem(param redis.Item, out *bytes.Buffer) error {
	for _, co := range param.Comments {
		out.WriteString(co)
		out.WriteByte('\n')
	}
	for i, v := range param.Values {
		if i > 0 {
			out.WriteByte(' ')
		}
		if v == "" || redis.ContainerEscapeString(v) {
			v = strconv.Quote(v)
		}
		out.WriteString(v)
	}
	return nil
}

func (r *redisConfig) Unmarshal(str string) error {
	r.lex = &redis.Lexer{}
	return r.lex.Load(str)
}
