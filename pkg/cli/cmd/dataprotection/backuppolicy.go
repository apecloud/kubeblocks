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
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/list"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

var (
	listBackupPolicyExample = templates.Examples(`
		# list all backup policies
		kbcli cluster list-backup-policy

		# list backup policies with specified name
		kbcli cluster list-backup-policy mypolicy

		# list backup policies of the specified cluster
		kbcli cluster list-backup-policy --cluster mycluster
	`)

	describeBackupPolicyExample = templates.Examples(`
		# describe a backup policy
		kbcli cluster describe-backup-policy mypolicy

		# describe the default backup policy of the specified cluster
		kbcli cluster describe-backup-policy --cluster mycluster
	`)
)

func newListBackupPolicyCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.BackupPolicyGVR())
	clusterName := ""
	cmd := &cobra.Command{
		Use:               "list-backup-policy",
		Short:             "List backup policies",
		Aliases:           []string{"list-bp"},
		Example:           listBackupPolicyExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupPolicyGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			if clusterName != "" {
				o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, []string{clusterName})
			}
			o.Names = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete())
			util.CheckErr(cluster.PrintBackupPolicyList(*o))
		},
	}
	cmd.Flags().StringVar(&clusterName, "cluster", "", "The cluster name")
	o.AddFlags(cmd)

	return cmd
}

func newDescribeBackupPolicyCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := cluster.DescribeBackupPolicyOptions{
		IOStreams: streams,
		Factory:   f,
	}
	cmd := &cobra.Command{
		Use:               "describe-backup-policy",
		Short:             "Describe a backup policy",
		Aliases:           []string{"desc-backup-policy"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.BackupPolicyGVR()),
		Example:           describeBackupPolicyExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, o.ClusterNames)
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			util.CheckErr(o.Complete())
			util.CheckErr(o.Validate())
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringSliceVar(&o.ClusterNames, "cluster", []string{}, "The cluster name")
	return cmd
}
