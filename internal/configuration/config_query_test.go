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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestGetParameterFromConfiguration(t *testing.T) {

	var iniConfig = `
[mysqld]
innodb-buffer-pool-size=512M
log-bin=master-bin
gtid_mode=OFF
consensus_auto_leader_transfer=ON

log_error=/data/mysql/log/mysqld.err
character-sets-dir=/usr/share/mysql-8.0/charsets
datadir=/data/mysql/data
port=3306
general_log=1
general_log_file=/data/mysql/mysqld.log
pid-file=/data/mysql/run/mysqld.pid
server-id=1
slow_query_log=1
#slow_query_log_file=/data/mysql/mysqld-slow.log2
slow_query_log_file=/data/mysql/mysqld-slow.log
socket=/data/mysql/tmp/mysqld.sock
ssl-ca=/data/mysql/std_data/cacert.pem
ssl-cert=/data/mysql/std_data/server-cert.pem
ssl-key=/data/mysql/std_data/server-key.pem
tmpdir=/data/mysql/tmp/
loose-sha256_password_auto_generate_rsa_keys=0
loose-caching_sha2_password_auto_generate_rsa_keys=0
secure-file-priv=/data/mysql

[mysqld_test]
innodb-buffer-pool-size=512M
log-bin=master-bin
gtid_mode=OFF
consensus_auto_leader_transfer=ON
slow_query_log_file=/data/mysql/mysqld-slow-test.log


[client]
socket=/data/mysql/tmp/mysqld.sock
host=localhost
`

	cm := corev1.ConfigMap{
		Data: map[string]string{
			"my.cnf": iniConfig,
		},
	}

	// empty test
	{
		emptyCM := corev1.ConfigMap{}
		_, err := GetParameterFromConfiguration(&emptyCM, false,
			"$..slow_query_log_file",
		)
		require.NotNil(t, err)

		_, err = GetParameterFromConfiguration(nil, true,
			"$..slow_query_log_file",
		)
		require.NotNil(t, err)
	}

	// for normal test
	results, err := GetParameterFromConfiguration(&cm, false,
		"$..slow_query_log",
		"$..slow_query_log_file",
		"$..log_output",
		"$..long_query_time",
		"$..log_error",
	)

	require.Nil(t, err)
	require.Equal(t, 5, len(results))

	var (
		slowQueryLog     []interface{}
		slowQueryLogFile []interface{}
		logOutput        []interface{}
		longQueryTime    []interface{}
		logError         []interface{}
	)

	require.Nil(t, json.Unmarshal([]byte(results[0]), &slowQueryLog))
	require.Nil(t, json.Unmarshal([]byte(results[1]), &slowQueryLogFile))
	require.Nil(t, json.Unmarshal([]byte(results[2]), &logOutput))
	require.Nil(t, json.Unmarshal([]byte(results[3]), &longQueryTime))
	require.Nil(t, json.Unmarshal([]byte(results[4]), &logError))

	require.Equal(t, len(slowQueryLog), 1)
	require.Equal(t, len(slowQueryLogFile), 2)
	require.Equal(t, len(logOutput), 0)
	require.Equal(t, len(longQueryTime), 0)
	require.Equal(t, len(logError), 1)

	require.Equal(t, "1", slowQueryLog[0])
	require.Equal(t, "/data/mysql/mysqld-slow.log", slowQueryLogFile[0])
	require.Equal(t, "/data/mysql/mysqld-slow-test.log", slowQueryLogFile[1])
	require.Equal(t, "/data/mysql/log/mysqld.err", logError[0])

	// multi file
	{
		cm := corev1.ConfigMap{
			Data: map[string]string{
				"my.cnf":  iniConfig,
				"my2.cnf": iniConfig,
			},
		}

		// for normal test
		results, err := GetParameterFromConfiguration(&cm, true,
			"$..slow_query_log",
			"$..slow_query_log_file",
			"$..log_output",
			"$..long_query_time",
			"$..log_error",
		)

		require.Nil(t, err)
		require.Equal(t, 5, len(results))

		var (
			slowQueryLog     []interface{}
			slowQueryLogFile []interface{}
			logOutput        []interface{}
			longQueryTime    []interface{}
			logError         []interface{}
		)

		require.Nil(t, json.Unmarshal([]byte(results[0]), &slowQueryLog))
		require.Nil(t, json.Unmarshal([]byte(results[1]), &slowQueryLogFile))
		require.Nil(t, json.Unmarshal([]byte(results[2]), &logOutput))
		require.Nil(t, json.Unmarshal([]byte(results[3]), &longQueryTime))
		require.Nil(t, json.Unmarshal([]byte(results[4]), &logError))

		require.Equal(t, len(slowQueryLog), 2)
		require.Equal(t, len(slowQueryLogFile), 4)
		require.Equal(t, len(logOutput), 0)
		require.Equal(t, len(longQueryTime), 0)
		require.Equal(t, len(logError), 2)

		require.Equal(t, "1", slowQueryLog[0])
		require.Equal(t, "/data/mysql/mysqld-slow.log", slowQueryLogFile[0])
		require.Equal(t, "/data/mysql/log/mysqld.err", logError[0])
	}
}
