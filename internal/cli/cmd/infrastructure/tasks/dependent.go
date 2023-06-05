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
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/container"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/container/templates"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/prepare"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/images"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/registry"
	"github.com/mitchellh/mapstructure"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
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
			if result.Socat == "" ||
				result.Conntrack == "" ||
				result.Curl == "" ||
				result.Ipset == "" ||
				result.Chronyd == "" ||
				result.Ipvsadm == "" ||
				result.Ebtables == "" {
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

	i.Tasks = []task.Interface{
		&task.RemoteTask{
			Name:     "GetOSData",
			Desc:     "Get OS release",
			Hosts:    i.Runtime.GetAllHosts(),
			Action:   new(os.GetOSData),
			Parallel: true,
		},
		&task.RemoteTask{
			Name:     "InstallDependenciesModule",
			Desc:     "check and install dependencies",
			Hosts:    hosts,
			Action:   new(InstallDependenciesTask),
			Parallel: true,
		},
	}
}

type CheckNodeArchitectureModule struct {
	common.KubeModule
}

// Init install dependencies module
func (i *CheckNodeArchitectureModule) Init() {
	i.Name = "CheckNodeArch"
	i.Desc = "check and update host arch"
	i.Tasks = []task.Interface{
		&task.RemoteTask{
			Name:     "CheckNodeArch",
			Desc:     "check and update node arch",
			Hosts:    i.Runtime.GetAllHosts(),
			Action:   new(UpdateNodeTask),
			Parallel: true,
		},
	}
}

type InstallCRIModule struct {
	common.KubeModule
}

func (i *InstallCRIModule) Init() {
	i.Name = "InstallContainerModule"
	i.Desc = "Install container manager"

	syncContainerd := &task.RemoteTask{
		Name:  "SyncContainerd",
		Desc:  "Sync containerd binaries",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.ContainerdExist{Not: true},
		},
		Action:   new(container.SyncContainerd),
		Parallel: true,
		Retry:    2,
	}

	syncCrictlBinaries := &task.RemoteTask{
		Name:  "SyncCrictlBinaries",
		Desc:  "Sync crictl binaries",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.CrictlExist{Not: true},
		},
		Action:   new(container.SyncCrictlBinaries),
		Parallel: true,
		Retry:    2,
	}

	generateContainerdService := &task.RemoteTask{
		Name:  "GenerateContainerdService",
		Desc:  "Generate containerd service",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.ContainerdExist{Not: true},
		},
		Action: &builder.Template{
			Template: ContainerdService,
			Dst:      ContainerdServiceInstallPath,
		},
		Parallel: true,
	}

	generateContainerdConfig := &task.RemoteTask{
		Name:  "GenerateContainerdConfig",
		Desc:  "Generate containerd config",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.ContainerdExist{Not: true},
		},
		Action: &builder.Template{
			Template: ContainerdConfig,
			Dst:      ContainerdConfigInstallPath,
			Values: gotemplate.TplValues{
				"Mirrors":            templates.Mirrors(i.KubeConf),
				"InsecureRegistries": i.KubeConf.Cluster.Registry.InsecureRegistries,
				"SandBoxImage":       images.GetImage(i.Runtime, i.KubeConf, "pause").ImageName(),
				"Auths":              registry.DockerRegistryAuthEntries(i.KubeConf.Cluster.Registry.Auths),
				"DataRoot":           templates.DataRoot(i.KubeConf),
			}},
		Parallel: true,
	}

	generateCrictlConfig := &task.RemoteTask{
		Name:  "GenerateCrictlConfig",
		Desc:  "Generate crictl config",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.ContainerdExist{Not: true},
		},
		Action: &builder.Template{
			Template: CRICtlConfig,
			Dst:      CRICtlConfigInstallPath,
			Values: gotemplate.TplValues{
				"Endpoint": i.KubeConf.Cluster.Kubernetes.ContainerRuntimeEndpoint,
			}},
		Parallel: true,
	}

	enableContainerd := &task.RemoteTask{
		Name:  "EnableContainerd",
		Desc:  "Enable containerd",
		Hosts: i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&container.ContainerdExist{Not: true},
		},
		Action:   new(container.EnableContainerd),
		Parallel: true,
	}

	i.Tasks = []task.Interface{
		syncContainerd,
		syncCrictlBinaries,
		generateContainerdService,
		generateContainerdConfig,
		generateCrictlConfig,
		enableContainerd,
	}
}
