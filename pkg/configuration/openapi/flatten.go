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
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const schemaFieldDelim = "."

// FlattenSchema flattens the given schema to a single level.
func FlattenSchema(src apiextv1.JSONSchemaProps) apiextv1.JSONSchemaProps {
	flattenMap := make(map[string]apiextv1.JSONSchemaProps)
	return apiextv1.JSONSchemaProps{
		Properties: flattenSchemaProps(flattenMap, src, "", schemaFieldDelim),
	}
}

func flattenSchemaProps(shadow map[string]apiextv1.JSONSchemaProps, m apiextv1.JSONSchemaProps, prefix string, delim string) map[string]apiextv1.JSONSchemaProps {
	if prefix != "" {
		prefix += delim
	}
	for k, val := range m.Properties {
		fullKey := prefix + k
		if val.Properties != nil {
			// recursively merge to shadow map
			shadow = flattenSchemaProps(shadow, val, fullKey, delim)
		} else {
			shadow[fullKey] = val
		}
	}
	return shadow
}
