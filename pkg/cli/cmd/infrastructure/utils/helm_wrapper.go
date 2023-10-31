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

package utils

import (
	"fmt"
	"path/filepath"

	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util/helm"
)

type HelmInstallHelper struct {
	types.HelmChart
	kubeconfig string
}

func (h *HelmInstallHelper) Install(name, ns string) error {
	if err := h.addRepo(); err != nil {
		return err
	}
	helmConfig := helm.NewConfig(ns, h.kubeconfig, "", klog.V(1).Enabled())
	installOpts := h.buildChart(name, ns)
	output, err := installOpts.Install(helmConfig)
	if err != nil && helm.ReleaseNotFound(err) {
		fmt.Println(output)
		return nil
	}
	return err
}

func NewHelmInstaller(chart types.HelmChart, kubeconfig string) Installer {
	installer := HelmInstallHelper{
		HelmChart:  chart,
		kubeconfig: kubeconfig}
	return &installer
}

func (h *HelmInstallHelper) buildChart(name, ns string) *helm.InstallOpts {
	return &helm.InstallOpts{
		Name:            name,
		Chart:           h.getChart(name),
		Wait:            true,
		Version:         h.Version,
		Namespace:       ns,
		ValueOpts:       &h.ValueOptions,
		TryTimes:        3,
		CreateNamespace: true,
		Atomic:          true,
	}
}

func (h *HelmInstallHelper) getChart(name string) string {
	if h.Name == "" {
		return ""
	}
	// install helm package form local path
	if h.Repo != "" {
		return filepath.Join(h.Name, name)
	}
	if h.Path != "" {
		return filepath.Join(h.Path, name)
	}
	return ""
}

func (h *HelmInstallHelper) addRepo() error {
	if h.Repo == "" {
		return nil
	}
	return helm.AddRepo(&repo.Entry{Name: h.Name, URL: h.Repo})
}
