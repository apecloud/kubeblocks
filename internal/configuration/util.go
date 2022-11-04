/*
Copyright 2022.

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
	"bytes"
	"reflect"
)

func compareWithConfig(left, right interface{}, option CfgOption) (bool, error) {
	switch option.Type {
	case CFG_RAW:
		if !TypeMatch([]byte{}, left, right) {
			return false, makeError("invalid []byte data type!")
		}
		return bytes.Equal(left.([]byte), right.([]byte)), nil
	case CFG_LOCAL:
		if !TypeMatch(string(""), left, right) {
			return false, makeError("invalid string data type!")
		}
		return left.(string) == right.(string), nil
	case CFG_CM, CFG_TPL:
		if !TypeMatch(&K8sConfig{}, left, right) {
			return false, makeError("invalid data type!")
		}
		return left.(*K8sConfig) == right.(*K8sConfig), nil
	default:
		return false, makeError("not support config type compare!")
	}
}

func withOption(option CfgOption, data interface{}) CfgOption {
	op := option
	switch option.Type {
	case CFG_RAW:
		op.RawData = data.([]byte)
	case CFG_LOCAL:
		op.Path = data.(string)
	case CFG_CM, CFG_TPL:
		op.K8sKey = data.(*K8sConfig)
	default:
		// TODO(zt) process error
	}
	return op
}

func TypeMatch(expected interface{}, values ...interface{}) bool {
	matcher := NewMatcherWithType(expected)

	for _, v := range values {
		if ok, err := matcher.Match(v); !ok || err != nil {
			return false
		}
	}
	return true
}

type typeMatcher struct {
	expected interface{}
}

func NewMatcherWithType(expected interface{}) typeMatcher {
	return typeMatcher{
		expected: expected,
	}
}

func (matcher *typeMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && matcher.expected == nil {
		return false, makeError("type is <nil> to <nil>.")
	} else if matcher.expected == nil {
		return false, makeError("expected type is <nil>.")
	} else if actual == nil {
		return false, nil
	}

	actualType := reflect.TypeOf(actual)
	expectedType := reflect.TypeOf(matcher.expected)
	return actualType.AssignableTo(expectedType), nil
}
