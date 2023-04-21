/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
