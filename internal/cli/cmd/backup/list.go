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

package backup

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listBackupExample = templates.Examples(`
		# list all backups
		kbcli backup list

		# list all backups of specified cluster
		kbcli backup list --cluster mycluster
	`)
)

func newListCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &cluster.ListBackupOptions{ListOptions: list.NewListOptions(f, streams, types.BackupGVR())}
	clusterName := ""
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List backups.",
		Aliases:           []string{"ls"},
		Example:           listBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			if clusterName != "" {
				o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, []string{clusterName})
			}
			o.Names = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(cluster.PrintBackupList(*o))
		},
	}
	o.AddFlags(cmd, true)
	cmd.Flags().StringVar(&clusterName, "cluster", "", "List backups in the specified cluster")
	util.RegisterClusterCompletionFunc(cmd, f)

	return cmd
}
