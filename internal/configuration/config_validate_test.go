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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var fromTestData = func(fileName string) string {
	content, err := testdata.GetTestDataFileContent(fileName)
	if err != nil {
		panic(err)
	}
	return string(content)
}

var newFakeConfConstraint = func(cueFile string, cfgFormatter dbaasv1alpha1.ConfigurationFormatter) *dbaasv1alpha1.ConfigConstraintSpec {
	return &dbaasv1alpha1.ConfigConstraintSpec{
		ConfigurationSchema: &dbaasv1alpha1.CustomParametersValidation{
			CUE: fromTestData(cueFile),
		},
		FormatterConfig: &dbaasv1alpha1.FormatterConfig{
			Formatter: cfgFormatter,
		},
	}
}

func TestSchemaValidatorWithCue(t *testing.T) {
	type args struct {
		cueFile    string
		configFile string
		formatter  dbaasv1alpha1.ConfigurationFormatter
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
			formatter:  dbaasv1alpha1.INI,
		},
		err: nil,
	}, {
		name: "test_pg14",
		args: args{
			cueFile:    "cue_testdata/pg14.cue",
			configFile: "cue_testdata/pg14.conf",
			formatter:  dbaasv1alpha1.DOTENV,
		},
		err: nil,
	}, {
		name: "test_ck",
		args: args{
			cueFile:    "cue_testdata/clickhouse.cue",
			configFile: "cue_testdata/clickhouse.xml",
			formatter:  dbaasv1alpha1.XML,
		},
		err: nil,
	}, {
		name: "test_mysql",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql.cnf",
			formatter:  dbaasv1alpha1.INI,
		},
		err: nil,
	}, {
		name: "test_failed",
		args: args{
			cueFile:    "cue_testdata/mysql.cue",
			configFile: "cue_testdata/mysql_err.cnf",
			formatter:  dbaasv1alpha1.INI,
		},
		err: errors.New(`failed to cue template render configure: [mysqld.innodb_autoinc_lock_mode: 3 errors in empty disjunction:
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    14:35
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    14:39
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    14:43
]`),
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigValidator(newFakeConfConstraint(tt.args.cueFile, tt.args.formatter))
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
		formatter      dbaasv1alpha1.ConfigurationFormatter
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
			formatter:  dbaasv1alpha1.INI,
		},
		err: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tplConstraint := newFakeConfConstraint(tt.args.cueFile, tt.args.formatter)
			validator := &schemaValidator{
				typeName: tt.args.SchemaTypeName,
				cfgType:  tplConstraint.FormatterConfig.Formatter,
				schema:   tplConstraint.ConfigurationSchema.Schema,
			}
			require.Equal(t, tt.err, validator.Validate(
				map[string]string{
					"key": fromTestData(tt.args.configFile),
				}))
		})
	}
}
