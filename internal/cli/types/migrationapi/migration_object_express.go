/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"fmt"
	"strings"
)

type MigrationObjectExpress struct {
	WhiteList []DBObjectExpress `json:"whiteList"`
	// +optional
	BlackList []DBObjectExpress `json:"blackList"`
}

func (m *MigrationObjectExpress) String(isWhite bool) string {
	expressArr := m.WhiteList
	if !isWhite {
		expressArr = m.BlackList
	}
	stringArr := make([]string, 0)
	for _, db := range expressArr {
		stringArr = append(stringArr, db.String()...)
	}
	return strings.Join(stringArr, ",")
}

type DBObjectExpress struct {
	SchemaName string `json:"schemaName"`
	// +optional
	SchemaMappingName string `json:"schemaMappingName"`
	// +optional
	IsAll bool `json:"isAll"`
	// +optional
	TableList   []TableObjectExpress `json:"tableList"`
	DxlOpConfig `json:""`
}

func (d *DBObjectExpress) String() []string {
	stringArr := make([]string, 0)
	if d.IsAll {
		stringArr = append(stringArr, d.SchemaName)
	} else {
		for _, tb := range d.TableList {
			stringArr = append(stringArr, fmt.Sprintf("%s.%s", d.SchemaName, tb.TableName))
		}
	}
	return stringArr
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
