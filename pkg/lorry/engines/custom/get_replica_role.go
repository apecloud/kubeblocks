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

package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

var perNodeRegx = regexp.MustCompile("^[^,]*$")

func (mgr *Manager) GetReplicaRole(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	if mgr.actionSvcPorts != nil && len(*mgr.actionSvcPorts) > 0 {
		return mgr.GetReplicaRoleThroughASMAction(ctx, cluster)
	}
	return mgr.GetReplicaRoleThroughCommands(ctx, cluster)
}

// GetReplicaRoleThroughCommands provides the following dedicated environment variables for the action:
//
// - KB_POD_FQDN: The pod FQDN of the replica to check the role.
// - KB_SERVICE_PORT: The port on which the DB service listens.
// - KB_SERVICE_USER: The username used to access the DB service and retrieve the role information with sufficient privileges.
// - KB_SERVICE_PASSWORD: The password of the user used to access the DB service and retrieve the role information.
func (mgr *Manager) GetReplicaRoleThroughCommands(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	roleProbeCmd, ok := mgr.actionCommands[constant.RoleProbeAction]
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
	return util.ExecCommand(ctx, roleProbeCmd, os.Environ())
}

func (mgr *Manager) GetReplicaRoleThroughASMAction(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	var (
		lastOutput []byte
		err        error
	)

	for _, port := range *mgr.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_RSM_LAST_STDOUT=%s", port, url.QueryEscape(string(lastOutput)))
		lastOutput, err = mgr.callAction(ctx, u)
		if err != nil {
			return "", err
		}
		mgr.Logger.Info("action succeed", "url", u, "output", string(lastOutput))
	}
	finalOutput := strings.TrimSpace(string(lastOutput))

	if perNodeRegx.MatchString(finalOutput) {
		return finalOutput, nil
	}

	// csv format: term,podName,role
	parseCSV := func(input string) (string, error) {
		res := common.GlobalRoleSnapshot{}
		lines := strings.Split(input, "\n")
		for _, line := range lines {
			fields := strings.Split(strings.TrimSpace(line), ",")
			if len(fields) != 3 {
				return "", err
			}
			res.Version = strings.TrimSpace(fields[0])
			pair := common.PodRoleNamePair{
				PodName:  strings.TrimSpace(fields[1]),
				RoleName: strings.ToLower(strings.TrimSpace(fields[2])),
			}
			res.PodRoleNamePairs = append(res.PodRoleNamePairs, pair)
		}
		resByte, err := json.Marshal(res)
		return string(resByte), err
	}
	return parseCSV(finalOutput)
}

// callAction sends an HTTP POST request to the specified URL and returns the response body.
// It takes a context.Context and the URL as input parameters.
// The function returns the response body as a byte slice and an error if any.
func (mgr *Manager) callAction(ctx context.Context, url string) ([]byte, error) {
	// construct http request
	request, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	// send http request
	resp, err := mgr.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// parse http response
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("received status code %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, err
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
