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

package openapi

import (
	"math"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	SchemaFieldDelim      = "."
	SchemaMapFieldKeyName = "*"
)

// FlattenSchema flattens the given schema to a single level.
func FlattenSchema(src apiextv1.JSONSchemaProps) apiextv1.JSONSchemaProps {
	flattenMap := make(map[string]apiextv1.JSONSchemaProps)
	flattenSchemaProps(flattenMap, src, "", SchemaFieldDelim)
	return apiextv1.JSONSchemaProps{Properties: flattenMap}
}

func genFieldPrefix(prefix, delim, key string) string {
	if prefix != "" {
		prefix += delim
	}
	return prefix + key
}

func flattenSchemaAdditionalProps(flattenProps map[string]apiextv1.JSONSchemaProps, m apiextv1.JSONSchemaProps, prefix string, delim string) {
	if m.AdditionalProperties != nil && m.AdditionalProperties.Schema != nil {
		flattenSchemaProps(flattenProps, *m.AdditionalProperties.Schema, genFieldPrefix(prefix, delim, SchemaMapFieldKeyName), delim)
	}
}

func addFlattenedSchema(flattenProps map[string]apiextv1.JSONSchemaProps, prefix string, schema apiextv1.JSONSchemaProps) {
	existing, ok := flattenProps[prefix]
	if !ok {
		flattenProps[prefix] = schema
		return
	}
	intersectFlattenedSchemaType(&existing, schema)
	intersectFlattenedSchemaConversionHints(&existing, schema)
	existing.AllOf = append(existing.AllOf, schema)
	flattenProps[prefix] = existing
}

func intersectFlattenedSchemaType(existing *apiextv1.JSONSchemaProps, schema apiextv1.JSONSchemaProps) {
	switch {
	case existing.Type == "":
		existing.Type = schema.Type
		existing.Format = schema.Format
	case schema.Type == "" || existing.Type == schema.Type:
		return
	case existing.Type == "number" && schema.Type == "integer":
		existing.Type = schema.Type
		existing.Format = schema.Format
	case existing.Type == "integer" && schema.Type == "number":
		return
	}
}

func intersectFlattenedSchemaConversionHints(existing *apiextv1.JSONSchemaProps, schema apiextv1.JSONSchemaProps) {
	if existing.Type != "integer" {
		return
	}
	intersectFlattenedSchemaIntegerFormatBounds(existing)
	intersectFlattenedSchemaIntegerFormatBounds(&schema)
	intersectFlattenedSchemaFormat(existing, schema)
	intersectFlattenedSchemaMinimum(existing, schema)
	intersectFlattenedSchemaMaximum(existing, schema)
}

func intersectFlattenedSchemaIntegerFormatBounds(schema *apiextv1.JSONSchemaProps) {
	var minimum, maximum float64
	var exclusiveMaximum bool
	switch schema.Format {
	case "int8":
		minimum, maximum = -128, 127
	case "int16":
		minimum, maximum = -32768, 32767
	case "int32":
		minimum, maximum = -2147483648, 2147483647
	case "int64":
		minimum, maximum = -math.Exp2(63), math.Exp2(63)
		exclusiveMaximum = true
	case "uint8":
		minimum, maximum = 0, 255
	case "uint16":
		minimum, maximum = 0, 65535
	case "uint32":
		minimum, maximum = 0, 4294967295
	case "uint64":
		minimum, maximum = 0, math.Exp2(64)
		exclusiveMaximum = true
	case "uint":
		minimum = 0
		intersectFlattenedSchemaMinimum(schema, apiextv1.JSONSchemaProps{Minimum: &minimum})
		return
	default:
		return
	}
	intersectFlattenedSchemaMinimum(schema, apiextv1.JSONSchemaProps{Minimum: &minimum})
	intersectFlattenedSchemaMaximum(schema, apiextv1.JSONSchemaProps{
		Maximum:          &maximum,
		ExclusiveMaximum: exclusiveMaximum,
	})
}

func intersectFlattenedSchemaFormat(existing *apiextv1.JSONSchemaProps, schema apiextv1.JSONSchemaProps) {
	if isSignedIntegerFormat(existing.Format) {
		return
	}
	if isSignedIntegerFormat(schema.Format) {
		existing.Format = schema.Format
		return
	}
	if isUnsignedIntegerFormat(existing.Format) {
		return
	}
	if existing.Format == "" || isUnsignedIntegerFormat(schema.Format) {
		existing.Format = schema.Format
	}
}

func isSignedIntegerFormat(format string) bool {
	switch format {
	case "int", "int8", "int16", "int32", "int64":
		return true
	default:
		return false
	}
}

func isUnsignedIntegerFormat(format string) bool {
	switch format {
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func intersectFlattenedSchemaMinimum(existing *apiextv1.JSONSchemaProps, schema apiextv1.JSONSchemaProps) {
	if schema.Minimum == nil {
		return
	}
	if existing.Minimum == nil || *schema.Minimum > *existing.Minimum {
		minimum := *schema.Minimum
		existing.Minimum = &minimum
		existing.ExclusiveMinimum = schema.ExclusiveMinimum
		return
	}
	if *schema.Minimum == *existing.Minimum {
		existing.ExclusiveMinimum = existing.ExclusiveMinimum || schema.ExclusiveMinimum
	}
}

func intersectFlattenedSchemaMaximum(existing *apiextv1.JSONSchemaProps, schema apiextv1.JSONSchemaProps) {
	if schema.Maximum == nil {
		return
	}
	if existing.Maximum == nil || *schema.Maximum < *existing.Maximum {
		maximum := *schema.Maximum
		existing.Maximum = &maximum
		existing.ExclusiveMaximum = schema.ExclusiveMaximum
		return
	}
	if *schema.Maximum == *existing.Maximum {
		existing.ExclusiveMaximum = existing.ExclusiveMaximum || schema.ExclusiveMaximum
	}
}

func flattenSchemaProps(flattenProps map[string]apiextv1.JSONSchemaProps, m apiextv1.JSONSchemaProps, prefix string, delim string) {
	if m.Type != SchemaStructType && m.Type != "" {
		addFlattenedSchema(flattenProps, prefix, m)
		return
	}

	flattenSchemaAdditionalProps(flattenProps, m, prefix, delim)
	for k, val := range m.Properties {
		flattenSchemaProps(flattenProps, val, genFieldPrefix(prefix, delim, k), delim)
	}
	for _, schema := range m.AllOf {
		flattenSchemaProps(flattenProps, schema, prefix, delim)
	}
}
