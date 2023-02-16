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
	"fmt"

	"helm.sh/helm/v3/pkg/cli/values"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

type upgradeOptions struct {
	genericclioptions.IOStreams

	helmCfg   *action.Configuration
	client    dynamic.Interface
	namespace string
	valueOpts values.Options
}

// adjust for test
var helmAddRepo = helm.AddRepo

func (o *upgradeOptions) upgrade() error {
	entry := &repo.Entry{
		Name: types.KubeBlocksChartName,
		URL:  util.GetHelmChartRepoURL(),
	}
	if err := helmAddRepo(entry); err != nil {
		return err
	}

	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Login:     true,
		TryTimes:  2,
		Namespace: o.namespace,
		ValueOpts: &o.valueOpts,
	}

	return chart.Upgrade(o.helmCfg)
}

func (o *upgradeOptions) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	o.helmCfg, err = helm.NewActionConfig(o.namespace, kubeconfig)
	if err != nil {
		return err
	}

	o.client, err = f.DynamicClient()
	return err
}

func (o *upgradeOptions) run() error {
	// check flags already been set
	if helm.ValueOptsIsEmpty(&o.valueOpts) {
		fmt.Fprint(o.Out, "Nothing to config, --set should be specified.\n")
		return nil
	}

	spinner := util.Spinner(o.Out, "Config backup")
	defer spinner(false)
	if err := o.upgrade(); err != nil {
		return errors.Wrap(err, "failed to update backup config")
	}
	spinner(true)

	fmt.Fprintf(o.Out, "Backup config SUCCESSFULLY!\n")
	return nil
}

var backupConfigExample = templates.Examples(`
		# Enable the snapshot-controller and volume snapshot, to support snapshot backup.
		kbcli backup-config --set snapshot-controller.enabled=true --set dataProtection.enableVolumeSnapshot=true
	`)

// NewBackupConfigCmd creates the backup-config command
func NewBackupConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &upgradeOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "backup-config",
		Short:   "KubeBlocks backup config",
		Example: backupConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.run())
		},
	}

	helm.AddValueOptionsFlags(cmd.Flags(), &o.valueOpts)
	return cmd
}
