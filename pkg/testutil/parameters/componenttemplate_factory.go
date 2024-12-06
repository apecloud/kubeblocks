/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	corev1 "k8s.io/api/core/v1"

	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockComponentTemplateFactory struct {
	testapps.BaseFactory[corev1.ConfigMap, *corev1.ConfigMap, MockComponentTemplateFactory]
}

func NewComponentTemplateFactory(name, ns string) *MockComponentTemplateFactory {
	f := &MockComponentTemplateFactory{}
	f.Init(ns, name, &corev1.ConfigMap{
		Data: map[string]string{
			MysqlConfigFile: mysqConfig,
		},
	}, f)
	return f
}

func (f *MockComponentTemplateFactory) AddConfigFile(key, value string) *MockComponentTemplateFactory {
	f.Get().Data[key] = value
	return f
}

var MysqlConfigFile = "my.cnf"
var mysqConfig = `
[mysqld]
innodb-buffer-pool-size=512M
log-bin=master-bin
gtid_mode=OFF
consensus_auto_leader_transfer=ON

pid-file=/var/run/mysqld/mysqld.pid
socket=/var/run/mysqld/mysqld.sock

port=3306
general_log=0
server-id=1
slow_query_log=0

[client]
socket=/var/run/mysqld/mysqld.sock
host=localhost
`
