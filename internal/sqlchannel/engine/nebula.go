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

var _ ClusterCommands = &nebula{}

type nebula struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newNebula() *nebula {
	return &nebula{
		info: EngineInfo{
			Client:    "nebula-console",
			Container: "nebula-console",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# nebula client connection example
nebula --addr %s --port %s --user %s -port%s
`, info.Host, info.Port, info.User, info.Password)
			},
		},
	}
}

func (m *nebula) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := "root"
	userPass := "nebula"

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	nebulaCmd := []string{fmt.Sprintf("%s --addr $GRAPHD_SVC_NAME --port $GRAPHD_SVC_PORT --user %s --password %s", m.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(nebulaCmd, " ")}
}

func (m *nebula) Container() string {
	return m.info.Container
}

func (m *nebula) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, m.examples)
}

func (m *nebula) ExecuteCommand([]string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("%s not implemented", m.info.Client)
}
