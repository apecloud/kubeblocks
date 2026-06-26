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
	"math"
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
			"max_queries": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: ptrFloat64(float64(math.MaxUint64)),
			},
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
		t.Fatalf("expected parse error for negative value with uint64 bounds")
	}

	got, err = ConvertStringToInterfaceBySchemaType(schema, map[string]string{"max_queries": "9223372036854775808"})
	if err != nil {
		t.Fatalf("expected value above MaxInt64 to parse as uint64, got %v", err)
	}
	if got["max_queries"] != uint64(9223372036854775808) {
		t.Fatalf("expected uint64(9223372036854775808), got %T(%v)", got["max_queries"], got["max_queries"])
	}
}

func TestValidateDataWithSchemaUint64(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"max_queries": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: ptrFloat64(float64(math.MaxUint64)),
			},
		},
	}

	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_queries": uint64(50)}); err != nil {
		t.Fatalf("expected uint64(50) to pass validation, got %v", err)
	}
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_queries": uint64(math.MaxInt64) + 1}); err != nil {
		t.Fatalf("expected uint64 above MaxInt64 to pass validation, got %v", err)
	}
}

func TestValidateDataWithSchemaInt64(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"count": {
				Type:    "integer",
				Minimum: ptrFloat64(float64(-math.MaxInt64)),
				Maximum: ptrFloat64(float64(math.MaxInt64)),
			},
		},
	}

	if err := ValidateDataWithSchema(schema, map[string]interface{}{"count": int64(50)}); err != nil {
		t.Fatalf("expected int64(50) to pass validation, got %v", err)
	}
}

func TestStripIntegerOverflow(t *testing.T) {
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"u64_field": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: ptrFloat64(float64(math.MaxUint64)),
			},
			"i64_field": {
				Type:    "integer",
				Maximum: ptrFloat64(100),
			},
			"i64_cue": {
				Type:    "integer",
				Minimum: ptrFloat64(float64(-math.MaxInt64)),
				Maximum: ptrFloat64(float64(math.MaxInt64)),
			},
			"custom_small": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: ptrFloat64(1000),
			},
		},
	}

	result := stripIntegerOverflow(schema)
	if result.Properties["u64_field"].Maximum != nil {
		t.Fatalf("expected u64_field Maximum to be stripped (overflow)")
	}
	if result.Properties["i64_field"].Maximum == nil || *result.Properties["i64_field"].Maximum != 100 {
		t.Fatalf("expected i64_field Maximum to be preserved")
	}
	if result.Properties["i64_cue"].Maximum != nil {
		t.Fatalf("expected i64_cue Maximum to be stripped (float64(MaxInt64) rounds to 2^63)")
	}
	if result.Properties["custom_small"].Maximum == nil || *result.Properties["custom_small"].Maximum != 1000 {
		t.Fatalf("expected custom_small Maximum to be preserved (below 2^63)")
	}
	if schema.Properties["u64_field"].Maximum == nil {
		t.Fatalf("expected original schema to be unchanged")
	}
}

func TestValidateLargeIntegerBounds(t *testing.T) {
	// User-declared maximum 1e19 (not a CUE extremum) must be enforced
	userMax := float64(1e19)
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"custom_max": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: &userMax,
			},
		},
	}

	// Value within user-declared max should pass
	if err := validateLargeIntegerBounds(schema, map[string]interface{}{"custom_max": uint64(9e18)}); err != nil {
		t.Fatalf("expected value within user max to pass, got %v", err)
	}
	// Value above user-declared max should fail
	if err := validateLargeIntegerBounds(schema, map[string]interface{}{"custom_max": uint64(11e18)}); err == nil {
		t.Fatalf("expected value above user max to be rejected")
	}

	// CUE uint64 extremum (2^64) should be skipped (not enforced manually)
	cueSchema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"cue_u64": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: ptrFloat64(math.Exp2(64)),
			},
		},
	}
	if err := validateLargeIntegerBounds(cueSchema, map[string]interface{}{"cue_u64": uint64(math.MaxUint64)}); err != nil {
		t.Fatalf("expected CUE uint64 extremum to be skipped, got %v", err)
	}

	// CUE int64 extremum (2^63) should be skipped
	cueI64Schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"cue_i64": {
				Type:    "integer",
				Minimum: ptrFloat64(-math.Exp2(63)),
				Maximum: ptrFloat64(math.Exp2(63)),
			},
		},
	}
	if err := validateLargeIntegerBounds(cueI64Schema, map[string]interface{}{"cue_i64": int64(math.MaxInt64)}); err != nil {
		t.Fatalf("expected CUE int64 extremum to be skipped, got %v", err)
	}

	// float64 value (JSON/YAML parser output) within user max should pass
	if err := validateLargeIntegerBounds(schema, map[string]interface{}{"custom_max": float64(9e18)}); err != nil {
		t.Fatalf("expected float64 value within user max to pass, got %v", err)
	}
	// float64 value above user max should fail
	if err := validateLargeIntegerBounds(schema, map[string]interface{}{"custom_max": float64(1.1e19)}); err == nil {
		t.Fatalf("expected float64 value above user max to be rejected")
	}
	// int value (YAML parser output) within user max should pass
	if err := validateLargeIntegerBounds(schema, map[string]interface{}{"custom_max": int(100)}); err != nil {
		t.Fatalf("expected int value within user max to pass, got %v", err)
	}
}

func TestValidateDataWithSchemaUserDeclaredLargeMax(t *testing.T) {
	userMax := float64(1e19)
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"max_items": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: &userMax,
			},
		},
	}

	// Value within user max passes end-to-end
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_items": uint64(9e18)}); err != nil {
		t.Fatalf("expected value within user max to pass, got %v", err)
	}
	// Value above user max fails end-to-end
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_items": uint64(11e18)}); err == nil {
		t.Fatalf("expected value above user-declared max to be rejected")
	}
}

func TestValidateDataWithSchemaIntTypeLargeMax(t *testing.T) {
	userMax := float64(1e19)
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"max_items": {
				Type:    "integer",
				Minimum: ptrFloat64(0),
				Maximum: &userMax,
			},
		},
	}

	// int value within user max passes end-to-end (YAML parser produces int)
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_items": int(100)}); err != nil {
		t.Fatalf("expected int value within user max to pass, got %v", err)
	}
	// int32 value passes end-to-end
	if err := ValidateDataWithSchema(schema, map[string]interface{}{"max_items": int32(100)}); err != nil {
		t.Fatalf("expected int32 value within user max to pass, got %v", err)
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
