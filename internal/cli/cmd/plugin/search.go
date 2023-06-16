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
	"strings"

	"github.com/pkg/errors"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var (
	pluginSearchExample = templates.Examples(`
	# search a kbcli or kubectl plugin with keywords
	kbcli plugin search keyword1 keyword2
	`)
)

type pluginSearchOptions struct {
	keyword string
	limit   int

	genericclioptions.IOStreams
}

func NewPluginSearchCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &pluginSearchOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search kbcli or kubectl plugins",
		Long:    "Search kbcli or kubectl plugins by keywords",
		Example: pluginSearchExample,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().IntVar(&o.limit, "limit", 50, "Limit the number of plugin descriptions to output")
	return cmd
}

func (o *pluginSearchOptions) complete(args []string) error {
	o.keyword = strings.Join(args, "")

	return nil
}

func (o *pluginSearchOptions) run() error {
	indexes, err := ListIndexes(paths)
	if err != nil {
		return errors.Wrap(err, "failed to list indexes")
	}

	var plugins []pluginEntry
	for _, index := range indexes {
		ps, err := LoadPluginListFromFS(paths.IndexPluginsPath(index.Name))
		if err != nil {
			return errors.Wrapf(err, "failed to load plugin list from the index %s", index.Name)
		}
		for _, p := range ps {
			plugins = append(plugins, pluginEntry{
				index:  index.Name,
				plugin: p,
			})
		}
	}

	searchPrinter := NewPluginSearchPrinter(o.Out)
	for _, p := range plugins {
		// fuzzy search
		if fuzzySearchByNameAndDesc(o.keyword, p.plugin.Name, p.plugin.Spec.ShortDescription) {
			_, err := os.Stat(paths.PluginInstallReceiptPath(p.plugin.Name))
			addPluginSearchRow(p.index, p.plugin.Name, limitString(p.plugin.Spec.ShortDescription, o.limit), !os.IsNotExist(err), searchPrinter)
		}
	}
	searchPrinter.Print()
	return nil
}

func NewPluginSearchPrinter(out io.Writer) *printer.TablePrinter {
	t := printer.NewTablePrinter(out)
	t.SetHeader("INDEX", "NAME", "DESCRIPTION", "INSTALLED")
	return t
}

func addPluginSearchRow(index, plugin, description string, installed bool, p *printer.TablePrinter) {
	if installed {
		p.AddRow(index, plugin, description, "yes")
	} else {
		p.AddRow(index, plugin, description, "no")
	}
}

func fuzzySearchByNameAndDesc(keyword, name, description string) bool {
	if keyword == "" {
		return false
	}

	// find by name and description
	matches := fuzzy.Find(keyword, []string{name})
	if len(matches) > 0 {
		return true
	}

	matches = fuzzy.Find(keyword, []string{description})
	if len(matches) > 0 && matches[0].Score > 0 {
		return true
	}

	return false

}

func limitString(s string, length int) string {
	if len(s) > length && length > 3 {
		s = s[:length] + "..."
	}
	return s
}
