/*
Copyright 2022.

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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCueOpenApi(t *testing.T) {

	mysqlCfgTpl := `
#MysqlParameter: {
	[SectionName=_]: {
		// SectionName is extract section name

		// [OFF|ON] default ON
		//if SectionName != "client" {
			automatic_sp_privileges: string & "OFF" | "ON" | *"ON"
		//}

		// [1~65535] default ON
		auto_increment_increment: int & >= 1 & <= 65535 | *1

		binlog_stmt_cache_size?: int & >= 4096 & <= 16777216 | *2097152
		// [0|1|2] default: 2
		innodb_autoinc_lock_mode?: int & 0 | 1 | 2 | *2

		// other parmeters
		// reference mysql parmeters
		...
	}
}

// configuration require
configuration: #MysqlParameter & {
}
`

	schema, err := GenerateOpenApiSchema(mysqlCfgTpl, "MysqlParameter")
	require.Nil(t, err)
	schemaString, _ := json.Marshal(schema)
	fmt.Println(string(schemaString))

	schema, err = GenerateOpenApiSchema(mysqlCfgTpl, "MysqlParameter_not_exist")
	require.NotNil(t, err)
	require.Nil(t, schema)

	schema, err = GenerateOpenApiSchema(mysqlCfgTpl, "")
	require.Nil(t, err)
	require.NotNil(t, schema)
}
