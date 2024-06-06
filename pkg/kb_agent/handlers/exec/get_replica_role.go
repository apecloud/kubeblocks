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

package exec

import (
	"context"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

// GetReplicaRole provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The pod FQDN of the replica to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service and retrieve the role information with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service and retrieve the role information.
func (h *Handler) GetReplicaRole(ctx context.Context) (string, error) {
	roleProbeCmd, ok := h.actionCommands[constant.RoleProbeAction]
	if !ok || len(roleProbeCmd) == 0 {
		return "", errors.New("role probe commands is empty!")
	}

	supportedShells := []string{"sh", "bash", "zsh", "csh", "ksh", "tcsh", "fish"}
	// Check if the cmd is one kind of "sh"
	if !contains(supportedShells, roleProbeCmd[0]) {
		roleProbeCmd = append([]string{"sh", "-c"}, strings.Join(roleProbeCmd, " "))
	}

	// envs, err := util.GetGlobalSharedEnvs()
	// if err != nil {
	// 	return "", err
	// }
	return h.Executor.ExecCommand(ctx, roleProbeCmd, os.Environ())
}

func contains(supportedShells []string, shell string) bool {
	cmds := strings.Split(shell, "/")
	shell = cmds[len(cmds)-1]
	for _, s := range supportedShells {
		if s == shell {
			return true
		}
	}
	return false
}
