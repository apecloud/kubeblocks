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

package engine

import (
	"strings"
)

type mysql struct{}

const (
	mysqlEngineName      = "mysql"
	mysqlClient          = "mysql "
	mysqlContainerName   = "mysql"
	mysqlDefaultPassword = "$MYSQL_ROOT_PASSWORD"
)

var _ Interface = &mysql{}

func (m *mysql) ConnectCommand(database string) []string {
	mysqlCmd := []string{mysqlClient}
	if len(database) > 0 {
		mysqlCmd = append(mysqlCmd, "-D", database)
	}
	mysqlCmd = append(mysqlCmd, "-p"+mysqlDefaultPassword)
	return []string{"sh", "-c", strings.Join(mysqlCmd, " ")}
}

func (m *mysql) EngineName() string {
	return mysqlEngineName
}

func (m *mysql) EngineContainer() string {
	return mysqlContainerName
}
