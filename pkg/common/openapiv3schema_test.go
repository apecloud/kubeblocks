/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package common

import (
	"reflect"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestValidateDataWithSchema(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type:     "object",
		Required: []string{"replicas"},
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"replicas": {Type: "integer", Minimum: ptrFloat64(1)},
		},
	}

	if err := ValidateDataWithSchema(schema, map[string]interface{}{"replicas": int64(3)}); err != nil {
		t.Fatalf("expected valid data, got %v", err)
	}
	if err := ValidateDataWithSchema(nil, map[string]interface{}{}); err == nil {
		t.Fatalf("expected nil schema error")
	}
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"replicas": int64(0)}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestConvertStringToInterfaceBySchemaType(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"replicas": {Type: "integer"},
			"ratio":    {Type: "number"},
			"enabled":  {Type: "boolean"},
			"items":    {Type: "array"},
			"name":     {Type: "string"},
		},
	}

	got, err := ConvertStringToInterfaceBySchemaType(schema, map[string]string{
		"replicas": "3",
		"ratio":    "1.5",
		"enabled":  "true",
		"items":    "a,b,c",
		"name":     "mysql",
		"ignored":  "value",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]interface{}{
		"replicas": int64(3),
		"ratio":    1.5,
		"enabled":  true,
		"items":    []string{"a", "b", "c"},
		"name":     "mysql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}

	_, err = ConvertStringToInterfaceBySchemaType(schema, map[string]string{"replicas": "bad"})
	if err == nil || !strings.Contains(err.Error(), `convert "replicas" failed`) {
		t.Fatalf("expected conversion error for replicas, got %v", err)
	}
}

func TestConvertStringToInterfaceBySchemaTypeUint64(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"max_queries": {Type: "integer", Format: "uint64"},
		},
	}

	got, err := ConvertStringToInterfaceBySchemaType(schema, map[string]string{"max_queries": "50"})
	if err != nil {
		t.Fatalf("expected uint64 parse to succeed, got %v", err)
	}
	if got["max_queries"] != uint64(50) {
		t.Fatalf("expected uint64(50), got %T(%v)", got["max_queries"], got["max_queries"])
	}

	_, err = ConvertStringToInterfaceBySchemaType(schema, map[string]string{"max_queries": "-1"})
	if err == nil {
		t.Fatalf("expected parse error for negative value with uint64 format")
	}
}

func TestValidateDataWithSchemaUint64(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"max_queries": {Type: "integer", Format: "uint64", Minimum: ptrFloat64(0)},
		},
	}

	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_queries": uint64(50)}); err != nil {
		t.Fatalf("expected uint64(50) to pass validation, got %v", err)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
