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

package plugin

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	pluginUninstallExample = templates.Examples(`
	# uninstall a kbcli or kubectl plugin by name
	kbcli plugin uninstall [PLUGIN]
	`)
)

func NewPluginUninstallCmd(_ genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall kbcli or kubectl plugins",
		Example: pluginUninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(uninstallPlugins(args))
		},
	}

	return cmd
}

func uninstallPlugins(names []string) error {
	for _, name := range names {
		if _, err := ReadReceiptFromFile(paths.PluginInstallReceiptPath(name)); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrapf(err, "failed to look up install receipt for plugin %q", name)
		}
		if IsWindows() {
			name += ".exe"
		}
		if err := removeBinFile(name); err != nil {
			return err
		}
		if err := os.Remove(paths.PluginInstallReceiptPath(name)); err != nil {
			return err
		}
	}
	return nil
}

func removeBinFile(name string) error {
	kubectlPluginName := "kubectl-" + name
	kbcliPluginName := "kbcli-" + name
	if _, err := os.Stat(filepath.Join(paths.BinPath(), kubectlPluginName)); !os.IsNotExist(err) {
		return os.Remove(filepath.Join(paths.BinPath(), kubectlPluginName))
	}
	if _, err := os.Stat(filepath.Join(paths.BinPath(), kbcliPluginName)); !os.IsNotExist(err) {
		return os.Remove(filepath.Join(paths.BinPath(), kbcliPluginName))
	}

	return nil
}
