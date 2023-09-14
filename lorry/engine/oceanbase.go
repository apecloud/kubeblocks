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
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var _ ClusterCommands = &oceanbase{}

type oceanbase struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newOceanbase() *oceanbase {
	return &oceanbase{
		info: EngineInfo{
			Client: "mysql",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# oceanbase client connection example
mysql -h %s -P %s -u %s
`, info.Host, info.Port, info.User)
			},
		},
	}
}

func (r *oceanbase) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := "root"
	userPass := ""

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	var obCmd []string

	if userPass != "" {
		obCmd = []string{fmt.Sprintf("%s -P2881 -u%s -A -p%s", r.info.Client, userName, userPass)}
	} else {
		obCmd = []string{fmt.Sprintf("%s -P2881 -u%s -A", r.info.Client, userName)}
	}

	return []string{"bash", "-c", strings.Join(obCmd, " ")}
}

func (r *oceanbase) Container() string {
	return r.info.Container
}

func (r *oceanbase) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, r.examples)
}

func (r *oceanbase) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	cmd = append(cmd, "/bin/bash", "-c", "-ex")
	if envVarMap[password] == "" {
		cmd = append(cmd, fmt.Sprintf("%s -P2881 -u%s -e %s", r.info.Client, envVarMap[user], strconv.Quote(strings.Join(scripts, " "))))
	} else {
		cmd = append(cmd, fmt.Sprintf("%s -P2881 -u%s -p%s -e %s", r.info.Client,
			fmt.Sprintf("$%s", envVarMap[user]),
			fmt.Sprintf("$%s", envVarMap[password]),
			strconv.Quote(strings.Join(scripts, " "))))
	}

	return cmd, nil, nil
}
