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
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/utils"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type AddonsInstaller struct {
	common.KubeModule

	Addons     []types.PluginMeta
	Kubeconfig string
}

type KBAddonsInstall struct {
	common.KubeAction

	Addons     []types.PluginMeta
	Kubeconfig string
}

func (a *AddonsInstaller) Init() {
	a.Name = "AddonsInstaller"
	a.Desc = "Install helm addons"
	a.Tasks = []task.Interface{
		&task.LocalTask{
			Name:   "AddonsInstaller",
			Desc:   "Install helm addons",
			Action: &KBAddonsInstall{Addons: a.Addons, Kubeconfig: a.Kubeconfig},
		}}
}

func (i *KBAddonsInstall) Execute(runtime connector.Runtime) error {
	var installer utils.Installer
	for _, addon := range i.Addons {
		switch {
		case addon.Sources.Chart != nil:
			installer = utils.NewHelmInstaller(*addon.Sources.Chart, i.Kubeconfig)
		case addon.Sources.Yaml != nil:
			installer = utils.NewYamlInstaller(*addon.Sources.Yaml, i.Kubeconfig)
		default:
			return cfgcore.MakeError("addon source not supported: addon: %v", addon)
		}
		if err := installer.Install(addon.Name, addon.Namespace); err != nil {
			return err
		}
	}
	return nil
}
