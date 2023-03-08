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

package backupconfig

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/kubeblocks"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var backupConfigExample = templates.Examples(`
		# Enable the snapshot-controller and volume snapshot, to support snapshot backup.
		kbcli backup-config --set snapshot-controller.enabled=true

		# If you have already installed a snapshot-controller, only enable the snapshot backup feature
        kbcli backup-config --set dataProtection.enableVolumeSnapshot=true

		# Schedule automatic backup at 18:00 every day (UTC timezone)
		kbcli backup-config --set dataProtection.backupSchedule="0 18 * * *"

		# Set automatic backup retention for 7 days
		kbcli backup-config --set dataProtection.backupTTL="168h0m0s"
	`)

// NewBackupConfigCmd creates the backup-config command
func NewBackupConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &kubeblocks.InstallOptions{
		Options: kubeblocks.Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "backup-config",
		Short:   "KubeBlocks backup config",
		Example: backupConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade(cmd))
		},
	}
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)
	return cmd
}
