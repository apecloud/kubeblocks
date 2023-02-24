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

package configuration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var fromTestData = func(fileName string) string {
	content, err := testdata.GetTestDataFileContent(fileName)
	if err != nil {
		panic(err)
	}
	return string(content)
}

var newFakeConfConstraint = func(cueFile string, cfgFormatter appsv1alpha1.CfgFileFormat) *appsv1alpha1.ConfigConstraintSpec {
	return &appsv1alpha1.ConfigConstraintSpec{
		ConfigurationSchema: &appsv1alpha1.CustomParametersValidation{
			CUE: fromTestData(cueFile),
		},
		FormatterConfig: &appsv1alpha1.FormatterConfig{
			Format: cfgFormatter,
		},
	}
}

func TestSchemaValidatorWithCue(t *testing.T) {
	type args struct {
		cueFile    string
		configFile string
		format     appsv1alpha1.CfgFileFormat
	}
	tests := []struct {
		name string
		args args
		err  error
	}{{
		name: "test_wesql",
		args: args{
			cueFile:    "cue_testdata/wesql.cue",
			configFile: "cue_testdata/wesql.cnf",
			format:     appsv1alpha1.Ini,
		},
		err: nil,
	}, {
		name: "test_pg14",
		args: args{
			cueFile:    "cue_testdata/pg14.cue",
			configFile: "cue_testdata/pg14.conf",
			format:     appsv1alpha1.Properties,
		},
		err: nil,
	}, {
		name: "test_ck",
		args: args{
			cueFile:    "cue_testdata/clickhouse.cue",
			configFile: "cue_testdata/clickhouse.xml",
			format:     appsv1alpha1.XML,
		},
		err: nil,
	}, {
		name: "test_mysql",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql.cnf",
			format:     appsv1alpha1.Ini,
		},
		err: nil,
	}, {
		name: "test_failed",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql_err.cnf",
			format:     appsv1alpha1.Ini,
		},
		err: errors.New(`failed to cue template render configure: [mysqld.innodb_autoinc_lock_mode: 3 errors in empty disjunction:
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    28:35
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    28:39
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    28:43
]`),
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigValidator(newFakeConfConstraint(tt.args.cueFile, tt.args.format))
			require.NotNil(t, validator)
			require.Equal(t, tt.err, validator.Validate(
				map[string]string{
					"key": fromTestData(tt.args.configFile),
				}))
		})
	}
}

func TestSchemaValidatorWithOpenSchema(t *testing.T) {
	type args struct {
		cueFile        string
		configFile     string
		format         appsv1alpha1.CfgFileFormat
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
			format:     appsv1alpha1.Ini,
		},
		err: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tplConstraint := newFakeConfConstraint(tt.args.cueFile, tt.args.format)
			validator := &schemaValidator{
				typeName: tt.args.SchemaTypeName,
				cfgType:  tplConstraint.FormatterConfig.Format,
				schema:   tplConstraint.ConfigurationSchema.Schema,
			}
			require.Equal(t, tt.err, validator.Validate(
				map[string]string{
					"key": fromTestData(tt.args.configFile),
				}))
		})
	}
}
