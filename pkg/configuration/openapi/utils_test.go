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
	"encoding/json"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/encoding/openapi"
	"github.com/stretchr/testify/assert"
)

func TestGetFieldByLabel(t *testing.T) {
	file := &ast.File{Decls: []ast.Decl{
		&ast.Field{
			Label: ast.NewIdent("a"),
			Value: ast.NewStruct(
				ast.NewIdent("b"), &ast.Ident{
					Name: "c",
					Node: ast.NewString("foo"),
				},
				ast.NewIdent("d"), ast.NewIdent("e"),
			),
		},
	}}

	type args struct {
		f     ast.Node
		label string
	}
	tests := []struct {
		name    string
		args    args
		want    *ast.Field
		wantErr bool
	}{{
		name: "simple",
		args: args{
			f:     file,
			label: "a",
		},
	}, {
		name: "not found",
		args: args{
			f:     file,
			label: "c",
		},
		wantErr: true,
	}, {
		name: "failed",
		args: args{
			f:     file.Decls[0],
			label: "b",
		},
		wantErr: true,
	}, {
		name: "not found",
		args: args{
			f:     file.Decls[0].(*ast.Field).Value,
			label: "b",
		},
		wantErr: false,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetFieldByLabel(tt.args.f, tt.args.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFieldByLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestDeReference(t *testing.T) {
	cueString := `
#TestSA: {
    field1?: string
    field2?: int
    filed3?: {
        field1?: string
        field2?: int
    }
}

// mysql config validator
#Exemplar: {
    l: {
        name: string
    }
    sa: #TestSA

    ta: {
        field1: string
        field2: int
    }
}
`

	expectString := `{"type":"object","properties":{"field1":{"type":"string"},"field2":{"type":"integer"},"filed3":{"type":"object","properties":{"field1":{"type":"string"},"field2":{"type":"integer"}}}}}`

	rt, err := NewRuntime(cueString)
	assert.Nil(t, err)
	assert.NotNil(t, rt)

	v := rt.Context().CompileString("#Exemplar")
	defPath := cue.MakePath(cue.Def("Exemplar"))
	defSche := v.FillPath(defPath, rt.Underlying().LookupPath(cue.MakePath(cue.Def("Exemplar"))))
	openapiOption := &openapi.Config{
		SelfContained:    true,
		ExpandReferences: false,
		Info: ast.NewStruct(
			"title", ast.NewString("test"),
			"version", ast.NewString("none")),
	}

	astf, err := openapi.Generate(defSche.Eval(), openapiOption)
	assert.Nil(t, err)
	deRef := DeReference(astf, rt)

	props, err := deRef("#components/schemas/TestSA")
	assert.Nil(t, err)
	str, _ := json.Marshal(props)
	assert.Equal(t, string(str), expectString)
}
