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
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
	cmd.AddCommand(NewPluginIndexUpdateCmd(streams))
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

func (o *PluginIndexOptions) UpdateIndex() error {
	indexes, err := ListIndexes(paths)
	if err != nil {
		return errors.Wrap(err, "failed to list indexes")
	}

	for _, idx := range indexes {
		indexPath := paths.IndexPath(idx.Name)
		klog.V(1).Infof("Updating the local copy of plugin index (%s)", indexPath)
		if err := util.EnsureUpdated(idx.URL, indexPath); err != nil {
			klog.Warningf("failed to update index %q: %v", idx.Name, err)
			continue
		}

		fmt.Fprintf(o.Out, "Updated the local copy of plugin index %q\n", idx.Name)
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

func NewPluginIndexUpdateCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &PluginIndexOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "update all configured indexes",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.UpdateIndex())
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

// ListIndexes returns a slice of Index objects. The path argument is used as
// the base path of the index.
func ListIndexes(paths *Paths) ([]Index, error) {
	entries, err := os.ReadDir(paths.IndexBase())
	if err != nil {
		return nil, err
	}

	var indexes []Index
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		indexName := e.Name()
		remote, err := util.GitGetRemoteURL(paths.IndexPath(indexName))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list the remote URL for index %s", indexName)
		}

		indexes = append(indexes, Index{
			Name: indexName,
			URL:  remote,
		})
	}
	return indexes, nil
}

// AddIndex initializes a new index to install plugins from.
func AddIndex(paths *Paths, name, url string) error {
	if name == "" {
		return errors.New("index name must be specified")
	}
	dir := paths.IndexPath(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return util.EnsureCloned(url, dir)
	} else if err != nil {
		return err
	}
	return fmt.Errorf("index %q already exists", name)
}

// DeleteIndex removes specified index name. If index does not exist, returns an error that can be tested by os.IsNotExist.
func DeleteIndex(paths *Paths, name string) error {
	dir := paths.IndexPath(name)
	if _, err := os.Stat(dir); err != nil {
		return err
	}

	return os.RemoveAll(dir)
}
