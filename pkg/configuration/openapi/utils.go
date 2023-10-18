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
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/pkg/encoding/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
)

func strEq(lit *ast.BasicLit, str string) bool {
	if lit.Kind != token.STRING {
		return false
	}
	ls, _ := strconv.Unquote(lit.Value)
	return str == ls || str == lit.Value
}

func identStrEq(id *ast.Ident, str string) bool {
	if str == id.Name {
		return true
	}
	ls, _ := strconv.Unquote(id.Name)
	return str == ls
}

func IsFieldWithLabel(n ast.Node, label string) bool {
	field, ok := n.(*ast.Field)
	if !ok {
		return false
	}
	switch l := field.Label.(type) {
	default:
		return false
	case *ast.BasicLit:
		return strEq(l, label)
	case *ast.Ident:
		return identStrEq(l, label)
	}
}

func GetFieldByLabel(n ast.Node, label string) (*ast.Field, error) {
	var d []ast.Decl
	switch x := n.(type) {
	case *ast.File:
		d = x.Decls
	case *ast.StructLit:
		d = x.Elts
	default:
		return nil, core.MakeError("not an *ast.File or *ast.StructLit")
	}

	for _, el := range d {
		if IsFieldWithLabel(el, label) {
			return el.(*ast.Field), nil
		}
	}
	return nil, core.MakeError("no field with label %q", label)
}

func DeReference(f *ast.File, rt *Runtime) func(path string) (*apiextv1.JSONSchemaProps, error) {
	return func(path string) (*apiextv1.JSONSchemaProps, error) {
		typeRef := fetchTypeDefine(strings.Trim(path, "#"), f)
		slit, ok := typeRef.(*ast.StructLit)
		if !ok {
			return nil, core.MakeError("not a struct literal")
		}
		return FromCueAST(rt, slit.Elts)
	}
}

func fetchTypeDefine(value string, node ast.Node) ast.Node {
	n := node
	for _, field := range strings.Split(value, "/") {
		if field == "" {
			continue
		}
		nn, _ := GetFieldByLabel(n, field)
		if nn == nil {
			return nil
		}
		n = nn.Value
	}
	return n
}

func FromCueAST(rt *Runtime, cueSchema []ast.Decl) (*apiextv1.JSONSchemaProps, error) {
	yamlStr, err := yaml.Marshal(rt.BuildFile(
		&ast.File{Decls: cueSchema},
	))
	if err != nil {
		return nil, core.MakeError("failed to marshal cue-yaml: %s", errors.Details(err, nil))
	}

	jsonProps := apiextv1.JSONSchemaProps{}
	if err = yaml2.Unmarshal([]byte(yamlStr), &jsonProps); err != nil {
		return nil, core.WrapError(err, "failed to unmarshal raw OpenAPI schema to JSONSchemaProps")
	}
	return &jsonProps, nil
}
