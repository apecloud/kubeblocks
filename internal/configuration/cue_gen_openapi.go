/*
Copyright 2022.

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
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/openapi"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func GenerateOpenApiSchema(cueTpl string, schemaType string) (*apiextv1.JSONSchemaProps, error) {
	cfg := &load.Config{
		Stdin: strings.NewReader(cueTpl),
	}

	insts := load.Instances([]string{"-"}, cfg)
	if len(insts) != 1 {
		return nil, MakeError("failed to create cue.Instances. [%s]", cueTpl)
	}

	openapicfg := openapi.Config{
		// ExpandReferences: true,
		Info: ast.NewStruct(
			"title", ast.NewString(fmt.Sprintf("%s configuration schema", schemaType)),
			"version", ast.NewString("v1alpha1"),
		),
	}

	schema, err := openapicfg.Schemas(cue.Build(insts)[0])
	if err != nil {
		return nil, err
	}

	var (
		typeSchema *openapi.OrderedMap
		all        = make([]string, len(schema.Elts))
	)

	for _, kv := range schema.Pairs() {
		all = append(all, kv.Key)
		if kv.Key == schemaType || schemaType == "" {
			typeSchema = kv.Value.(*openapi.OrderedMap)
			break
		}
	}

	if typeSchema == nil {
		return nil, MakeError("not found schema type:[%s], all: %s", schemaType, all)
	}

	b, err := typeSchema.MarshalJSON()
	if err != nil {
		return nil, WrapError(err, "failed to marshal OpenAPI schema")
	}

	j := &apiextv1.JSONSchemaProps{}
	if err = json.Unmarshal(b, j); err != nil {
		log.Fatalf("Cannot unmarshal raw OpenAPI schema to JSONSchemaProps: %v", err)
	}

	return &apiextv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextv1.JSONSchemaProps{
			"spec": *j,
		},
	}, nil
}
