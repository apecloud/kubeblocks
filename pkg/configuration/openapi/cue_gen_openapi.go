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
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/encoding/openapi"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
)

const (
	SchemaFieldName    = "schemas"
	ComponentFieldName = "components"
	DefaultSchemaName  = "spec"
	SchemaStructType   = "object"
)

// GenerateOpenAPISchema generates openapi schema from cue type Definitions.
func GenerateOpenAPISchema(cueTpl string, schemaType string) (*apiextv1.JSONSchemaProps, error) {
	const (
		rootPath       = "root"
		openAPIVersion = "3.1.0"
	)

	rt, err := NewRuntime(cueTpl)
	if err != nil {
		return nil, err
	}

	var rootValue cue.Value
	if schemaType != "" {
		rootValue = rt.Underlying().LookupPath(cue.MakePath(cue.Def(schemaType)))
		if rootValue.Err() != nil {
			return nil, core.MakeError(errors.Details(rootValue.Err(), nil))
		}
	} else {
		defs, err := rt.Underlying().Fields(cue.Definitions(true))
		if err != nil {
			return nil, err
		}
		if !defs.Next() {
			return nil, core.MakeError("no definitions found")
		}
		schemaType = strings.Trim(defs.Label(), "#")
		rootValue = defs.Value()
	}

	v := rt.Context().CompileString(fmt.Sprintf("#%s", schemaType))
	defPath := cue.MakePath(cue.Def(schemaType))
	defSche := v.FillPath(defPath, rootValue)
	openapiOption := &openapi.Config{
		Version:          openAPIVersion,
		SelfContained:    true,
		ExpandReferences: false,
		Info: ast.NewStruct(
			"title", ast.NewString(fmt.Sprintf("%s configuration schema", schemaType)),
			"version", ast.NewString(openAPIVersion),
		),
	}

	astf, err := openapi.Generate(defSche.Eval(), openapiOption)
	if err != nil {
		return nil, err
	}
	schema, err := getSchemas(astf, schemaType)
	if err != nil {
		return nil, err
	}

	return transformOpenAPISchema(rt, schema, DeReference(astf, rt))
}

func getSchemas(f *ast.File, schemaType string) ([]ast.Decl, error) {
	compos, err := GetFieldByLabel(f, ComponentFieldName)
	if err != nil {
		return nil, err
	}
	schemas, err := GetFieldByLabel(compos.Value, SchemaFieldName)
	if err != nil {
		return nil, err
	}
	typeSchema, err := GetFieldByLabel(schemas.Value, schemaType)
	if err != nil {
		return nil, err
	}
	slit, ok := typeSchema.Value.(*ast.StructLit)
	if ok {
		return slit.Elts, nil
	}
	return nil, core.MakeError("not a struct literal")
}

func transformOpenAPISchema(rt *Runtime, cueSchema []ast.Decl, resolveFn func(path string) (*apiextv1.JSONSchemaProps, error)) (*apiextv1.JSONSchemaProps, error) {
	jsonProps, err := FromCueAST(rt, cueSchema)
	if err != nil {
		return nil, err
	}

	if err := deReferenceSchema(jsonProps, resolveFn); err != nil {
		return nil, err
	}
	r := apiextv1.JSONSchemaProps{
		Type: SchemaStructType,
		Properties: map[string]apiextv1.JSONSchemaProps{
			DefaultSchemaName: *jsonProps,
		},
	}
	return &r, nil
}

func deReferenceSchema(props *apiextv1.JSONSchemaProps, resolveFn func(path string) (*apiextv1.JSONSchemaProps, error)) (err error) {
	resolve := func(props *apiextv1.JSONSchemaProps) (*apiextv1.JSONSchemaProps, error) {
		if props.Ref == nil {
			return props, nil
		}
		refProps, err := resolveFn(*props.Ref)
		if err != nil {
			return nil, err
		}
		return refProps, nil
	}

	oneProps := func(props *apiextv1.JSONSchemaProps) (*apiextv1.JSONSchemaProps, error) {
		schemaProps, err := resolve(props)
		if err != nil {
			return nil, err
		}
		// recursively deReference to schemaProps
		if err = deReferenceSchema(schemaProps, resolveFn); err != nil {
			return nil, err
		}
		return schemaProps, nil
	}

	// process additionalProperties
	if props.AdditionalProperties != nil && props.AdditionalProperties.Schema != nil {
		props.AdditionalProperties.Schema, err = oneProps(props.AdditionalProperties.Schema)
		if err != nil {
			return err
		}
	}
	for key := range props.Properties {
		schemaProps := util.ToPointer(props.Properties[key])
		schemaProps, err = oneProps(schemaProps)
		if err != nil {
			return err
		}
		props.Properties[key] = *schemaProps
	}
	return nil
}
