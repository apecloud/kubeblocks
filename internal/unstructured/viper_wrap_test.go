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

func TestYAMLFormat(t *testing.T) {
	const yamlContext = `
spec:
    clusterref: pg
    reconfigure:
        componentname: postgresql
        configurations:
            - keys:
                - key: postgresql.conf
                  parameters:
                    - key: max_connections
                      value: "2666"
              name: postgresql-configuration
`

	yamlConfigObj, err := LoadConfig("yaml_test", yamlContext, appsv1alpha1.YAML)
	assert.Nil(t, err)

	assert.EqualValues(t, yamlConfigObj.Get("spec.clusterRef"), "pg")
	assert.EqualValues(t, yamlConfigObj.Get("spec.reconfigure.componentName"), "postgresql")

	dumpContext, err := yamlConfigObj.Marshal()
	assert.Nil(t, err)
	assert.EqualValues(t, dumpContext, yamlContext[1:]) // trim "\n"

	assert.Nil(t, yamlConfigObj.Update("spec.my_test", "100"))
	assert.EqualValues(t, yamlConfigObj.Get("spec.my_test"), "100")
}
