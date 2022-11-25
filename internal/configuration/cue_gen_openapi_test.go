/*
Copyright ApeCloud Inc.

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
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
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
	}{
		{
			name: "normal_test",
			args: args{
				cueFile:    "mysql_openapi.cue",
				schemaType: "MysqlParameter",
			},
			want:    "mysql_openapi.json",
			wantErr: false,
		},
		{
			name: "normal_with_not_empty",
			args: args{
				cueFile:    "mysql_openapi.cue",
				schemaType: "",
			},
			want:    "mysql_openapi.json",
			wantErr: false,
		},
		{
			name: "failed_test",
			args: args{
				cueFile:    "mysql.cue",
				schemaType: "NotType",
			},
			want:    "mysql_openapi_failed_not_exist",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runOpenApiTest(tt.args.cueFile, tt.args.schemaType)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateOpenApiSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantContent := getContentFromFile(tt.want)
			if !reflect.DeepEqual(got, wantContent) {
				t.Errorf("GenerateOpenApiSchema() diff: %s", cmp.Diff(wantContent, got))
			}
		})
	}
}

func getContentFromFile(file string) []byte {
	content, err := os.ReadFile("./testdata/" + file)
	if err != nil {
		return nil
	}
	return content
}

func runOpenApiTest(cueFile string, typeName string) ([]byte, error) {
	cueTpl := getContentFromFile(cueFile)
	if cueTpl == nil {
		return nil, MakeError("not open file[%s]", cueTpl)
	}

	schema, err := GenerateOpenApiSchema(string(cueTpl), typeName)
	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(schema)

	var out = &bytes.Buffer{}
	_ = json.Indent(out, b, "", "  ")

	return out.Bytes(), nil
}
