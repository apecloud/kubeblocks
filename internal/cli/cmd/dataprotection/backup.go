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

package dataprotection

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	createBackupExample = templates.Examples(`
		# Create a backup for the cluster
		kbcli backup create mybackup --cluster mycluster

		# create a snapshot backup
		kbcli backup create mybackup --cluster mycluster --type snapshot

		# create a backup with specified policy
		kbcli backup create mybackup --cluster mycluster --policy mypolicy
	`)

	deleteBackupExample = templates.Examples(`
		# delete a backup
		kbcli backup delete mybackup
	`)

	describeBackupExample = templates.Examples(`
		# describe a backup
		kbcli backup describe mybackup
	`)

	listBackupExample = templates.Examples(`
		# list all backups
		kbcli backup list

		# list all backups of specified cluster
		kbcli backup list --cluster mycluster
	`)
)

func NewBackupCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup command",
		Short: "Backup command.",
	}
	cmd.AddCommand(
		newListCommand(f, streams),
		newCreateCommand(f, streams),
		newDeleteCommand(f, streams),
		newDescribeCommand(f, streams),
	)
	return cmd
}

func newCreateCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("Backup %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli dp backup list %s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}

	clusterName := ""

	o := &cluster.CreateBackupOptions{
		CreateOptions: create.CreateOptions{
			IOStreams:       streams,
			Factory:         f,
			GVR:             types.BackupGVR(),
			CueTemplateName: "backup_template.cue",
			CustomOutPut:    customOutPut,
		},
	}
	o.CreateOptions.Options = o

	cmd := &cobra.Command{
		Use:     "create NAME",
		Short:   "Create a backup for the cluster.",
		Example: createBackupExample,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				o.BackupName = args[0]
			}
			if clusterName != "" {
				o.Args = []string{clusterName}
			}
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.CompleteBackup())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.BackupMethod, "method", "snapshot", "Backup type")
	cmd.Flags().StringVar(&clusterName, "cluster", "", "Cluster name")
	cmd.Flags().StringVar(&o.BackupPolicy, "policy", "", "Backup policy name, this flag will be ignored when backup-type is snapshot")
	util.RegisterClusterCompletionFunc(cmd, f)

	return cmd
}

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

func newDescribeCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := cluster.DescribeBackupOptions{
		Factory:   f,
		IOStreams: streams,
		Gvr:       types.BackupGVR(),
	}
	cmd := &cobra.Command{
		Use:               "describe",
		Short:             "Describe a backup",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupGVR()),
		Example:           describeBackupExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete(args))
			util.CheckErr(o.Run())
		},
	}
	return cmd
}

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
