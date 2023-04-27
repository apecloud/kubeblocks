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
		if err := Uninstall(paths, name); err != nil {
			return errors.Wrapf(err, "failed to uninstall plugin %s", name)
		}
	}
	return nil
}
