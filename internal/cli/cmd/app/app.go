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

package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/cli/values"
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
    	# Install application named "nyancat"
    	kbcli app install nyancat
    
    	# Install "nyancat" in a specific namespace
    	kbcli app install nyancat --namespace cat
    
        # Install "nyancat" with creating a "LoadBalancer" type "Service" if you're using a cloud K8s service, such as EKS/GKE
    	kbcli app install nyancat --set service.type=LoadBalancer
    `)

	uninstallExample = templates.Examples(`
		# Uninstall application named "nyancat"
        kbcli app uninstall nyancat
	`)
)

type options struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams
	Version         string
	Sets            []string
	HelmCfg         *helm.Config
	Namespace       string
	AppName         string
	CreateNamespace bool
}

func NewAppCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app [install | uninstall] APP_NAME",
		Short: "Manage external applications related to KubeBlocks.",
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
		Short:   "Install the application with the specified name.",
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.install())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", "", "Application version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values to the application on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "Create the namespace if not present")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &options{
		Factory:   f,
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall the application with the specified name.",
		Example: uninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.uninstall())
		},
	}

	return cmd
}

func (o *options) install() error {
	spinner := util.Spinner(o.Out, "Installing application %s", o.AppName)
	defer spinner(false)

	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	notes, err := o.installChart()
	if err != nil {
		return err
	}

	spinner(true)

	fmt.Fprintf(o.Out, "Install %s SUCCESSFULLY!\n", o.AppName)
	fmt.Fprintln(o.Out, notes)

	return nil
}

func (o *options) installChart() (string, error) {
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}

	chart := helm.InstallOpts{
		Name:            o.AppName,
		Chart:           types.KubeBlocksChartName + "/" + o.AppName,
		Wait:            true,
		Version:         o.Version,
		Namespace:       o.Namespace,
		ValueOpts:       &values.Options{Values: sets},
		TryTimes:        2,
		CreateNamespace: o.CreateNamespace,
	}
	return chart.Install(o.HelmCfg)
}

func (o *options) uninstall() error {
	chart := helm.InstallOpts{
		Name:      o.AppName,
		Namespace: o.Namespace,
	}
	if err := chart.Uninstall(o.HelmCfg); err != nil {
		return err
	}

	if err := helm.RemoveRepo(&repo.Entry{Name: o.AppName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Uninstall %s SUCCESSFULLY!\n", o.AppName)
	return nil
}

// Complete receive exec parameters
func (o *options) complete(cmd *cobra.Command, args []string) error {
	var err error

	if o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	if len(args) == 0 {
		return errors.New("missing application name")
	}
	o.AppName = args[0]

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

	o.HelmCfg = helm.NewConfig(o.Namespace, kubeconfig, kubecontext, false)
	return nil
}
