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
)

func Test_schemaValidator_Validate(t *testing.T) {

	formString := func(str string) *string {
		return &str
	}

	toMap := func(str string) map[string]string {
		return map[string]string{
			"noKey": str,
		}
	}

	// not config validate
	{
		validator := NewConfigValidator(&dbaasv1alpha1.ConfigurationTemplateSpec{})
		require.NotNil(t, validator)
		require.Nil(t, validator.Validate(nil))
	}

	// cue validate for ini
	{
		validator := NewConfigValidator(&dbaasv1alpha1.ConfigurationTemplateSpec{
			ConfigurationSchema: &dbaasv1alpha1.CustomParametersValidation{
				Cue: formString(loadTestData("./testdata/mysql.cue")),
			},
			Formatter: dbaasv1alpha1.INI,
		})

		require.Nil(t, validator.Validate(toMap(loadTestData("./testdata/mysql.cnf"))))
		expectErr := errors.New(`failed to cue template render configure: [configuration: field not allowed: notsection:
    2:18
    30:16
    30:34
]`)
		require.Equal(t, expectErr, validator.Validate(toMap(loadTestData("./testdata/mysql_err.cnf"))))
	}

	// cue validate for xml
	{
		validator := NewConfigValidator(&dbaasv1alpha1.ConfigurationTemplateSpec{
			ConfigurationSchema: &dbaasv1alpha1.CustomParametersValidation{
				Cue: formString(loadTestData("./testdata/clickhouse.cue")),
			},
			Formatter: dbaasv1alpha1.XML,
		})

		require.Nil(t, validator.Validate(toMap(loadTestData("./testdata/clickhouse.xml"))))
	}

}

func loadTestData(fileName string) string {
	content, _ := os.ReadFile(fileName)
	return string(content)
}
