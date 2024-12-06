/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
