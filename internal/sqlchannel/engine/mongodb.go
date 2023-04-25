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

type mongodb struct {
	info     EngineInfo
	examples map[ClientType]buildConnectExample
}

func newMongoDB() *mongodb {
	return &mongodb{
		info: EngineInfo{
			Client:      "mongosh",
			Container:   "mongodb",
			UserEnv:     "$MONGODB_ROOT_USER",
			PasswordEnv: "$MONGODB_ROOT_PASSWORD",
		},
		examples: map[ClientType]buildConnectExample{},
	}
}

func (r mongodb) ConnectCommand(connectInfo *AuthInfo) []string {
	userName := r.info.UserEnv
	userPass := r.info.PasswordEnv

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	mongodbCmd := []string{fmt.Sprintf("%s mongodb://%s:%s@$KB_POD_FQDN:27017/admin?replicaSet=$KB_CLUSTER_COMP_NAME", r.info.Client, userName, userPass)}

	return []string{"sh", "-c", strings.Join(mongodbCmd, " ")}
}

func (r mongodb) Container() string {
	return r.info.Container
}

func (r mongodb) ConnectExample(info *ConnectionInfo, client string) string {
	// TODO implement me
	panic("implement me")
}

var _ Interface = &mongodb{}
