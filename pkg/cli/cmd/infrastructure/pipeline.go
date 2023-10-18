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

package infrastructure

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/certs"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/container"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/module"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/etcd"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/filesystem"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/dns"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/network"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/tasks"
)

func NewCreatePipeline(o *createOptions) []module.Module {
	return []module.Module{
		&precheck.GreetingsModule{},
		&tasks.CheckNodeArchitectureModule{},
		&precheck.NodePreCheckModule{},
		&tasks.InstallDependenciesModule{},
		&tasks.PrepareK8sBinariesModule{BinaryVersion: o.version},
		&tasks.ConfigureNodeOSModule{Nodes: o.Nodes},
		&kubernetes.StatusModule{},
		&tasks.InstallCRIModule{SandBoxImage: o.Cluster.Kubernetes.CRI.SandBoxImage},
		&etcd.PreCheckModule{},
		&etcd.CertsModule{},
		&etcd.InstallETCDBinaryModule{},
		&etcd.ConfigureModule{},
		&etcd.BackupModule{},
		&kubernetes.InstallKubeBinariesModule{},
		&kubernetes.InitKubernetesModule{},
		&dns.ClusterDNSModule{},
		&kubernetes.StatusModule{},
		&tasks.SaveKubeConfigModule{OutputKubeconfig: o.outputKubeconfig},
		&kubernetes.JoinNodesModule{},
		&network.DeployNetworkPluginModule{},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&kubernetes.SecurityEnhancementModule{Skip: !o.securityEnhancement},
		&tasks.AddonsInstaller{Addons: o.Addons, Kubeconfig: o.outputKubeconfig},
	}
}

func NewDeletePipeline(o *deleteOptions) []module.Module {
	return []module.Module{
		&precheck.GreetingsModule{},
		&kubernetes.ResetClusterModule{},
		&container.UninstallContainerModule{Skip: !o.deleteCRI},
		&os.ClearOSEnvironmentModule{},
		&certs.UninstallAutoRenewCertsModule{},
	}
}
