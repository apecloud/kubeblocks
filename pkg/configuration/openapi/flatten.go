/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

func flattenSchemaProps(flattenProps map[string]apiextv1.JSONSchemaProps, m apiextv1.JSONSchemaProps, prefix string, delim string) {
	if m.Type != SchemaStructType && m.Type != "" {
		flattenProps[prefix] = m
		return
	}

	flattenSchemaAdditionalProps(flattenProps, m, prefix, delim)
	for k, val := range m.Properties {
		flattenSchemaProps(flattenProps, val, genFieldPrefix(prefix, delim, k), delim)
	}
}
