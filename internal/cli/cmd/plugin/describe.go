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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var pluginDescribeExample = templates.Examples(`
	# Describe a plugin
	kbcli plugin describe [PLUGIN]

	# Describe a plugin with index
	kbcli plugin describe [INDEX/PLUGIN]
	`)

func NewPluginDescribeCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe",
		Short:   "Describe a plugin",
		Example: pluginDescribeExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(printPluginInfo(streams.Out, args[0]))
		},
	}
	return cmd
}

func printPluginInfo(out io.Writer, name string) error {
	indexName, pluginName := CanonicalPluginName(name)
	plugin, err := LoadPluginByName(paths.IndexPluginsPath(indexName), pluginName)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "NAME: %s\n", plugin.Name)
	fmt.Fprintf(out, "INDEX: %s\n", indexName)
	if platform, ok, err := GetMatchingPlatform(plugin.Spec.Platforms); err == nil && ok {
		if platform.URI != "" {
			fmt.Fprintf(out, "URI: %s\n", platform.URI)
			fmt.Fprintf(out, "SHA256: %s\n", platform.Sha256)
		}
	}
	if plugin.Spec.Version != "" {
		fmt.Fprintf(out, "VERSION: %s\n", plugin.Spec.Version)
	}
	if plugin.Spec.Homepage != "" {
		fmt.Fprintf(out, "HOMEPAGE: %s\n", plugin.Spec.Homepage)
	}
	if plugin.Spec.Description != "" {
		fmt.Fprintf(out, "DESCRIPTION: \n%s\n", plugin.Spec.Description)
	}
	if plugin.Spec.Caveats != "" {
		fmt.Fprintf(out, "CAVEATS:\n%s\n", indent(plugin.Spec.Caveats))
	}
	return nil
}
