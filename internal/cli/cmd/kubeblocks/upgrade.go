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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var (
	upgradeExample = templates.Examples(`
	# Upgrade KubeBlocks to specified version
	kbcli kubeblocks upgrade --version=0.4.0

	# Upgrade KubeBlocks other settings, for example, set replicaCount to 3
	kbcli kubeblocks upgrade --set replicaCount=3`)
)

func newUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrade KubeBlocks",
		Args:    cobra.NoArgs,
		Example: upgradeExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade(cmd))
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.PostInstall())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana")
	cmd.Flags().StringVar(&o.Version, "version", "", "Set KubeBlocks version")
	cmd.Flags().BoolVar(&o.check, "check", true, "Check kubernetes environment before upgrade")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for upgrading KubeBlocks")
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)

	return cmd
}

func (o *InstallOptions) Upgrade(cmd *cobra.Command) error {
	// check whether monitor flag is set by user
	monitorIsSet := false
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "monitor" {
			monitorIsSet = true
		}
	})

	// check flags already been set
	if !monitorIsSet && len(o.Version) == 0 && helm.ValueOptsIsEmpty(&o.ValueOpts) {
		fmt.Fprint(o.Out, "Nothing to upgrade, --set, --version or --monitor should be specified.\n")
		return nil
	}
	if monitorIsSet {
		o.ValueOpts.Values = append(o.ValueOpts.Values, fmt.Sprintf(kMonitorParam, o.Monitor))
	}

	// check if KubeBlocks has been installed
	versionInfo, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	msg := ""
	v := versionInfo[util.KubeBlocksApp]
	if len(v) > 0 {
		if len(o.Version) > 0 {
			if v == o.Version && helm.ValueOptsIsEmpty(&o.ValueOpts) {
				fmt.Fprintf(o.Out, "Current version %s is the same as the upgraded version, no need to upgrade.\n", o.Version)
				return nil
			}
			msg = "to " + o.Version
		}
		fmt.Fprintf(o.Out, "Current KubeBlocks version %s.", v)
	} else {
		return errors.New("KubeBlocks does not exist, try to run \"kbcli kubeblocks install\" to install")
	}

	// it's time to upgrade
	spinner := util.Spinner(o.Out, "%-40s", "Upgrading KubeBlocks "+msg)
	defer spinner(false)

	if err = o.preCheck(versionInfo); err != nil {
		return err
	}

	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}

	// upgrade KubeBlocks chart
	if err = o.upgradeChart(); err != nil {
		return err
	}

	// successfully upgraded
	spinner(true)

	return nil
}

func (o *InstallOptions) upgradeChart() error {
	return o.buildChart().Upgrade(o.HelmCfg)
}
