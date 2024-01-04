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

package oracle

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
)

var _ engines.ClusterCommands = &Commands{}

type Commands struct {
	info     engines.EngineInfo
	examples map[models.ClientType]engines.BuildConnectExample
}

func NewCommands() engines.ClusterCommands {
	return &Commands{
		info: engines.EngineInfo{
			Client:      "sqlplus",
			PasswordEnv: "$ORACLE_PWD",
			UserEnv:     "sys",
			Database:    "$ORACLE_SID",
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			models.CLI: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# oracle client connection example
sqlplus sys/%s@//localhost:1521/%s as sysdba`, info.Password, info.Database)
			},
		},
	}
}

func (c *Commands) ConnectCommand(info *engines.AuthInfo) []string {
	userName := c.info.UserEnv
	userPass := c.info.PasswordEnv
	dataBase := c.info.Database
	if info != nil {
		userPass = engines.AddSingleQuote(info.UserPasswd)
	}
	return []string{"sh", "-c", fmt.Sprintf("%s %s/%s@//localhost:1521/%s as sysdba", c.info.Client, userName, userPass, dataBase)}
}

func (c *Commands) Container() string {
	return c.info.Container
}

func (c *Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	return engines.BuildExample(info, client, c.examples)
}

func (c *Commands) ExecuteCommand(strings []string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("oracle execute cammand interface do not implement")
}
