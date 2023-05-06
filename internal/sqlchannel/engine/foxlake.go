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
)

type foxlake struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newFoxLake() *foxlake {
	return &foxlake{
		info: EngineInfo{
			Client:      "usql",
			Container:   "foxlake",
			UserEnv:     "$FOXLAKE_ROOT_USER",
			PasswordEnv: "$FOXLAKE_ROOT_PASSWORD",
		},
		examples: map[ClientType]buildConnectExample{
			CLI: func(info *ConnectionInfo) string {
				return fmt.Sprintf(`# foxlake client connection example
mysql -h%s -P%s -u%s -p%s
`, info.Host, info.Port, info.User, info.Password)
			},
		},
	}
}

func (r *foxlake) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := r.info.UserEnv
	userPass := r.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	foxlakeCmd := []string{fmt.Sprintf("%s mysql://%s:%s@:${serverPort}", r.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(foxlakeCmd, " ")}
}

func (r *foxlake) Container() string {
	return r.info.Container
}

func (r *foxlake) ConnectExample(info *ConnectionInfo, client string) string {
	return buildExample(info, client, r.examples)
}
