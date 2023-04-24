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

package cluster

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	EnvExperimentalExpose = "KBCLI_EXPERIMENTAL_EXPOSE"
)

func init() {
	_ = viper.BindEnv(EnvExperimentalExpose)
}

// NewClusterCmd creates the cluster command
func NewClusterCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster command.",
	}

	groups := templates.CommandGroups{
		{
			Message: "Basic Cluster Commands:",
			Commands: []*cobra.Command{
				NewCreateCmd(f, streams),
				NewConnectCmd(f, streams),
				NewDescribeCmd(f, streams),
				NewListCmd(f, streams),
				NewListInstancesCmd(f, streams),
				NewListComponentsCmd(f, streams),
				NewListEventsCmd(f, streams),
				NewLabelCmd(f, streams),
				NewDeleteCmd(f, streams),
			},
		},
		{
			Message: "Cluster Operation Commands:",
			Commands: []*cobra.Command{
				NewUpdateCmd(f, streams),
				NewStopCmd(f, streams),
				NewStartCmd(f, streams),
				NewRestartCmd(f, streams),
				NewUpgradeCmd(f, streams),
				NewVolumeExpansionCmd(f, streams),
				NewVerticalScalingCmd(f, streams),
				NewHorizontalScalingCmd(f, streams),
				NewDescribeOpsCmd(f, streams),
				NewListOpsCmd(f, streams),
				NewDeleteOpsCmd(f, streams),
				NewExposeCmd(f, streams),
			},
		},
		{
			Message: "Cluster Configuration Operation Commands:",
			Commands: []*cobra.Command{
				NewReconfigureCmd(f, streams),
				NewEditConfigureCmd(f, streams),
				NewDescribeReconfigureCmd(f, streams),
				NewExplainReconfigureCmd(f, streams),
				NewDiffConfigureCmd(f, streams),
			},
		},
		{
			Message: "Backup/Restore Commands:",
			Commands: []*cobra.Command{
				NewListBackupPolicyCmd(f, streams),
				NewEditBackupPolicyCmd(f, streams),
				NewBackupCmd(f, streams),
				NewListBackupCmd(f, streams),
				NewDeleteBackupCmd(f, streams),
				NewRestoreCmd(f, streams),
			},
		},
		{
			Message: "Troubleshooting Commands:",
			Commands: []*cobra.Command{
				NewLogsCmd(f, streams),
				NewListLogsCmd(f, streams),
			},
		},

		{
			Message: "User Accounts Commands:",
			Commands: []*cobra.Command{
				NewCreateAccountCmd(f, streams),
				NewDeleteAccountCmd(f, streams),
				NewDescAccountCmd(f, streams),
				NewListAccountsCmd(f, streams),
				NewGrantOptions(f, streams),
				NewRevokeOptions(f, streams),
			},
		},
	}

	// add subcommands
	groups.Add(cmd)
	templates.ActsAsRootCommand(cmd, nil, groups...)

	return cmd
}
