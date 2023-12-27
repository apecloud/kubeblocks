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

package oceanbase

import (
	"fmt"
	"strconv"
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
			Client: "mysql",
		},
		examples: map[models.ClientType]engines.BuildConnectExample{
			models.CLI: func(info *engines.ConnectionInfo) string {
				return fmt.Sprintf(`# oceanbase client connection example
mysql -h %s -P $COMP_MYSQL_PORT -u %s
`, info.Host, info.User)
			},
		},
	}
}

func (r *Commands) ConnectCommand(connectInfo *engines.AuthInfo) []string {
	userName := "root"
	userPass := ""

	if connectInfo != nil {
		userName = connectInfo.UserName
		userPass = connectInfo.UserPasswd
	}

	var obCmd []string

	if userPass != "" {
		obCmd = []string{fmt.Sprintf("%s -h127.0.0.1 -P $OB_SERVICE_PORT -u%s -A -p%s", r.info.Client, userName, engines.AddSingleQuote(userPass))}
	} else {
		obCmd = []string{fmt.Sprintf("%s -h127.0.0.1 -P $OB_SERVICE_PORT -u%s -A", r.info.Client, userName)}
	}

	return []string{"bash", "-c", strings.Join(obCmd, " ")}
}

func (r *Commands) Container() string {
	return r.info.Container
}

func (r *Commands) ConnectExample(info *engines.ConnectionInfo, client string) string {
	return engines.BuildExample(info, client, r.examples)
}

func (r *Commands) ExecuteCommand(scripts []string) ([]string, []corev1.EnvVar, error) {
	cmd := []string{}
	cmd = append(cmd, "/bin/bash", "-c", "-ex")
	if engines.EnvVarMap[engines.PASSWORD] == "" {
		cmd = append(cmd, fmt.Sprintf("%s -h127.0.0.1 -P $COMP_MYSQL_PORT -u%s -e %s", r.info.Client, engines.EnvVarMap[engines.USER], strconv.Quote(strings.Join(scripts, " "))))
	} else {
		cmd = append(cmd, fmt.Sprintf("%s -h127.0.0.1 -P $COMP_MYSQL_PORT -u%s -p%s -e %s", r.info.Client,
			fmt.Sprintf("$%s", engines.EnvVarMap[engines.USER]),
			fmt.Sprintf("$%s", engines.EnvVarMap[engines.PASSWORD]),
			strconv.Quote(strings.Join(scripts, " "))))
	}

	return cmd, nil, nil
}
