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
	"strings"
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
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.upgrade(cmd))
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.postInstall())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana")
	cmd.Flags().StringVar(&o.Version, "version", "", "Set KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.check, "check", true, "Check kubernetes environment before upgrade")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for upgrading KubeBlocks")

	return cmd
}

func (o *InstallOptions) upgrade(cmd *cobra.Command) error {
	// check whether monitor flag is set by user
	monitorIsSet := false
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "monitor" {
			monitorIsSet = true
		}
	})

	// check flags already been set
	if !monitorIsSet && len(o.Version) == 0 && len(o.Sets) == 0 {
		fmt.Fprint(o.Out, "Nothing to upgrade, --set, --version or --monitor should be specified")
		return nil
	}
	if monitorIsSet {
		o.Sets = append(o.Sets, fmt.Sprintf(kMonitorParam, o.Monitor))
	}

	// check if KubeBlocks has been installed
	versionInfo, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	v := versionInfo[util.KubeBlocksApp]
	if len(v) > 0 {
		fmt.Fprintln(o.Out, "Current KubeBlocks version "+v)
	} else {
		return errors.New("KubeBlocks does not exist, try to run \"kbcli kubeblocks install\" to install")
	}

	msg := ""
	if len(o.Version) > 0 {
		if v == o.Version && len(o.Sets) == 0 {
			fmt.Fprintf(o.Out, "Current version %s is the same as the upgraded version, no need to upgrade\n", o.Version)
			return nil
		}
		msg = "to " + o.Version
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
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Version:   o.Version,
		Namespace: o.Namespace,
		Sets:      sets,
		Login:     true,
		TryTimes:  2,
		Timeout:   o.timeout,
	}
	return chart.Upgrade(o.HelmCfg)
}
