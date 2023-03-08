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

package unstructured

import (
	"bytes"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/StudioSol/set"
	"github.com/spf13/cast"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type redisConfig struct {
	name string
	lex  *lexer

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
		lineNo = v.lineNo
		r.lex.remove(v)
	}
	keys = append(keys, value)
	t, err := r.lex.parseParameter(strings.Join(keys, " "), lineNo)
	if err == nil {
		r.lex.appendValidConfigParameter(t, lineNo)
	}
	return err
}

func matchSubKeys(keys []string, it item) bool {
	if len(keys) > len(it.values) {
		return false
	}

	for i, k := range keys {
		if it.values[i] != k {
			return false
		}
	}
	return true
}

func (r *redisConfig) Get(key string) interface{} {
	v, _ := r.GetString(key)
	return v
}

func (r *redisConfig) GetItem(keys []string) *item {
	v, ok := r.lex.dict[keys[0]]
	if !ok {
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
	for i := len(keys); i < len(item.values); i++ {
		v := item.values[i]
		if containerEscapeString(v) {
			res = append(res, strconv.Quote(v))
		} else {
			res = append(res, v)
		}
	}
	return strings.Join(res, " "), nil
}

func (r *redisConfig) GetAllParameters() map[string]interface{} {
	params := make(map[string]interface{})
	for key, param := range r.lex.dict {
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

func encodeMultiKeys(it item, prefix int) string {
	buffer := &bytes.Buffer{}
	for i := 0; i < prefix && i < len(it.values); i++ {
		if i > 0 {
			buffer.WriteByte(' ')
		}
		buffer.WriteString(it.values[i])
	}
	return buffer.String()
}

func encodeStringValue(param item, index int) string {
	buffer := &bytes.Buffer{}
	for i := index; i < len(param.values); i++ {
		v := param.values[i]
		if i > index {
			buffer.WriteByte(' ')
		}
		if v == "" || containerEscapeString(v) {
			v = strconv.Quote(v)
		}
		buffer.WriteString(v)
	}
	return buffer.String()
}

func uniqKeysParameters(its []item) int {
	maxPrefix := len(its[0].values)
	for i := 2; i < maxPrefix; i++ {
		if !hasPrefixKey(its, i) {
			return i
		}
	}
	return maxPrefix
}

func hasPrefixKey(its []item, prefix int) bool {
	keys := set.NewLinkedHashSetString()
	for _, i := range its {
		prefixKeys := strings.Join(i.values[0:prefix], " ")
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
	if len(r.lex.dict) == 0 {
		return "", nil
	}

	out := &bytes.Buffer{}
	for i, param := range sortParameters(r.lex) {
		if i > 0 {
			out.WriteByte('\n')
		}
		if err := encodeParamItem(param, out); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

func encodeParamItem(param item, out *bytes.Buffer) error {
	for _, co := range param.comments {
		out.WriteString(co)
		out.WriteByte('\n')
	}
	for i, v := range param.values {
		if i > 0 {
			out.WriteByte(' ')
		}
		if v == "" || containerEscapeString(v) {
			v = strconv.Quote(v)
		}
		out.WriteString(v)
	}
	return nil
}

func (r *redisConfig) Unmarshal(str string) error {
	r.lex = &lexer{
		dict: make(map[string][]item),
	}
	return r.lex.load(str)
}

func sortParameters(lex *lexer) []item {
	items := make([]item, 0)
	for _, v := range lex.dict {
		items = append(items, v...)
	}

	sort.SliceStable(items, func(i, j int) bool {
		no1 := items[i].lineNo
		no2 := items[j].lineNo
		if no1 == no2 {
			return strings.Compare(items[i].values[0], items[j].values[0]) < 0
		}
		return no1 < no2
	})
	return items
}
