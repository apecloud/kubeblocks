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
	"testing"

	"github.com/stretchr/testify/require"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

func TestCueLang(t *testing.T) {

	var (
		mysqlCfg = `
	[mysqld]
	innodb-buffer-pool-size=512M
	log-bin=master-bin
	gtid_mode=OFF
	consensus_auto_leader_transfer=ON

	log-error=/data/mysql/log/mysqld.err
	character-sets-dir=/usr/share/mysql-8.0/charsets
	datadir=/data/mysql/data
	port=3306
	general_log=1
	general_log_file=/data/mysql/mysqld.log
	pid-file=/data/mysql/run/mysqld.pid
	server-id=1
	slow_query_log=1
	slow_query_log_file=/data/mysql/mysqld-slow.log
	socket=/data/mysql/tmp/mysqld.sock
	ssl-ca=/data/mysql/std_data/cacert.pem
	ssl-cert=/data/mysql/std_data/server-cert.pem
	ssl-key=/data/mysql/std_data/server-key.pem
	tmpdir=/data/mysql/tmp/
	loose-sha256_password_auto_generate_rsa_keys=0
	loose-caching_sha2_password_auto_generate_rsa_keys=0
	secure-file-priv=/data/mysql

	[client]
	socket=/data/mysql/tmp/mysqld.sock
	host=localhost
	`

		mysqlCfgTpl = `
#MysqlParameter: {
	[SectionName=_]: {
		// SectionName is extract section name

		// [OFF|ON] default ON
		if SectionName != "client" {
			automatic_sp_privileges: string & "OFF" | "ON" | *"ON"
		}

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
	)

	require.Nil(t, CueValidate(mysqlCfgTpl))

	err := ValidateConfigurationWithCue(mysqlCfgTpl, dbaasv1alpha1.INI, mysqlCfg)
	require.Nil(t, err)
}
