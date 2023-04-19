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
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
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
		Short:   "Upgrade KubeBlocks.",
		Args:    cobra.NoArgs,
		Example: upgradeExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", "", "Set KubeBlocks version")
	cmd.Flags().BoolVar(&o.Check, "check", true, "Check kubernetes environment before upgrade")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for upgrading KubeBlocks")
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)

	return cmd
}

func (o *InstallOptions) Upgrade() error {
	if o.HelmCfg.Namespace() == "" {
		ns, err := util.GetKubeBlocksNamespace(o.Client)
		if err != nil || ns == "" {
			printer.Warning(o.Out, "Failed to find deployed KubeBlocks.\n\n")
			fmt.Fprint(o.Out, "Use \"kbcli kubeblocks install\" to install KubeBlocks.\n")
			fmt.Fprintf(o.Out, "Use \"kbcli kubeblocks status\" to get information in more details.\n")
			return nil
		}
		o.HelmCfg.SetNamespace(ns)
	}

	// check flags already been set
	if o.Version == "" && helm.ValueOptsIsEmpty(&o.ValueOpts) {
		fmt.Fprint(o.Out, "Nothing to upgrade, --set, --version should be specified.\n")
		return nil
	}

	// check if KubeBlocks has been installed
	v, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	kbVersion := v.KubeBlocks
	if kbVersion == "" {
		return errors.New("KubeBlocks does not exist, try to run \"kbcli kubeblocks install\" to install")
	}

	if kbVersion == o.Version && helm.ValueOptsIsEmpty(&o.ValueOpts) {
		fmt.Fprintf(o.Out, "Current version %s is the same as the upgraded version, no need to upgrade.\n", o.Version)
		return nil
	}
	fmt.Fprintf(o.Out, "Current KubeBlocks version %s.\n", v)

	if err = o.preCheck(v); err != nil {
		return err
	}

	// add helm repo
	spinner := printer.Spinner(o.Out, "%-40s", "Add and update repo "+types.KubeBlocksChartName)
	defer spinner(false)
	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	spinner(true)

	// it's time to upgrade
	msg := ""
	if o.Version != "" {
		msg = "to " + o.Version
	}
	spinner = printer.Spinner(o.Out, "%-40s", "Upgrading KubeBlocks "+msg)
	defer spinner(false)
	// upgrade KubeBlocks chart
	if err = o.upgradeChart(); err != nil {
		return err
	}
	// successfully upgraded
	spinner(true)

	// create VolumeSnapshotClass
	if err = o.createVolumeSnapshotClass(); err != nil {
		return err
	}

	if !o.Quiet {
		fmt.Fprintf(o.Out, "\nKubeBlocks has been upgraded %s SUCCESSFULLY!\n", msg)
		// set monitor to true, so that we can print notes with monitor
		o.Monitor = true
		o.printNotes()
	}
	return nil
}

func (o *InstallOptions) upgradeChart() error {
	return o.buildChart().Upgrade(o.HelmCfg)
}
