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

package unstructured

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

func TestViperRemoveKey(t *testing.T) {
	tests := []struct {
		name      string
		format    parametersv1alpha1.CfgFileFormat
		content   string
		key       string
		preserved map[string]any
	}{
		{
			name: "json top level", format: parametersv1alpha1.JSON,
			content: `{"remove":"old","keep":"unchanged","enabled":false,"limit":0,"hosts":["a","b"]}`, key: "remove",
			preserved: map[string]any{"keep": "unchanged", "enabled": false, "limit": float64(0), "hosts": []any{"a", "b"}},
		},
		{
			name: "json nested", format: parametersv1alpha1.JSON,
			content: `{"server":{"remove":"old","keep":"unchanged"},"other":"retained"}`, key: "server.remove",
			preserved: map[string]any{"server.keep": "unchanged", "other": "retained"},
		},
		{
			name: "ini section", format: parametersv1alpha1.Ini,
			content: "[server]\nremove=old\nkeep=unchanged\n[client]\nremove=retained\n", key: "server.remove",
			preserved: map[string]any{"server.keep": "unchanged", "client.remove": "retained"},
		},
		{
			name: "toml nested", format: parametersv1alpha1.TOML,
			content: "[server]\nremove = \"old\"\nkeep = \"unchanged\"\n", key: "server.remove",
			preserved: map[string]any{"server.keep": "unchanged"},
		},
		{
			name: "properties literal dotted key", format: parametersv1alpha1.Properties,
			content: "server.remove=old\nserver.keep=unchanged\n", key: "server.remove",
			preserved: map[string]any{"server.keep": "unchanged"},
		},
		{
			name: "dotenv literal dotted key", format: parametersv1alpha1.Dotenv,
			content: "server.remove=old\nserver.keep=unchanged\n", key: "server.remove",
			preserved: map[string]any{"server.keep": "unchanged"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfig("remove_test", tt.content, tt.format)
			require.NoError(t, err)
			require.Equal(t, "old", config.Get(tt.key))

			require.NoError(t, config.RemoveKey(strings.ToUpper(tt.key)))
			assert.Nil(t, config.Get(tt.key))
			require.NoError(t, config.RemoveKey(tt.key))
			require.NoError(t, config.RemoveKey("missing"))

			content, err := config.Marshal()
			require.NoError(t, err)
			reloaded, err := LoadConfig("remove_test", content, tt.format)
			require.NoError(t, err)
			assert.Nil(t, reloaded.Get(tt.key))
			for key, value := range tt.preserved {
				assert.Equal(t, value, config.Get(key))
				assert.Equal(t, value, reloaded.Get(key))
			}
		})
	}
}

func TestViperRemoveKeyAfterUpdate(t *testing.T) {
	config, err := LoadConfig("remove_test", `{"server":{"remove":"old","keep":"unchanged"}}`, parametersv1alpha1.JSON)
	require.NoError(t, err)
	require.NoError(t, config.Update("server.remove", "updated"))
	require.Equal(t, "updated", config.Get("server.remove"))

	require.NoError(t, config.RemoveKey("server.remove"))
	assert.Nil(t, config.Get("server.remove"))
	content, err := config.Marshal()
	require.NoError(t, err)
	reloaded, err := LoadConfig("remove_test", content, parametersv1alpha1.JSON)
	require.NoError(t, err)
	assert.Nil(t, reloaded.Get("server.remove"))
	assert.Equal(t, "unchanged", reloaded.Get("server.keep"))

	require.NoError(t, config.Update("server.remove", "restored"))
	assert.Equal(t, "restored", config.Get("server.remove"))
	content, err = config.Marshal()
	require.NoError(t, err)
	reloaded, err = LoadConfig("remove_test", content, parametersv1alpha1.JSON)
	require.NoError(t, err)
	assert.Equal(t, "restored", reloaded.Get("server.remove"))
	assert.Equal(t, "unchanged", reloaded.Get("server.keep"))
}

func TestIniFormat(t *testing.T) {
	const iniContext = `
[client]
socket=/data/mysql/tmp/mysqld.sock

[mysqld]
gtid_mode=OFF
innodb-buffer-pool-size=512M
log_error=/data/mysql/log/mysqld.err
loose-caching_sha2_password_auto_generate_rsa_keys=0
plugin-load = "rpl_semi_sync_master=semisync_master.so;rpl_semi_sync_slave=semisync_slave.so"
port=3306
`

	iniConfigObj, err := LoadConfig("ini_test", iniContext, parametersv1alpha1.Ini)
	assert.Nil(t, err)

	assert.EqualValues(t, iniConfigObj.Get("mysqld.gtid_mode"), "OFF")
	assert.EqualValues(t, iniConfigObj.Get("mysqld.log_error"), "/data/mysql/log/mysqld.err")
	assert.EqualValues(t, iniConfigObj.Get("client.socket"), "/data/mysql/tmp/mysqld.sock")

	// dumpContext, err := iniConfigObj.Marshal()
	// assert.Nil(t, err)
	// assert.EqualValues(t, dumpContext, iniContext[1:]) // trim "\n"

	// test sub
	subConfigObj := iniConfigObj.SubConfig("mysqld")
	assert.Nil(t, subConfigObj.Update("gtid_mode", "ON"))
	assert.EqualValues(t, subConfigObj.Get("gtid_mode"), "ON")
	assert.EqualValues(t, subConfigObj.Get("log_error"), "/data/mysql/log/mysqld.err")
	assert.EqualValues(t, subConfigObj.Get("plugin-load"), "\"rpl_semi_sync_master=semisync_master.so;rpl_semi_sync_slave=semisync_slave.so\"")
}

