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
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/create"
	"github.com/apecloud/kubeblocks/pkg/cli/delete"
	"github.com/apecloud/kubeblocks/pkg/cli/list"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

var (
	createBackupExample = templates.Examples(`
		# Create a backup for the cluster, use the default backup policy and volume snapshot backup method
		kbcli dp backup mybackup --cluster mycluster

		# create a backup with a specified method, run "kbcli cluster desc-backup-policy mycluster" to show supported backup methods
		kbcli dp backup mybackup --cluster mycluster --method mymethod

		# create a backup with specified backup policy, run "kbcli cluster list-backup-policy mycluster" to show the cluster supported backup policies
		kbcli dp backup mybackup --cluster mycluster --policy mypolicy

		# create a backup from a parent backup
		kbcli dp backup mybackup --cluster mycluster --parent-backup myparentbackup
	`)

	deleteBackupExample = templates.Examples(`
		# delete a backup
		kbcli dp delete-backup mybackup
	`)

	describeBackupExample = templates.Examples(`
		# describe a backup
		kbcli dp describe-backup mybackup
	`)

	listBackupExample = templates.Examples(`
		# list all backups
		kbcli dp list-backups

		# list all backups of specified cluster
		kbcli dp list-backups --cluster mycluster
	`)
)

func newBackupCommand(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("Backup %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli dp list-backups %s -n %s", opt.Name, opt.Namespace)
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
		Use:     "backup NAME",
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

	cmd.Flags().StringVar(&o.BackupMethod, "method", "", "Backup method that defined in backup policy (required), if only one backup method in backup policy, use it as default backup method, if multiple backup methods in backup policy, use method which volume snapshot is true as default backup method")
	cmd.Flags().StringVar(&clusterName, "cluster", "", "Cluster name")
	cmd.Flags().StringVar(&o.BackupPolicy, "policy", "", "Backup policy name, this flag will be ignored when backup-type is snapshot")
	cmd.Flags().StringVar(&o.DeletionPolicy, "deletion-policy", "Delete", "Deletion policy for backup, determine whether the backup content in backup repo will be deleted after the backup is deleted, supported values: [Delete, Retain]")
	cmd.Flags().StringVar(&o.RetentionPeriod, "retention-period", "", "Retention period for backup, supported values: [1y, 1m, 1d, 1h, 1m] or combine them [1y1m1d1h1m]")
	cmd.Flags().StringVar(&o.ParentBackupName, "parent-backup", "", "Parent backup name, used for incremental backup")
	util.RegisterClusterCompletionFunc(cmd, f)
	registerBackupFlagCompletionFunc(cmd, f)

	return cmd
}

func newBackupDeleteCommand(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.BackupGVR())
	clusterName := ""
	cmd := &cobra.Command{
		Use:               "delete-backup",
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

func newBackupDescribeCommand(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := cluster.DescribeBackupOptions{
		Factory:   f,
		IOStreams: streams,
		Gvr:       types.BackupGVR(),
	}
	cmd := &cobra.Command{
		Use:               "describe-backup NAME",
		Short:             "Describe a backup",
		Aliases:           []string{"desc-backup"},
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

func newListBackupCommand(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &cluster.ListBackupOptions{ListOptions: list.NewListOptions(f, streams, types.BackupGVR())}
	clusterName := ""
	cmd := &cobra.Command{
		Use:               "list-backups",
		Short:             "List backups.",
		Aliases:           []string{"ls-backups"},
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

func registerBackupFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"method",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var methods []string
			var labelSelector string
			clusterName, _ := cmd.Flags().GetString("cluster")
			if clusterName != "" {
				labelSelector = util.BuildLabelSelectorByNames(labelSelector, []string{clusterName})
			}
			dynamic, err := f.DynamicClient()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			backupPolicies, err := dynamic.Resource(types.BackupPolicyGVR()).List(context.TODO(), metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			for _, obj := range backupPolicies.Items {
				backupPolicy := &dpv1alpha1.BackupPolicy{}
				if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, backupPolicy); err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
				for _, method := range backupPolicy.Spec.BackupMethods {
					methods = append(methods, method.Name)
				}
			}
			return methods, cobra.ShellCompDirectiveDefault
		}))
}
