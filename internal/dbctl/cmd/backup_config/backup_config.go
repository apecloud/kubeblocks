/*
Copyright ApeCloud Inc.

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

package backup_config

import (
	"fmt"
	"strings"

	"k8s.io/kubectl/pkg/util/templates"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

type configOptions struct {
	HelmCfg *action.Configuration

	Namespace string
	Version   string
	Sets      []string
	client    dynamic.Interface
}

type upgradeOptions struct {
	genericclioptions.IOStreams
	cfg    *action.Configuration
	client dynamic.Interface

	Namespace string
	Sets      []string
}

func (i *configOptions) upgrade() (string, error) {
	entry := &repo.Entry{
		Name: types.KubeBlocksChartName,
		URL:  types.KubeBlocksChartURL,
	}
	if err := helm.AddRepo(entry); err != nil {
		return "", err
	}

	var sets []string
	for _, set := range i.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Login:     true,
		TryTimes:  2,
		Namespace: i.Namespace,
		Version:   i.Version,
		Sets:      sets,
	}

	notes, err := chart.Upgrade(i.HelmCfg)
	if err != nil {
		return "", err
	}

	return notes, nil
}

func (o *upgradeOptions) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	o.cfg, err = helm.NewActionConfig(o.Namespace, kubeconfig)
	if err != nil {
		return err
	}

	o.client, err = f.DynamicClient()
	return err
}

func (o *upgradeOptions) run() error {
	fmt.Fprintf(o.Out, "Config backup...\n")

	config := configOptions{
		HelmCfg:   o.cfg,
		Namespace: o.Namespace,
		client:    o.client,
		Sets:      o.Sets,
	}

	if _, err := config.upgrade(); err != nil {
		return errors.Wrap(err, "Failed to update backup config")
	}

	fmt.Fprintf(o.Out, "Backup config SUCCESSFULLY!\n")
	return nil
}

var BackupConfigExample = templates.Examples(`
		# Enable the snapshot-controller and volumesnapshot, to support snapshot backup.
		dbctl backup-config --set snapshot-controller.enabled=true --set dataProtection.disableVolumeSnapshot=false
	`)

// NewBackupConfigCmd creates the backup-config command
func NewBackupConfigCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &upgradeOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "backup-config",
		Short:   "KubeBlocks backup config",
		Example: BackupConfigExample,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	return cmd
}
