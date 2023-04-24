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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var (
	pluginListIndexExample = templates.Examples(`
	# List all configured plugin indexes
	kbcli plugin index list
	`)

	pluginAddIndexExample = templates.Examples(`
	# Add a new plugin index
	kbcli plugin index add myIndex
	`)

	pluginDeleteIndexExample = templates.Examples(`
	# Delete a plugin index
	kbcli plugin index delete myIndex
	`)
)

func NewPluginIndexCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage custom plugin indexes",
		Long:  "Manage which repositories are used to discover plugins and install plugins from",
	}

	cmd.AddCommand(NewPluginIndexListCmd(streams))
	cmd.AddCommand(NewPluginIndexAddCmd(streams))
	cmd.AddCommand(NewPluginIndexDeleteCmd(streams))
	return cmd
}

type PluginIndexOptions struct {
	IndexName string
	URL       string

	genericclioptions.IOStreams
}

func (o *PluginIndexOptions) ListIndex() error {
	indexes, err := ListIndexes(paths)
	if err != nil {
		return errors.Wrap(err, "failed to list indexes")
	}

	p := NewPluginIndexPrinter(o.IOStreams.Out)
	for _, index := range indexes {
		addPluginIndexRow(index.Name, index.URL, p)
	}
	p.Print()

	return nil
}

func (o *PluginIndexOptions) AddIndex() error {
	err := AddIndex(paths, o.IndexName, o.URL)
	if err != nil {
		return err
	}
	return nil
}

func (o *PluginIndexOptions) DeleteIndex() error {
	err := DeleteIndex(paths, o.IndexName)
	if err != nil {
		return err
	}
	return nil
}

func NewPluginIndexListCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &PluginIndexOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List configured indexes",
		Example: pluginListIndexExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.ListIndex())
		},
	}

	return cmd
}

func NewPluginIndexAddCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &PluginIndexOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a new index",
		Example: pluginAddIndexExample,
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			o.IndexName = args[0]
			o.URL = args[1]
			cmdutil.CheckErr(o.AddIndex())
		},
	}

	return cmd
}

func NewPluginIndexDeleteCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &PluginIndexOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Remove a configured index",
		Example: pluginDeleteIndexExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			o.IndexName = args[0]
			cmdutil.CheckErr(o.DeleteIndex())
		},
	}

	return cmd
}

func NewPluginIndexPrinter(out io.Writer) *printer.TablePrinter {
	t := printer.NewTablePrinter(out)
	t.SetHeader("INDEX", "URL")
	return t
}

func addPluginIndexRow(index, url string, p *printer.TablePrinter) {
	p.AddRow(index, url)
}
