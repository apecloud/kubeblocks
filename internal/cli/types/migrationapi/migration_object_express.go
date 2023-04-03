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

package v1alpha1

type MigrationObjectExpress struct {
	WhiteList []DbObjectExpress `json:"whiteList"`
	// +optional
	BlackList []DbObjectExpress `json:"blackList"`
}

type DbObjectExpress struct {
	SchemaName string `json:"schemaName"`
	// +optional
	SchemaMappingName string `json:"schemaMappingName"`
	// +optional
	IsAll bool `json:"isAll"`
	// +optional
	TableList   []TableObjectExpress `json:"tableList"`
	DxlOpConfig `json:""`
}

type TableObjectExpress struct {
	TableName string `json:"tableName"`
	// +optional
	TableMappingName string `json:"tableMappingName"`
	// +optional
	IsAll bool `json:"isAll"`
	// +optional
	FieldList   []FieldObjectExpress `json:"fieldList"`
	DxlOpConfig `json:""`
}

type FieldObjectExpress struct {
	FieldName string `json:"fieldName"`
	// +optional
	FieldMappingName string `json:"fieldMappingName"`
}

type DxlOpConfig struct {
	// +optional
	DmlOp []DMLOpEnum `json:"dmlOp"`
	// +optional
	DdlOp []DDLOpEnum `json:"ddlOp"`
	// +optional
	DclOp []DCLOpEnum `json:"dclOp"`
}

func (op *DxlOpConfig) IsEmpty() bool {
	return len(op.DmlOp) == 0
}
