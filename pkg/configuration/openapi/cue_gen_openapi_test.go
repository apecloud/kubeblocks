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
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestGenerateOpenApiSchema(t *testing.T) {
	type args struct {
		cueFile    string
		schemaType string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{{
		name: "normal_test",
		args: args{
			cueFile:    "test_import_type.cue",
			schemaType: "Exemplar",
		},
		want:    "test_import_type.json",
		wantErr: false,
	}, {
		name: "normal_test",
		args: args{
			cueFile:    "mysql_openapi.cue",
			schemaType: "MysqlParameter",
		},
		want:    "mysql_openapi.json",
		wantErr: false,
	}, {
		//	name: "normal_test",
		//	args: args{
		//		cueFile:    "mysql_openapi_v2.cue",
		//		schemaType: "MysqlSchema",
		//	},
		//	want:    "mysql_openapi_v2.json",
		//	wantErr: false,
		// }, {
		name: "normal_with_not_empty",
		args: args{
			cueFile:    "mysql_openapi.cue",
			schemaType: "",
		},
		want:    "mysql_openapi.json",
		wantErr: false,
	}, {
		name: "pg14_openapi",
		args: args{
			cueFile:    "pg14.cue",
			schemaType: "PGPameter",
		},
		want:    "pg14_openapi.json",
		wantErr: false,
	}, {
		name: "failed_test",
		args: args{
			cueFile:    "mysql.cue",
			schemaType: "NotType",
		},
		want:    "mysql_openapi_failed_not_exist",
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runOpenAPITest(tt.args.cueFile, tt.args.schemaType)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateOpenAPISchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantContent := getContentFromFile(tt.want)
			if !reflect.DeepEqual(got, wantContent) {
				t.Errorf("GenerateOpenAPISchema() diff: %s", cmp.Diff(wantContent, got))
			}
		})
	}
}

func getContentFromFile(file string) []byte {
	content, err := os.ReadFile(testdata.SubTestDataPath("./cue_testdata/" + file))
	if err != nil {
		return nil
	}
	return content
}

func runOpenAPITest(cueFile string, typeName string) ([]byte, error) {
	cueTpl := getContentFromFile(cueFile)
	if cueTpl == nil {
		return nil, core.MakeError("cannot open file[%s]", cueTpl)
	}

	schema, err := GenerateOpenAPISchema(string(cueTpl), typeName)
	if err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, core.MakeError("cannot find schema.")
	}

	b, _ := json.Marshal(schema)

	var out = &bytes.Buffer{}
	_ = json.Indent(out, b, "", "  ")

	return out.Bytes(), nil
}
