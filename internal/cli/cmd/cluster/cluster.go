/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
				NewReconfigureCmd(f, streams),
				NewEditConfigureCmd(f, streams),
				NewExposeCmd(f, streams),
				NewDescribeReconfigureCmd(f, streams),
				NewExplainReconfigureCmd(f, streams),
				NewDiffConfigureCmd(f, streams),
			},
		},
		{
			Message: "Backup/Restore Commands:",
			Commands: []*cobra.Command{
				NewListBackupPolicyCmd(f, streams),
				NewLEditBackupPolicyCmd(f, streams),
				NewCreateBackupCmd(f, streams),
				NewListBackupCmd(f, streams),
				NewDeleteBackupCmd(f, streams),
				NewCreateRestoreCmd(f, streams),
				NewListRestoreCmd(f, streams),
				NewDeleteRestoreCmd(f, streams),
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
