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

package mongodb

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
			Client:      "mongosh",
			Container:   "mongodb",
			UserEnv:     "$MONGODB_ROOT_USER",
			PasswordEnv: "$MONGODB_ROOT_PASSWORD",
			Database:    "admin",
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			models.CLI: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# mongodb client connection example
mongosh mongodb://%s:%s@%s/%s
`, info.User, info.Password, info.Host, info.Database)
			},
		},
	}
}

func (r Commands) ConnectCommand(connectInfo *engines.AuthInfo) []string {
	userName := r.info.UserEnv
	userPass := r.info.PasswordEnv
	dsn := fmt.Sprintf("mongodb://%s:%s@$KB_POD_FQDN:27017/admin?replicaSet=$KB_CLUSTER_COMP_NAME", userName, userPass)
	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
		dsn = engines.AddSingleQuote(fmt.Sprintf("mongodb://%s:%s@$KB_POD_FQDN:27017/admin?replicaSet=$KB_CLUSTER_COMP_NAME", userName, userPass))
	}

	mongodbCmd := []string{fmt.Sprintf("export CLIENT=`which mongosh>/dev/null&&echo %s||echo mongo`; $CLIENT %s", r.info.Client, dsn)}
	return []string{"sh", "-c", strings.Join(mongodbCmd, " ")}
}

func (r Commands) Container() string {
	return r.info.Container
}

func (r Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	if len(info.Database) == 0 {
		info.Database = r.info.Database
	}
	return engines.BuildExample(info, client, r.examples)
}

func (r Commands) ExecuteCommand([]string) ([]string, []corev1.EnvVar, error) {
	return nil, nil, fmt.Errorf("%s not implemented", r.info.Client)
}