func TestPropertiesFormat1(t *testing.T) {
	const propertiesContext = `
listen_addresses = '*'
port = '5432'
archive_command = '[[ $(date +%H%M) == 1200 ]] && rm -rf /home/postgres/pgdata/pgroot/arcwal/$(date -d"yesterday" +%Y%m%d); mkdir -p /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d) && gzip -kqc %p > /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d)/%f.gz'

#archive_mode = 'True'
auto_explain.log_analyze = 'True'
auto_explain.log_min_duration = '1s'
auto_explain.log_nested_statements = 'True'
auto_explain.log_timing = 'True'
auto_explain.log_verbose = 'True'
autovacuum_analyze_scale_factor = '0.05'
autovacuum_freeze_max_age = '100000000'
autovacuum_max_workers = '1'
autovacuum_naptime = '1min'
`
	propConfigObj, err := LoadConfig("prop_test", propertiesContext, parametersv1alpha1.Properties)
	assert.Nil(t, err)

	assert.EqualValues(t, propConfigObj.Get("auto_explain.log_nested_statements"), "'True'")
	assert.EqualValues(t, propConfigObj.Get("auto_explain.log_min_duration"), "'1s'")
	assert.EqualValues(t, propConfigObj.Get("autovacuum_naptime"), "'1min'")
	assert.EqualValues(t, propConfigObj.Get("archive_command"), `'[[ $(date +%H%M) == 1200 ]] && rm -rf /home/postgres/pgdata/pgroot/arcwal/$(date -d"yesterday" +%Y%m%d); mkdir -p /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d) && gzip -kqc %p > /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d)/%f.gz'`)

	dumpContext, err := propConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, dumpContext, propertiesContext[1:]) // trim "\n"

	assert.Nil(t, propConfigObj.Update("autovacuum_naptime", "'6min'"))
	assert.EqualValues(t, propConfigObj.Get("autovacuum_naptime"), "'6min'")
}

func TestJSONFormat(t *testing.T) {
	const jsonContext = `
{
  "id": "0001",
  "name": "zhangsan",
  "score": {
    "chemistry": "80",
    "literature": "78",
    "math": 98
  },
  "toplevel": 10,
  "type": "student"
}`

	jsonConfigObj, err := LoadConfig("json_test", jsonContext, parametersv1alpha1.JSON)
	assert.Nil(t, err)

	assert.EqualValues(t, jsonConfigObj.Get("id"), "0001")
	assert.EqualValues(t, jsonConfigObj.Get("score.chemistry"), "80")

	dumpContext, err := jsonConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, dumpContext, jsonContext[1:]) // trim "\n"

	assert.Nil(t, jsonConfigObj.Update("abcd", "test"))
	assert.EqualValues(t, jsonConfigObj.Get("abcd"), "test")

	assert.Nil(t, jsonConfigObj.Update("name", "test"))
	assert.EqualValues(t, jsonConfigObj.Get("name"), "test")

}

func TestTomlFormat(t *testing.T) {
	const tomlContext = `
[tick]
## the tick interval when starting PD inside (default: "100ms")
sim-tick-interval = "100ms"

[store]
## the capacity size of a new store in GB (default: 1024)
store-capacity = 1024
## the available size of a new store in GB (default: 1024)
store-available = 1024
## the io rate of a new store in MB/s (default: 40)
store-io-per-second = 40
## the version of a new store (default: "2.1.0")
store-version = "2.1.0"
`

	tomlConfigObj, err := LoadConfig("toml_test", tomlContext, parametersv1alpha1.TOML)
	assert.Nil(t, err)

	assert.EqualValues(t, tomlConfigObj.Get("tick.sim-tick-interval"), "100ms")
	assert.EqualValues(t, tomlConfigObj.Get("store.store-capacity"), 1024)
	assert.Nil(t, tomlConfigObj.Update("store.test-int-field", 200))
	assert.Nil(t, tomlConfigObj.Update("store.test-field", "test"))

	dumpContext, err := tomlConfigObj.Marshal()
	assert.Nil(t, err)
	tomlConfigObj, err = LoadConfig("toml_test", dumpContext, parametersv1alpha1.TOML)
	assert.Nil(t, err)
	assert.EqualValues(t, tomlConfigObj.Get("store.test-field"), "test")
	assert.EqualValues(t, tomlConfigObj.Get("store.test-int-field"), 200)

	assert.EqualValues(t, tomlConfigObj.Get("store.store-io-per-second"), 40)
	assert.Nil(t, tomlConfigObj.Update("store.store-io-per-second", 1000))
	assert.EqualValues(t, tomlConfigObj.Get("store.store-io-per-second"), 1000)
}
