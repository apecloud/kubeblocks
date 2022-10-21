/*
Copyright 2022 The KubeBlocks Authors

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

package dbaas

import (
	"fmt"

	"k8s.io/client-go/dynamic"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

type options struct {
	genericclioptions.IOStreams

	cfg       *action.Configuration
	Namespace string
	client    dynamic.Interface
}

type installOptions struct {
	options
	Version string
	Sets    []string
}

// NewDbaasCmd creates the dbaas command
func NewDbaasCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dbaas",
		Short: "DBaaS(KubeBlocks) operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUninstallCmd(f, streams),
	)
	return cmd
}

func (o *options) complete(f cmdutil.Factory, cmd *cobra.Command) error {
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

func (o *installOptions) run() error {
	fmt.Fprintf(o.Out, "Installing KubeBlocks %s ...\n", o.Version)
	installer := Installer{
		cfg:       o.cfg,
		Namespace: o.Namespace,
		Version:   o.Version,
		Sets:      o.Sets,
	}

	if err := installer.Install(); err != nil {
		return errors.Wrap(err, "Failed to install KubeBlocks")
	}

	fmt.Fprintf(o.Out, "\nKubeBlocks %s Install SUCCESSFULLY!\n"+
		"You can now create a database cluster by running the following command:\n"+
		"\tdbctl cluster create <you cluster name>\n", o.Version)
	return nil
}

func (o *options) run() error {
	fmt.Fprintln(o.Out, "Uninstalling KubeBlocks ...")

	installer := Installer{
		cfg:       o.cfg,
		Namespace: o.Namespace,
		client:    o.client,
	}

	if err := installer.Uninstall(); err != nil {
		return errors.Wrap(err, "Failed to uninstall KubeBlocks")
	}

	fmt.Fprintln(o.Out, "Successfully uninstall KubeBlocks")
	return nil
}

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &installOptions{
		options: options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install KubeBlocks",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(f, cmd))
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &options{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall KubeBlocks",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(f, cmd))
			cmdutil.CheckErr(o.run())
		},
	}
	return cmd
}
