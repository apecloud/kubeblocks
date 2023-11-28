/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package engine

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

var _ ClusterCommands = &oracle{}

type oracle struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

// sqlplus sys/$ORACLE_PWD@//localhost:1521/$ORACLE_SID as sysdba
func newOracle() *oracle {
	return &oracle{
		info: EngineInfo{
			Client:      "sqlplus",
			PasswordEnv: "$ORACLE_PWD",
			Database:    "$ORACLE_SID",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# sqlplus client connection example
sqlplus sys/%s@//localhost:1521/%s as sysdba`, info.Password, info.Database)
			},
		},
	}
}

func (c *oracle) ConnectCommand(info *AuthInfo) []string {
	userPass := c.info.PasswordEnv
	serviceSID := c.info.Database
	dsn := fmt.Sprintf("sqlplus sys/%s@//localhost:1521/%s as sysdba", userPass, serviceSID)
	return []string{"sh", "-c", dsn}
}

func (c *oracle) Container() string {
	return c.info.Container
}

func (c *oracle) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, c.examples)
}

func (c *oracle) ExecuteCommand(strings []string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("opengauss execute cammand interface do not implement")
}
