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
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	deleteBackupExample = templates.Examples(`
		# delete a backup
		kbcli backup delete mybackup
	`)
)

func newDeleteCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.BackupGVR())
	clusterName := ""
	cmd := &cobra.Command{
		Use:               "delete",
		Short:             "Delete a backup.",
		Example:           deleteBackupExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(completeForDeleteBackup(o, clusterName))
			util.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)
	cmd.Flags().StringVar(&clusterName, "cluster", "", "The cluster name.")
	util.RegisterClusterCompletionFunc(cmd, f)

	return cmd
}

func completeForDeleteBackup(o *delete.DeleteOptions, cluster string) error {
	if o.Force && len(o.Names) == 0 {
		if cluster == "" {
			return fmt.Errorf("must give a backup name or cluster name")
		}
		o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, []string{cluster})
	}
	return nil
}
