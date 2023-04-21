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

package unstructured

import (
	"testing"

	"github.com/stretchr/testify/assert"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestIniFormat(t *testing.T) {
	const iniContext = `
[client]
socket=/data/mysql/tmp/mysqld.sock

[mysqld]
gtid_mode=OFF
innodb-buffer-pool-size=512M
log_error=/data/mysql/log/mysqld.err
loose-caching_sha2_password_auto_generate_rsa_keys=0
port=3306
`

	iniConfigObj, err := LoadConfig("ini_test", iniContext, appsv1alpha1.Ini)
	assert.Nil(t, err)

	assert.EqualValues(t, iniConfigObj.Get("mysqld.gtid_mode"), "OFF")
	assert.EqualValues(t, iniConfigObj.Get("mysqld.log_error"), "/data/mysql/log/mysqld.err")
	assert.EqualValues(t, iniConfigObj.Get("client.socket"), "/data/mysql/tmp/mysqld.sock")

	dumpContext, err := iniConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, dumpContext, iniContext[1:]) // trim "\n"

	// test sub
	subConfigObj := iniConfigObj.SubConfig("mysqld")
	assert.Nil(t, subConfigObj.Update("gtid_mode", "ON"))
	assert.EqualValues(t, subConfigObj.Get("gtid_mode"), "ON")
	assert.EqualValues(t, subConfigObj.Get("log_error"), "/data/mysql/log/mysqld.err")
}

func TestPropertiesFormat(t *testing.T) {
	const propertiesContext = `
listen_addresses = '*'
port = '5432'

#archive_command = 'wal_dir=/pg/arcwal; [[ $(date +%H%M) == 1200 ]] && rm -rf ${wal_dir}/$(date -d"yesterday" +%Y%m%d); /bin/mkdir -p ${wal_dir}/$(date +%Y%m%d) && /usr/bin/lz4 -q -z %p > ${wal_dir}/$(date +%Y%m%d)/%f.lz4'
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
	propConfigObj, err := LoadConfig("prop_test", propertiesContext, appsv1alpha1.Properties)
	assert.Nil(t, err)

	assert.EqualValues(t, propConfigObj.Get("auto_explain.log_nested_statements"), "'True'")
	assert.EqualValues(t, propConfigObj.Get("auto_explain.log_min_duration"), "'1s'")
	assert.EqualValues(t, propConfigObj.Get("autovacuum_naptime"), "'1min'")

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

	jsonConfigObj, err := LoadConfig("json_test", jsonContext, appsv1alpha1.JSON)
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
