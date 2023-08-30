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

	"github.com/apecloud/kubeblocks/internal/configuration/core"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	SchemaFieldName    = "schemas"
	ComponentFieldName = "components"
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

	var path cue.Value
	if schemaType != "" {
		path = rt.Underlying().LookupPath(cue.MakePath(cue.Def(schemaType)))
		if path.Err() != nil {
			return nil, core.MakeError(errors.Details(path.Err(), nil))
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
		path = defs.Value()
	}

	v := rt.Context().CompileString(fmt.Sprintf("#%s", schemaType))
	defPath := cue.MakePath(cue.Def(schemaType))
	defSche := v.FillPath(defPath, path)
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

func transformOpenAPISchema(rt *Runtime, cueSchema []ast.Decl, refFn func(path string) (*apiextv1.JSONSchemaProps, error)) (*apiextv1.JSONSchemaProps, error) {
	jsonProps, err := FromCueAST(rt, cueSchema)
	if err != nil {
		return nil, err
	}

	if err := deReferenceSchema(jsonProps, refFn); err != nil {
		return nil, err
	}
	r := apiextv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextv1.JSONSchemaProps{
			"spec": *jsonProps,
		},
	}
	return &r, nil
}

func deReferenceSchema(props *apiextv1.JSONSchemaProps, fn func(path string) (*apiextv1.JSONSchemaProps, error)) error {
	for key := range props.Properties {
		schemaProps := props.Properties[key]
		if schemaProps.Ref != nil {
			refProps, err := fn(*schemaProps.Ref)
			if err != nil {
				return err
			}
			schemaProps = *refProps
		}
		if err := deReferenceSchema(&schemaProps, fn); err != nil {
			return err
		}
		props.Properties[key] = schemaProps
	}
	return nil
}
