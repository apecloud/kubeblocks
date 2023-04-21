/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

import "strings"

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
		examples: map[ClientType]buildConnectExample{},
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
	// TODO implement me
	panic("implement me")
}

var _ Interface = &redis{}
