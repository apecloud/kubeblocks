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
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var toMap = func(str string) map[string]string {
	return map[string]string{
		"noKey": str,
	}
}

func TestSchemaValidatorWithCue(t *testing.T) {
	// not config validate
	{
		validator := NewConfigValidator(&dbaasv1alpha1.ConfigConstraintSpec{})
		require.NotNil(t, validator)
		require.Nil(t, validator.Validate(nil))
	}

	{
		validator := NewConfigValidator(fakeConfigurationTpl("./cue_testdata/wesql.cue", dbaasv1alpha1.INI))
		require.Nil(t, validator.Validate(toMap(loadTestData("./cue_testdata/wesql.cnf"))))
	}

	// cue validate for ini
	{
		validator := NewConfigValidator(fakeConfigurationTpl("./cue_testdata/mysql.cue", dbaasv1alpha1.INI))
		require.Nil(t, validator.Validate(toMap(loadTestData("./cue_testdata/mysql.cnf"))))
		expectErr := errors.New(`failed to cue template render configure: [mysqld.innodb_autoinc_lock_mode: 3 errors in empty disjunction:
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    14:35
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    14:39
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    14:43
]`)
		require.Equal(t, expectErr, validator.Validate(toMap(loadTestData("./cue_testdata/mysql_err.cnf"))))
	}

	// cue validate for xml
	{
		validator := NewConfigValidator(fakeConfigurationTpl("./cue_testdata/clickhouse.cue", dbaasv1alpha1.XML))
		require.Nil(t, validator.Validate(toMap(loadTestData("./cue_testdata/clickhouse.xml"))))
	}

}

func TestSchemaValidatorWithOpenSchema(t *testing.T) {
	tpl := fakeConfigurationTpl("./cue_testdata/mysql.cue", dbaasv1alpha1.INI)
	validator := &schemaValidator{
		typeName: tpl.CfgSchemaTopLevelName,
		cfgType:  tpl.FormatterConfig.Formatter,
		schema:   tpl.ConfigurationSchema.Schema,
	}

	require.Nil(t, validator.Validate(toMap(loadTestData("./cue_testdata/mysql.cnf"))))
}

func fakeConfigurationTpl(cuefile string, cfgFormatter dbaasv1alpha1.ConfigurationFormatter) *dbaasv1alpha1.ConfigConstraintSpec {
	return &dbaasv1alpha1.ConfigConstraintSpec{
		ConfigurationSchema: &dbaasv1alpha1.CustomParametersValidation{
			CUE: loadTestData(cuefile),
		},
		FormatterConfig: &dbaasv1alpha1.FormatterConfig{
			Formatter: cfgFormatter,
		},
	}
}

func loadTestData(fileName string) string {
	content, _ := os.ReadFile(testdata.SubTestDataPath(fileName))
	return string(content)
}
