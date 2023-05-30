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

package tasks

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/confirm"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"

	"github.com/mitchellh/mapstructure"
)

type InstallDependenciesModule struct {
	common.KubeModule
}

func (i *InstallDependenciesModule) isNotReadyHosts() []connector.Host {
	hosts := make([]connector.Host, 0)
	for _, host := range i.Runtime.GetAllHosts() {
		var result confirm.PreCheckResults
		if v, ok := host.GetCache().Get(common.NodePreCheck); ok {
			_ = mapstructure.Decode(v, &result)
			if result.Socat == "" || result.Conntrack == "" || result.Ipset == "" || result.Chronyd == "" || result.Ebtables == "" {
				hosts = append(hosts, host)
			}
		}
	}
	return hosts
}

// Init install dependencies module
func (i *InstallDependenciesModule) Init() {
	i.Name = "InstallDependenciesModule"
	i.Desc = "install dependencies"

	hosts := i.isNotReadyHosts()
	if len(hosts) == 0 {
		logger.Log.Info("All hosts are ready, skip install dependencies")
		return
	}

	prepareOSData := &task.RemoteTask{
		Name:     "GetOSData",
		Desc:     "Get OS release",
		Hosts:    i.Runtime.GetAllHosts(),
		Action:   new(os.GetOSData),
		Parallel: true,
	}

	installPkg := &task.RemoteTask{
		Name:     "InstallDependenciesModule",
		Desc:     "check and install dependencies",
		Hosts:    hosts,
		Action:   new(InstallDependenciesTask),
		Parallel: true,
	}

	i.Tasks = []task.Interface{
		prepareOSData,
		installPkg,
	}
}

type CheckNodeArchitectureModule struct {
	common.KubeModule
}

// Init install dependencies module
func (i *CheckNodeArchitectureModule) Init() {
	i.Name = "CheckNodeArch"
	i.Desc = "check and update host arch"

	prepareNodeArch := &task.RemoteTask{
		Name:     "CheckNodeArch",
		Desc:     "check and update node arch",
		Hosts:    i.Runtime.GetAllHosts(),
		Action:   new(UpdateNodeTask),
		Parallel: true,
	}

	i.Tasks = []task.Interface{
		prepareNodeArch,
	}
}
