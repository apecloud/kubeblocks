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
	"os"
	"os/exec"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/files"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	kbutils "github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/utils"
)

const (
	CurlDownloadURLFormat = "curl -L -o %s %s"
	WgetDownloadURLFormat = "wget -O %s %s"

	defaultDownloadURL = CurlDownloadURLFormat
)

func downloadKubernetesBinaryWithArch(downloadPath string, arch string, binaryVersion types.InfraVersionInfo) (map[string]*files.KubeBinary, error) {
	downloadCommand := func(path, url string) string {
		return fmt.Sprintf(defaultDownloadURL, path, url)
	}

	binaries := []*files.KubeBinary{
		files.NewKubeBinary("etcd", arch, binaryVersion.EtcdVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("kubeadm", arch, binaryVersion.KubernetesVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("kubelet", arch, binaryVersion.KubernetesVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("kubectl", arch, binaryVersion.KubernetesVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("kubecni", arch, binaryVersion.CniVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("helm", arch, binaryVersion.HelmVersion, downloadPath, downloadCommand),
		// for containerd
		files.NewKubeBinary("crictl", arch, binaryVersion.CRICtlVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("containerd", arch, binaryVersion.ContainerVersion, downloadPath, downloadCommand),
		files.NewKubeBinary("runc", arch, binaryVersion.RuncVersion, downloadPath, downloadCommand),
	}

	binariesMap := make(map[string]*files.KubeBinary)
	for _, binary := range binaries {
		if err := binary.CreateBaseDir(); err != nil {
			return nil, cfgcore.WrapError(err, "failed to create file %s base dir.", binary.FileName)
		}
		logger.Log.Messagef(common.LocalHost, "downloading %s %s %s ...", arch, binary.ID, binary.Version)
		binariesMap[binary.ID] = binary
		if checkDownloadBinary(binary) {
			continue
		}
		if err := download(binary); err != nil {
			return nil, cfgcore.WrapError(err, "failed to download %s binary: %s", binary.ID, binary.GetCmd())
		}
	}
	return binariesMap, nil
}

func checkDownloadBinary(binary *files.KubeBinary) bool {
	if !util.IsExist(binary.Path()) {
		return false
	}
	err := kbutils.CheckSha256sum(binary)
	if err != nil {
		logger.Log.Messagef(common.LocalHost, "failed to check %s sha256, error: %v", binary.ID, err)
		_ = os.Remove(binary.Path())
		return false
	}
	logger.Log.Messagef(common.LocalHost, "%s is existed", binary.ID)
	return true
}

func download(binary *files.KubeBinary) error {
	if err := kbutils.RunCommand(exec.Command("/bin/sh", "-c", binary.GetCmd())); err != nil {
		return err
	}
	return kbutils.WriteSha256sum(binary)
}
