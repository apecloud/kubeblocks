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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var _ ClusterCommands = &redis{}

type redis struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newRedis() *redis {
	return &redis{
		info: EngineInfo{
			Client:    "redis-cli",
			Container: "redis",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# redis client connection example
redis-cli -h %s -p %s
`, info.Host, info.Port)
			},
		},
	}
}

func (r redis) ConnectCommand(connectInfo *AuthInfo) []string {
	redisCmd := []string{
		"redis-cli",
	}

	if connectInfo != nil {
		redisCmd = append(redisCmd, "--user", connectInfo.UserName)
		redisCmd = append(redisCmd, "--pass", connectInfo.UserPasswd)
	}
	return []string{"sh", "-c", strings.Join(redisCmd, " ")}
}

func (r redis) Container() string {
	return r.info.Container
}

func (r redis) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, r.examples)
}

func (r redis) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	args := []string{}
	cmd = append(cmd, "/bin/sh", "-c")
	for _, script := range scripts {
		args = append(args, fmt.Sprintf("%s -h %s -p 6379 --user %s --pass %s %s", r.info.Client,
			fmt.Sprintf("$%s", envVarMap[host]),
			fmt.Sprintf("$%s", envVarMap[user]),
			fmt.Sprintf("$%s", envVarMap[password]), script))
	}
	cmd = append(cmd, strings.Join(args, " && "))
	return cmd, nil, nil
}
