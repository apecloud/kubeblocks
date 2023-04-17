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
	"bytes"
	"reflect"
)

func compareWithConfig(left, right interface{}, option CfgOption) (bool, error) {
	switch option.Type {
	case CfgRawType:
		if !typeMatch([]byte{}, left, right) {
			return false, MakeError("invalid []byte data type!")
		}
		return bytes.Equal(left.([]byte), right.([]byte)), nil
	case CfgLocalType:
		if !typeMatch("", left, right) {
			return false, MakeError("invalid string data type!")
		}
		return left.(string) == right.(string), nil
	case CfgCmType, CfgTplType:
		if !typeMatch(&ConfigResource{}, left, right) {
			return false, MakeError("invalid data type!")
		}
		return left.(*ConfigResource) == right.(*ConfigResource), nil
	default:
		return false, MakeError("not support config type compare!")
	}
}

func withOption(option CfgOption, data interface{}) CfgOption {
	op := option
	switch option.Type {
	case CfgRawType:
		op.RawData = data.([]byte)
	case CfgLocalType:
		op.Path = data.(string)
	case CfgCmType, CfgTplType:
		op.ConfigResource = data.(*ConfigResource)
	}
	return op
}

func typeMatch(expected interface{}, values ...interface{}) bool {
	matcher := newMatcherWithType(expected)
	for _, v := range values {
		if ok, err := matcher.match(v); !ok || err != nil {
			return false
		}
	}
	return true
}

type typeMatcher struct {
	expected interface{}
}

func newMatcherWithType(expected interface{}) typeMatcher {
	return typeMatcher{
		expected: expected,
	}
}

func (matcher *typeMatcher) match(actual interface{}) (success bool, err error) {
	switch {
	case actual == nil && matcher.expected == nil:
		return false, MakeError("type is <nil> to <nil>.")
	case matcher.expected == nil:
		return false, MakeError("expected type is <nil>.")
	case actual == nil:
		return false, nil
	}

	actualType := reflect.TypeOf(actual)
	expectedType := reflect.TypeOf(matcher.expected)
	return actualType.AssignableTo(expectedType), nil
}
