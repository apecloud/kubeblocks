/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package kubeblocks

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// NewKubeBlocksCmd creates the kubeblocks command
func NewKubeBlocksCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kubeblocks [install | upgrade | list-versions | uninstall]",
		Short:   "KubeBlocks operation commands.",
		Aliases: []string{"kb"},
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUpgradeCmd(f, streams),
		newUninstallCmd(f, streams),
		newListVersionsCmd(streams),
		newStatusCmd(f, streams),
		NewConfigCmd(f, streams),
		NewDescribeConfigCmd(f, streams),
	)
	// add preflight cmd
	cmd.AddCommand(NewPreflightCmd(f, streams))
	return cmd
}
