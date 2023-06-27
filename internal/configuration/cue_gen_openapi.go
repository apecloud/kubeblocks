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

package configuration

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/openapi"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GenerateOpenAPISchema generates openapi schema from cue type Definitions.
func GenerateOpenAPISchema(cueTpl string, schemaType string) (*apiextv1.JSONSchemaProps, error) {
	const (
		openAPIVersion = "3.1.0"
	)

	cueOption := &load.Config{Stdin: strings.NewReader(cueTpl)}
	insts := load.Instances([]string{"-"}, cueOption)
	for _, ins := range insts {
		if err := ins.Err; err != nil {
			return nil, WrapError(err, "failed to generate build.Instance for %s", schemaType)
		}
	}
	if len(insts) != 1 {
		return nil, MakeError("failed to create cue.Instances. [%s]", cueTpl)
	}

	openapiOption := &openapi.Config{
		Version:       openAPIVersion,
		SelfContained: true,
		// ExpandReferences: true,
		Info: ast.NewStruct(
			"title", ast.NewString(fmt.Sprintf("%s configuration schema", schemaType)),
			"version", ast.NewString(openAPIVersion),
		),
	}
	// schema, err := openapiOption.All(cue.Build(insts)[0]) //nolint:staticcheck
	schema, err := openapiOption.Schemas(cue.Build(insts)[0]) //nolint:staticcheck
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return nil, nil
	}
	return transformOpenAPISchema(schema, schemaType)
}

func foundSchemaFromCueDefines(cueMap *openapi.OrderedMap, schemaType string) *openapi.OrderedMap {
	for _, kv := range cueMap.Pairs() {
		if schemaType == "" {
			m, ok := kv.Value.(*openapi.OrderedMap)
			if ok {
				return m
			}
			continue
		}
		if kv.Key == schemaType {
			return kv.Value.(*openapi.OrderedMap) //nolint:staticcheck
		}
	}
	return nil
}

func transformOpenAPISchema(cueSchema *openapi.OrderedMap, schemaType string) (*apiextv1.JSONSchemaProps, error) {
	allSchemaType := func(cueMap *openapi.OrderedMap) []string {
		keys := make([]string, len(cueMap.Elts))
		for i, pair := range cueMap.Pairs() {
			keys[i] = pair.Key
		}
		return keys
	}

	typeSchema := foundSchemaFromCueDefines(cueSchema, schemaType)
	if typeSchema == nil {
		log.Log.Info(fmt.Sprintf("not found schema type:[%s], all: %v", schemaType, allSchemaType(cueSchema)))
		return nil, nil
	}

	b, err := typeSchema.MarshalJSON()
	if err != nil {
		return nil, WrapError(err, "failed to marshal OpenAPI schema")
	}

	jsonProps := apiextv1.JSONSchemaProps{}
	if err = json.Unmarshal(b, &jsonProps); err != nil {
		log.Log.Error(err, "failed to unmarshal raw OpenAPI schema to JSONSchemaProps")
		return nil, err
	}

	r := apiextv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextv1.JSONSchemaProps{
			"spec": jsonProps,
		},
	}
	return &r, nil
}
