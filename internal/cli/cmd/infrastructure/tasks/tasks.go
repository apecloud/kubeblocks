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
	"strings"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/util/osrelease"
)

type InstallDependenciesTask struct {
	common.KubeAction
	pkg []string
}

var dependenciesPkg = []string{"socat", "conntrack", "ipset", "ebtables", "chrony", "iptables", "curl", "ipvsadm"}

func (i *InstallDependenciesTask) Execute(runtime connector.Runtime) (err error) {
	host := runtime.RemoteHost()
	release, ok := host.GetCache().Get(os.Release)
	if !ok {
		return cfgcore.MakeError("failed to get os release.")
	}

	r := release.(*osrelease.Data)
	installCommand, err := checkRepositoryInstallerCommand(r, runtime)
	if err != nil {
		return err
	}
	depPkg := strings.Join(append(i.pkg, dependenciesPkg...), " ")
	stdout, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("%s %s", installCommand, depPkg), true)
	logger.Log.Info(stdout)
	return err
}

type UpdateNodeTask struct {
	common.KubeAction
}

func (i *UpdateNodeTask) Execute(runtime connector.Runtime) (err error) {
	host := runtime.RemoteHost()
	if host.GetArch() != "" {
		return nil
	}

	stdout, err := runtime.GetRunner().Cmd("uname -m", false)
	if err != nil {
		return err
	}
	host.SetArch(parseNodeArchitecture(stdout))
	updateClusterSpecHost(i.KubeConf.Cluster, host)
	return nil
}

func updateClusterSpecHost(clusterSpec *kubekeyapiv1alpha2.ClusterSpec, host connector.Host) {
	for i := range clusterSpec.Hosts {
		h := &clusterSpec.Hosts[i]
		if h.Name == host.GetName() {
			h.Arch = host.GetArch()
		}
	}
}

func parseNodeArchitecture(stdout string) string {
	switch strings.TrimSpace(stdout) {
	default:
		return "amd64"
	case "x86_64":
		return "amd64"
	case "arm64", "arm":
		return "arm64"
	case "aarch64", "aarch32":
		return "arm"
	}
}

func checkRepositoryInstallerCommand(osData *osrelease.Data, runtime connector.Runtime) (string, error) {
	const (
		debianCommand = "apt install -y"
		rhelCommand   = "yum install -y"
	)

	isDebianCore := func() bool {
		checkDeb, err := runtime.GetRunner().SudoCmd("which apt", false)
		return err == nil && strings.Contains(checkDeb, "bin")
	}
	isRhelCore := func() bool {
		checkDeb, err := runtime.GetRunner().SudoCmd("which yum", false)
		return err == nil && strings.Contains(checkDeb, "bin")
	}

	switch strings.ToLower(osData.ID) {
	case "ubuntu", "debian":
		return debianCommand, nil
	case "centos", "rhel":
		return rhelCommand, nil
	}

	switch {
	default:
		return "", cfgcore.MakeError("failed to check apt or yum.")
	case isRhelCore():
		return rhelCommand, nil
	case isDebianCore():
		return debianCommand, nil
	}
}
