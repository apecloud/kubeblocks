/*
Copyright ApeCloud, Inc.

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
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/openapi"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func GenerateOpenAPISchema(cueTpl string, schemaType string) (*apiextv1.JSONSchemaProps, error) {
	const (
		OpenAPIVersion = "3.1.0"
	)

	cfg := &load.Config{
		Stdin: strings.NewReader(cueTpl),
	}

	insts := load.Instances([]string{"-"}, cfg)
	for _, ins := range insts {
		if err := ins.Err; err != nil {
			return nil, WrapError(err, "failed to generate build.Instance for %s", schemaType)
		}
	}
	if len(insts) != 1 {
		return nil, MakeError("failed to create cue.Instances. [%s]", cueTpl)
	}

	openapicfg := &openapi.Config{
		Version:       OpenAPIVersion,
		SelfContained: true,
		// ExpandReferences: true,
		Info: ast.NewStruct(
			"title", ast.NewString(fmt.Sprintf("%s configuration schema", schemaType)),
			"version", ast.NewString(OpenAPIVersion),
		),
	}

	// schema, err := openapicfg.All(cue.Build(insts)[0]) //nolint:staticcheck
	schema, err := openapicfg.Schemas(cue.Build(insts)[0]) //nolint:staticcheck
	if err != nil {
		return nil, err
	}

	var (
		typeSchema *openapi.OrderedMap //nolint:staticcheck
		all        = make([]string, len(schema.Elts))
	)

	for _, kv := range schema.Pairs() {
		all = append(all, kv.Key)
		if kv.Key == schemaType || schemaType == "" {
			typeSchema = kv.Value.(*openapi.OrderedMap) //nolint:staticcheck
			break
		}
	}

	if typeSchema == nil {
		log.Log.Info(fmt.Sprintf("Cannot found schema. type:[%s], all: %s", schemaType, all))
		return nil, nil
	}

	b, err := typeSchema.MarshalJSON()
	if err != nil {
		return nil, WrapError(err, "failed to marshal OpenAPI schema")
	}

	j := &apiextv1.JSONSchemaProps{}
	if err = json.Unmarshal(b, j); err != nil {
		log.Log.Info(fmt.Sprintf("Cannot unmarshal raw OpenAPI schema to JSONSchemaProps: %v", err))
	}

	return &apiextv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextv1.JSONSchemaProps{
			"spec": *j,
		},
	}, nil
}
