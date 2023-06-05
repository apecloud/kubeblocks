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

const (
	ContainerdService  = "containerd.service.tpl"
	ContainerdConfig   = "containerd.config.toml.tpl"
	CRICtlConfig       = "crictl.yaml.tpl"
	ConfigureOSScripts = "init_os.sh.tpl"

	ContainerdServiceInstallPath = "/etc/systemd/system/containerd.service"
	ContainerdConfigInstallPath  = "/etc/containerd/config.toml"
	CRICtlConfigInstallPath      = "/etc/crictl.yaml"

	DefaultK8sVersion        = "v1.26.5" // https://github.com/kubernetes/kubernetes/releases/tag/v1.26.5
	DefaultEtcdVersion       = "v3.4.26" // https://github.com/etcd-io/etcd/releases/tag/v3.4.26
	DefaultCRICtlVersion     = "v1.26.0" // https://github.com/kubernetes-sigs/cri-tools/releases/tag/v1.26.0
	DefaultHelmVersion       = "v3.12.0" // https://github.com/helm/helm/releases
	DefaultRuncVersion       = "v1.1.7"  // https://github.com/opencontainers/runc/releases
	DefaultCniVersion        = "v1.3.0"  // https://github.com/containernetworking/plugins/releases
	DefaultContainerdVersion = "1.7.2"   // https://github.com/containerd/containerd/releases
)
