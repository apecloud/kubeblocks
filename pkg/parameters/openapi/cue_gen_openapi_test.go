/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestGenerateOpenApiSchema(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenAPI Schema Suite")
}

var _ = Describe("GenerateOpenAPISchema", func() {
	DescribeTable("generates schema from CUE definitions",
		func(cueFile, schemaType, want string, wantErr bool) {
			got, err := runOpenAPITest(cueFile, schemaType)
			GinkgoWriter.Println(string(got))
			Expect(err != nil).To(Equal(wantErr), "GenerateOpenAPISchema() error = %v, wantErr %v", err, wantErr)
			if wantErr {
				return
			}

			wantContent := getContentFromFile(want)
			Expect(got).To(Equal(wantContent), "GenerateOpenAPISchema() diff: %s", cmp.Diff(wantContent, got))
		},
		Entry("test_import_type", "test_import_type.cue", "Exemplar", "test_import_type.json", false),
		Entry("normal_test with mysql", "mysql_openapi.cue", "MysqlParameter", "mysql_openapi.json", false),
		FEntry("normal_test with mysql2", "mysql_openapi_v2.cue", "MysqlSchema", "mysql_openapi_v2.json", false),
		Entry("normal_with_not_empty", "mysql_openapi.cue", "", "mysql_openapi.json", false),
		Entry("pg14_openapi", "pg14.cue", "PGPameter", "pg14_openapi.json", false),
		Entry("multiple_schema_arch_a", "multiple_schema.cue", "archA", "multiple_schema_arch_a.json", false),
		Entry("multiple_schema_combined", "multiple_schema.cue", "combined", "multiple_schema_combined.json", false),
		Entry("failed_test", "mysql.cue", "NotType", "mysql_openapi_failed_not_exist", true),
	)
})

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
