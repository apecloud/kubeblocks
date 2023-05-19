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
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var (
	pluginSearchExample = templates.Examples(`
	# search a kbcli or kubectl plugin by name
	kbcli plugin search myplugin
	`)
)

func NewPluginSearchCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search kbcli or kubectl plugins",
		Example: pluginSearchExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(searchPlugin(streams, args[0]))
		},
	}

	return cmd
}

func searchPlugin(streams genericclioptions.IOStreams, name string) error {
	indexes, err := ListIndexes(paths)
	if err != nil {
		return errors.Wrap(err, "failed to list indexes")
	}

	var plugins []pluginEntry
	for _, index := range indexes {
		plugin, err := LoadPluginByName(paths.IndexPluginsPath(index.Name), name)
		if err != nil && !os.IsNotExist(err) {
			klog.V(1).Info("failed to load plugin %q from the index", name)
		} else {
			plugins = append(plugins, pluginEntry{
				index:  index.Name,
				plugin: plugin,
			})
		}
	}

	p := NewPluginSearchPrinter(streams.Out)
	for _, plugin := range plugins {
		_, err := os.Stat(paths.PluginInstallReceiptPath(name))
		addPluginSearchRow(plugin.index, plugin.plugin.Name, !os.IsNotExist(err), p)
	}
	p.Print()
	return nil
}

func NewPluginSearchPrinter(out io.Writer) *printer.TablePrinter {
	t := printer.NewTablePrinter(out)
	t.SetHeader("INDEX", "NAME", "INSTALLED")
	return t
}

func addPluginSearchRow(index, plugin string, installed bool, p *printer.TablePrinter) {
	if installed {
		p.AddRow(index, plugin, "yes")
	} else {
		p.AddRow(index, plugin, "no")
	}
}
