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

package v1alpha1

import (
	"testing"

	"github.com/go-test/deep"
)

var testData = ConfigParams{
	ConfigItem: map[string]interface{}{
		"b":    true,
		"list": []interface{}{"abcd", 2, "test"},
		"map": map[string]interface{}{
			"m": map[string]interface{}{
				"float": 66.66,
				"int":   100,
			},
			"d": 1,
		},
		"field1.field2.field3": "my_test",
	},
}

var expectedConfigParams = ConfigParams{
	ConfigItem: map[string]interface{}{
		"b":    true,
		"list": []interface{}{"abcd", float64(2), "test"},
		"map": map[string]interface{}{
			"m": map[string]interface{}{
				"float": 66.66,
				"int":   float64(100),
			},
			"d": float64(1),
		},
		"field1.field2.field3": "my_test",
	},
}

func TestConfigParams_DeepCopyInto(t *testing.T) {
	tests := []struct {
		name     string
		in       ConfigParams
		expected ConfigParams
	}{{
		name:     "test",
		in:       testData,
		expected: expectedConfigParams,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out ConfigParams
			tt.in.DeepCopyInto(&out)
			if diff := deep.Equal(out, tt.expected); diff != nil {
				t.Error(diff)
			}
		})
	}
}
