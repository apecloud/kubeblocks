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

package kubeblocks

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var backupConfigExample = templates.Examples(`
		# Enable the snapshot-controller and volume snapshot, to support snapshot backup.
		kbcli kubeblocks config --set snapshot-controller.enabled=true
        
        Options Parameters:
		# If you have already installed a snapshot-controller, only enable the snapshot backup feature
        dataProtection.enableVolumeSnapshot=true

		# the global pvc name which persistent volume claim to store the backup data.
	    # will replace the pvc name when it is empty in the backup policy.
        dataProtection.backupPVCName=backup-data
		
        # the init capacity of pvc for creating the pvc, e.g. 10Gi.
        # will replace the init capacity when it is empty in the backup policy.
        dataProtection.backupPVCInitCapacity=100Gi

        # the pvc storage class name.
        # will replace the storageClassName when it is nil in the backup policy.
        dataProtection.backupPVCStorageClassName=csi-s3

 		# the pvc create policy.
	    # if the storageClass supports dynamic provisioning, recommend "IfNotPresent" policy.
        # otherwise, using "Never" policy. only affect the backupPolicy automatically created by Kubeblocks.
		dataProtection.backupPVCCreatePolicy=Never
	`)

// NewConfigCmd creates the config command
func NewConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "config",
		Short:   "KubeBlocks config.",
		Example: backupConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade())
			// TODO: post handle after the config updates
		},
	}
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)
	return cmd
}
