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
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/certs"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/module"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/etcd"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/filesystem"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/dns"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/network"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/tasks"
)

func NewCreateK8sClusterForKubeblocks(o *clusterOptions) []module.Module {
	// TODO: add a new module to check if the cluster is already installed
	return []module.Module{
		&precheck.GreetingsModule{},
		&tasks.CheckNodeArchitectureModule{},
		&precheck.NodePreCheckModule{},
		// install kubekey required packages
		&tasks.InstallDependenciesModule{},
		// &precheck.NodePreCheckModule{},
		// &confirm.InstallConfirmModule{},
		// &os.RepositoryModule{Skip: !runtime.Arg.InstallPackages},
		// &binaries.NodeBinariesModule{},
		&tasks.PrepareK8sBinariesModule{BinaryVersion: o.version},
		&tasks.ConfigureOSModule{},
		// &os.ConfigureOSModule{},
		// &customscripts.CustomScriptsModule{Phase: "PreInstall", Scripts: runtime.Cluster.System.PreInstall},
		&kubernetes.StatusModule{},
		// &container.InstallContainerModule{},
		&tasks.InstallCRIModule{},
		&etcd.PreCheckModule{},
		&etcd.CertsModule{},
		&etcd.InstallETCDBinaryModule{},
		&etcd.ConfigureModule{},
		&etcd.BackupModule{},
		&kubernetes.InstallKubeBinariesModule{},
		// init kubeVip on first master
		// &loadbalancer.KubevipModule{},
		&kubernetes.InitKubernetesModule{},
		&dns.ClusterDNSModule{},
		&kubernetes.StatusModule{},
		&kubernetes.JoinNodesModule{},
		// deploy kubeVip on other masters
		// &loadbalancer.KubevipModule{},
		// &loadbalancer.HaproxyModule{},
		&network.DeployNetworkPluginModule{},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !o.autoRenewCerts},
		&kubernetes.SecurityEnhancementModule{Skip: !o.securityEnhancement},
		&kubernetes.SaveKubeConfigModule{},
		&plugins.DeployPluginsModule{},
		// &customscripts.CustomScriptsModule{Phase: "PostInstall", Scripts: runtime.Cluster.System.PostInstall},
	}
}
