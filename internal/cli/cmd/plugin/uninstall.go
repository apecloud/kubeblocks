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

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
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
		klog.V(4).Infof("Going to uninstall plugin %s\n", name)
		if err := uninstall(paths, name); err != nil {
			return errors.Wrapf(err, "failed to uninstall plugin %s", name)
		}
	}
	return nil
}

func uninstall(p *Paths, name string) error {
	if _, err := ReadReceiptFromFile(p.PluginInstallReceiptPath(name)); err != nil {
		if os.IsNotExist(err) {
			return ErrIsNotInstalled
		}
		return errors.Wrapf(err, "failed to look up install receipt for plugin %q", name)
	}

	klog.V(1).Infof("Deleting plugin %s", name)

	symlinkPath := filepath.Join(p.BinPath(), pluginNameToBin(name, IsWindows()))
	klog.V(3).Infof("Unlink %q", symlinkPath)
	if err := removeLink(symlinkPath); err != nil {
		return errors.Wrap(err, "could not uninstall symlink of plugin")
	}

	pluginInstallPath := p.PluginInstallPath(name)
	klog.V(3).Infof("Deleting path %q", pluginInstallPath)
	if err := os.RemoveAll(pluginInstallPath); err != nil {
		return errors.Wrapf(err, "could not remove plugin directory %q", pluginInstallPath)
	}
	pluginReceiptPath := p.PluginInstallReceiptPath(name)
	klog.V(3).Infof("Deleting plugin receipt %q", pluginReceiptPath)
	err := os.Remove(pluginReceiptPath)
	return errors.Wrapf(err, "could not remove plugin receipt %q", pluginReceiptPath)
}
