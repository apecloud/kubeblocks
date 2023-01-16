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

package nyancat

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var (
	installExample = templates.Examples(`
	# Install Nyan Cat demo application
	kbcli nyancat install

	# Install application in namespace "cat" if name "cat" already exists
	kbcli nyancat install --namespace cat

    # Install application with creating a "LoadBalancer" type "Service" if you're using a cloud K8s service, such as EKS/GKE
	kbcli nyancat install --set service.type=LoadBalancer
`)

	uninstallExample = templates.Examples(`
		# Uninstall Nyan Cat demo application
        kbcli nyancat uninstall
`)
)

type options struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams
	Version         string
	Sets            []string
	HelmCfg         *action.Configuration
	Namespace       string
	CreateNamespace bool
}

// NewNyancatCmd creates the nyancat command
func NewNyancatCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nyancat [install | uninstall]",
		Short: "Nyan Cat demo application operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUninstallCmd(f, streams),
	)
	return cmd
}

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &options{
		Factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install Nyan Cat demo appliaction.",
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.install())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", "", "Nyan Cat application version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "create the namespace if not present")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &options{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall Nyan Cat demo appliaction.",
		Example: uninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.uninstall())
		},
	}

	return cmd
}

func (o *options) install() error {
	fmt.Fprintf(o.Out, "Install Nyan Cat...\n")

	if err := helm.AddRepo(&repo.Entry{Name: types.NyanCatChartName, URL: types.KubeBlocksChartURL}); err != nil {
		return err
	}
	if err := o.installChart(); err != nil {
		return err
	}
	o.printNotes()

	return nil
}

func (o *options) installChart() error {
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}

	chart := helm.InstallOpts{
		Name:            types.NyanCatChartName,
		Chart:           types.KubeBlocksChartName + "/" + types.NyanCatChartName,
		Wait:            true,
		Version:         o.Version,
		Namespace:       o.Namespace,
		Sets:            sets,
		Login:           true,
		TryTimes:        2,
		CreateNamespace: o.CreateNamespace,
	}
	return chart.Install(o.HelmCfg)
}

func (o *options) printNotes() {
	fmt.Fprintf(o.Out, `
Nyan Cat Install SUCCESSFULLY!

-> Visit the demo application:
    kubectl port-forward service/nyancat 8087:8087 -n %s
    http://127.0.0.1:8087

-> Uninstall Nyan Cat demo application:
    kbcli nyancat uninstall
`, o.Namespace)
}

func (o *options) uninstall() error {
	chart := helm.InstallOpts{
		Name:      types.NyanCatChartName,
		Namespace: o.Namespace,
	}
	if err := chart.UnInstall(o.HelmCfg); err != nil {
		return err
	}

	if err := helm.RemoveRepo(&repo.Entry{Name: types.NyanCatChartName, URL: types.KubeBlocksChartURL}); err != nil {
		return err
	}
	fmt.Fprintln(o.Out, "Uninstall Nyan Cat SUCCESSFULLY!")
	return nil
}

// Complete receive exec parameters
func (o *options) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	if o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	// Add namespace to helm values
	o.Sets = append(o.Sets, fmt.Sprintf("namespace=%s", o.Namespace))

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	kubecontext, err := cmd.Flags().GetString("context")
	if err != nil {
		return err
	}

	if o.HelmCfg, err = helm.NewActionConfig(o.Namespace, kubeconfig, helm.WithContext(kubecontext)); err != nil {
		return err
	}

	return err
}
