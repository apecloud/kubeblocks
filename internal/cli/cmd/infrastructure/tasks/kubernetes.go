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
	"path/filepath"

	"github.com/StudioSol/set"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os/templates"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

type PrepareK8sBinariesModule struct {
	common.KubeModule

	// kubernetes version
	BinaryVersion types.InfraVersionInfo
}

type ConfigureOSModule struct {
	common.KubeModule
}

func (p *PrepareK8sBinariesModule) Init() {
	p.Name = "PrepareK8sBinariesModule"
	p.Desc = "Download installation binaries for kubernetes"

	p.Tasks = []task.Interface{
		&task.LocalTask{
			Name:   "PrepareK8sBinaries",
			Desc:   "Download installation binaries",
			Action: &DownloadKubernetesBinary{BinaryVersion: p.BinaryVersion},
		}}
}

func (c *ConfigureOSModule) Init() {
	c.Name = "ConfigureOSModule"
	c.Desc = "Init os dependencies"
	c.Tasks = []task.Interface{
		&task.RemoteTask{
			Name:     "GetOSData",
			Desc:     "Get OS release",
			Hosts:    c.Runtime.GetAllHosts(),
			Action:   new(os.GetOSData),
			Parallel: true,
		},
		&task.RemoteTask{
			Name:     "SetHostName",
			Desc:     "Prepare to init OS",
			Hosts:    c.Runtime.GetAllHosts(),
			Action:   new(os.NodeConfigureOS),
			Parallel: true,
		},
		&task.RemoteTask{
			Name:  "GenerateScript",
			Desc:  "Generate init os script",
			Hosts: c.Runtime.GetAllHosts(),
			Action: &builder.Template{
				Template: ConfigureOSScripts,
				Dst:      filepath.Join(common.KubeScriptDir, "initOS.sh"),
				Values: gotemplate.TplValues{
					"Hosts": templates.GenerateHosts(c.Runtime, c.KubeConf),
				}},
			Parallel: true,
		},
		&task.RemoteTask{
			Name:     "ExecScript",
			Desc:     "Exec init os script",
			Hosts:    c.Runtime.GetAllHosts(),
			Action:   new(os.NodeExecScript),
			Parallel: true,
		},
		&task.RemoteTask{
			Name:     "ConfigureNtpServer",
			Desc:     "configure the ntp server for each node",
			Hosts:    c.Runtime.GetAllHosts(),
			Prepare:  new(os.NodeConfigureNtpCheck),
			Action:   new(os.NodeConfigureNtpServer),
			Parallel: true,
		},
	}
}

type DownloadKubernetesBinary struct {
	common.KubeAction
	BinaryVersion types.InfraVersionInfo
}

func (d *DownloadKubernetesBinary) Execute(runtime connector.Runtime) error {
	archSet := set.NewLinkedHashSetString()
	for _, host := range runtime.GetAllHosts() {
		archSet.Add(host.GetArch())
	}

	for _, arch := range archSet.AsSlice() {
		binariesMap, err := downloadKubernetesBinaryWithArch(runtime.GetWorkDir(), arch, d.BinaryVersion)
		if err != nil {
			return err
		}
		d.PipelineCache.Set(common.KubeBinaries+"-"+arch, binariesMap)
	}
	return nil
}
