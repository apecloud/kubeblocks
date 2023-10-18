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

package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestFlattenSchema(t *testing.T) {
	type args struct {
		src apiext.JSONSchemaProps
	}
	tests := []struct {
		name string
		args args
		want apiext.JSONSchemaProps
	}{{
		name: "normal test",
		args: args{
			src: apiext.JSONSchemaProps{
				Properties: map[string]apiext.JSONSchemaProps{
					"field1": {
						Type: "object",
						Properties: map[string]apiext.JSONSchemaProps{
							"abcd":   {Type: "string"},
							"test1":  {Type: "string", Format: "date-time"},
							"field2": {Type: "bool"},
						},
					},
					"field2": {
						Type: "object",
						Properties: map[string]apiext.JSONSchemaProps{
							"abcd": {Type: "string"},
							"test": {Type: "int"},
						},
					},
					"field3": {
						Type: "object",
						Properties: map[string]apiext.JSONSchemaProps{
							"abcd":   {Type: "int"},
							"field2": {Type: "bool"},
						},
					},
				},
			},
		},
		want: apiext.JSONSchemaProps{
			Properties: map[string]apiext.JSONSchemaProps{
				"field1.abcd":   {Type: "string"},
				"field1.test1":  {Type: "string", Format: "date-time"},
				"field1.field2": {Type: "bool"},
				"field2.abcd":   {Type: "string"},
				"field2.test":   {Type: "int"},
				"field3.abcd":   {Type: "int"},
				"field3.field2": {Type: "bool"},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FlattenSchema(tt.args.src), "FlattenSchema(%v)", tt.args.src)
		})
	}
}
