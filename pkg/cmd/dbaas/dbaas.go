/*
Copyright © 2022 The dbctl Authors

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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/pkg/utils/helm"
)

const defaultVersion = "0.1.0-alpha.5"

type Options struct {
	genericclioptions.IOStreams

	cfg       *action.Configuration
	Namespace string
}

type InstallOptions struct {
	Options
	Version string
}

// NewDbaasCmd creates the dbaas command
func NewDbaasCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dbaas",
		Short: "DBaaS operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUninstallCmd(f, streams),
	)
	return cmd
}

func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
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

	return nil
}

func (o *InstallOptions) Run() error {
	fmt.Fprintln(o.Out, "Installing dbaas...")

	installer := Installer{
		cfg:       o.cfg,
		Namespace: o.Namespace,
		Version:   o.Version,
	}

	err := installer.Install()
	if err != nil {
		return errors.Wrap(err, "Failed to install dbaas")
	}

	fmt.Fprintln(o.Out, "Successfully install dbaas.")
	return nil
}

func (o *Options) Run() error {
	fmt.Fprintln(o.Out, "Uninstalling dbaas...")

	installer := Installer{
		cfg:       o.cfg,
		Namespace: o.Namespace,
	}

	if err := installer.Uninstall(); err != nil {
		return errors.Wrap(err, "Failed to uninstall dbaas")
	}

	fmt.Fprintln(o.Out, "Successfully uninstall dbaas.")
	return nil
}

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Bootstrap a DBaaS",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", defaultVersion, "DBaaS version")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &Options{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall dbaas operator.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}
	return cmd
}
