/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package redis

import (
	"fmt"
	"strings"

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
			Client:    "redis-cli",
			Container: "redis",
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			// Display Username and Password in kbcli
			models.CLI: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# redis client connection example
redis-cli -h %s -p %s -a %s
`, info.Host, info.Port, info.Password)
			},
		},
	}
}

func (r Commands) ConnectCommand(connectInfo *engines.AuthInfo) []string {
	redisCmd := []string{
		"redis-cli",
	}

	if connectInfo != nil {
		redisCmd = append(redisCmd, "-a", engines.AddSingleQuote(connectInfo.UserPasswd))
	}
	return []string{"sh", "-c", strings.Join(redisCmd, " ")}
}

func (r Commands) Container() string {
	return r.info.Container
}

func (r Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	return engines.BuildExample(info, client, r.examples)
}

func (r Commands) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	args := []string{}
	cmd = append(cmd, "/bin/sh", "-c")
	for _, script := range scripts {
		args = append(args, fmt.Sprintf("%s -h %s -p 6379 -a %s %s", r.info.Client,
			fmt.Sprintf("$%s", engines.EnvVarMap[engines.HOST]),
			fmt.Sprintf("$%s", engines.EnvVarMap[engines.PASSWORD]), script))
	}
	cmd = append(cmd, strings.Join(args, " && "))
	return cmd, nil, nil
}
