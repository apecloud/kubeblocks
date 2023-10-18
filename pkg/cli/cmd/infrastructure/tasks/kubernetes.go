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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/StudioSol/set"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os/templates"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/constant"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/types"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
)

type PrepareK8sBinariesModule struct {
	common.KubeModule

	// kubernetes version
	BinaryVersion types.InfraVersionInfo
}

type ConfigureNodeOSModule struct {
	common.KubeModule
	Nodes []types.ClusterNode
}

type SaveKubeConfigModule struct {
	common.KubeModule

	OutputKubeconfig string
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

func (c *ConfigureNodeOSModule) Init() {
	c.Name = "ConfigureNodeOSModule"
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
			Action: &NodeScriptGenerator{
				Hosts: templates.GenerateHosts(c.Runtime, c.KubeConf),
				Nodes: c.Nodes,
			},
			Parallel: true,
		},
		&task.RemoteTask{
			Name:     "ExecScript",
			Desc:     "Exec init os script",
			Hosts:    c.Runtime.GetAllHosts(),
			Action:   new(os.NodeExecScript),
			Parallel: true,
		}}
}

func (p *SaveKubeConfigModule) Init() {
	p.Name = "SaveKubeConfigModule"
	p.Desc = "Save kube config to local file"

	p.Tasks = []task.Interface{
		&task.LocalTask{
			Name:   "SaveKubeConfig",
			Desc:   "Save kube config to local file",
			Action: &SaveKubeConfig{outputKubeconfig: p.OutputKubeconfig},
		}}
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

type SaveKubeConfig struct {
	common.KubeAction

	outputKubeconfig string
}

func (c *SaveKubeConfig) Execute(runtime connector.Runtime) error {
	master := runtime.GetHostsByRole(common.Master)[0]

	status, ok := c.PipelineCache.Get(common.ClusterStatus)
	if !ok {
		return cfgcore.MakeError("failed to get kubernetes status.")
	}
	cluster := status.(*kubernetes.KubernetesStatus)
	kubeConfigStr := cluster.KubeConfig
	kc, err := clientcmd.Load([]byte(kubeConfigStr))
	if err != nil {
		return err
	}
	updateClusterAPIServer(kc, master, c.KubeConf.Cluster.ControlPlaneEndpoint)
	kcFile := GetDefaultConfig()
	existingKC, err := kubeconfigLoad(kcFile)
	if err != nil {
		return err
	}
	if c.outputKubeconfig == "" {
		c.outputKubeconfig = kcFile
	}
	if existingKC != nil {
		return kubeconfigMerge(kc, existingKC, c.outputKubeconfig)
	}
	return kubeconfigWrite(kc, c.outputKubeconfig)
}

type NodeScriptGenerator struct {
	common.KubeAction

	Nodes []types.ClusterNode
	Hosts []string
}

func (c *NodeScriptGenerator) Execute(runtime connector.Runtime) error {
	foundHostOptions := func(nodes []types.ClusterNode, host connector.Host) types.NodeOptions {
		for _, node := range nodes {
			switch {
			default:
				return types.NodeOptions{}
			case node.Name != host.GetName():
			case node.NodeOptions != nil:
				return *node.NodeOptions
			}
		}
		return types.NodeOptions{}
	}

	scriptsTemplate := builder.Template{
		Template: constant.ConfigureOSScripts,
		Dst:      filepath.Join(common.KubeScriptDir, "initOS.sh"),
		Values: gotemplate.TplValues{
			"Hosts":   c.Hosts,
			"Options": foundHostOptions(c.Nodes, runtime.RemoteHost()),
		}}
	return scriptsTemplate.Execute(runtime)
}

func updateClusterAPIServer(kc *clientcmdapi.Config, master connector.Host, endpoint kubekeyapiv1alpha2.ControlPlaneEndpoint) {
	cpePrefix := fmt.Sprintf("https://%s:", endpoint.Domain)
	for _, cluster := range kc.Clusters {
		if strings.HasPrefix(cluster.Server, cpePrefix) {
			cluster.Server = fmt.Sprintf("https://%s:%d", master.GetAddress(), endpoint.Port)
		}
	}
}
