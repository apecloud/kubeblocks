/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package validate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var fromTestData = func(fileName string) string {
	content, err := testdata.GetTestDataFileContent(fileName)
	if err != nil {
		panic(err)
	}
	return string(content)
}

var newFakeConfigSchema = func(cueFile string) *parametersv1alpha1.ParametersSchema {
	return &parametersv1alpha1.ParametersSchema{
		CUE: fromTestData(cueFile),
	}
}

func TestSchemaValidatorWithCue(t *testing.T) {
	type args struct {
		cueFile    string
		configFile string
		format     parametersv1alpha1.CfgFileFormat
	}
	tests := []struct {
		name string
		args args
		err  error
	}{{
		name: "mongod_test",
		args: args{
			cueFile:    "cue_testdata/mongod.cue",
			configFile: "cue_testdata/mongod.conf",
			format:     parametersv1alpha1.YAML,
		},
		err: nil,
	}, {
		name: "test_wesql",
		args: args{
			cueFile:    "cue_testdata/wesql.cue",
			configFile: "cue_testdata/wesql.cnf",
			format:     parametersv1alpha1.Ini,
		},
		err: nil,
	}, {
		name: "test_pg14",
		args: args{
			cueFile:    "cue_testdata/pg14.cue",
			configFile: "cue_testdata/pg14.conf",
			format:     parametersv1alpha1.Properties,
		},
		err: nil,
	}, {
		name: "test_ck",
		args: args{
			cueFile:    "cue_testdata/clickhouse.cue",
			configFile: "cue_testdata/clickhouse.xml",
			format:     parametersv1alpha1.XML,
		},
		err: nil,
	}, {
		name: "test_mysql",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql.cnf",
			format:     parametersv1alpha1.Ini,
		},
		err: nil,
	}, {
		name: "test_failed",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql_err.cnf",
			format:     parametersv1alpha1.Ini,
		},
		err: errors.New(`failed to render cue template configure: [mysqld.innodb_autoinc_lock_mode: 3 errors in empty disjunction:
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    31:35
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    31:39
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    31:43]`),
	}, {
		name: "configmap_key_filter",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql.cnf",
			format:     parametersv1alpha1.Ini,
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigValidator(newFakeConfigSchema(tt.args.cueFile), &parametersv1alpha1.FileFormatConfig{Format: tt.args.format})
			require.NotNil(t, validator)
			require.Equal(t, tt.err, validator.Validate(fromTestData(tt.args.configFile)))
		})
	}
}

func TestSchemaValidatorWithSelector(t *testing.T) {
	validator := NewConfigValidator(newFakeConfigSchema("cue_testdata/mysql.cue"), &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini})
	require.NotNil(t, validator)
	require.ErrorContains(t, validator.Validate(
		fromTestData("cue_testdata/mysql_err.cnf"),
	), "[mysqld.innodb_autoinc_lock_mode: 3 errors in empty disjunction")
}

func TestSchemaValidatorWithOpenSchema(t *testing.T) {
	type args struct {
		cueFile        string
		configFile     string
		format         parametersv1alpha1.CfgFileFormat
		SchemaTypeName string
	}
	tests := []struct {
		name string
		args args
		err  error
	}{{
		name: "test_wesql",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql.cnf",
			format:     parametersv1alpha1.Ini,
		},
		err: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configSchema := newFakeConfigSchema(tt.args.cueFile)
			validator := &schemaValidator{
				typeName: tt.args.SchemaTypeName,
				cfgType:  tt.args.format,
				schema:   configSchema.SchemaInJSON,
			}
			require.Equal(t, tt.err, validator.Validate(fromTestData(tt.args.configFile)))
		})
	}
}
