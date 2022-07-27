/*
Copyright © 2022 The OpenCli Authors

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

package playground

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cluster"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
)

var installer = cluster.LocalInstaller

type InitOptions struct {
	genericclioptions.IOStreams
	Engine   string
	Provider string
	Version  string
	DryRun   bool
}

// NewPlaygroundCmd creates the playground command
func NewPlaygroundCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playground [init | destroy]",
		Short: "Bootstrap a dbaas in local host",
		Long:  "Bootstrap a dbaas in local host",
		Run: func(cmd *cobra.Command, args []string) {
			//nolint
			cmd.Help()
		},
	}

	// add subcommands
	cmd.AddCommand(
		newInitCmd(streams),
		newDestroyCmd(streams),
		newStatusCmd(streams),
	)

	return cmd
}

func newInitCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &InitOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a DBaaS",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Engine, "engine", "mysql", "Database engine type")
	cmd.Flags().StringVar(&o.Provider, "provider", "cloudape.com", "Database provider")
	cmd.Flags().StringVar(&o.Version, "version", "8.0.29", "Database engine version")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Dry run the playground init")
	return cmd
}

func newDestroyCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the playground cluster.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := destroyPlayground(streams); err != nil {
				utils.Errf("%v", err)
			}
		},
	}
	return cmd
}

func newStatusCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display playground cluster status.",
		Run: func(cmd *cobra.Command, args []string) {
			statusCmd(streams)
		},
	}
	return cmd
}

func (o *InitOptions) Validate() error {
	return nil
}

func (o *InitOptions) Run() error {
	utils.Info("Initializing playground cluster...")

	var err error

	defer func() {
		err := utils.CleanUpPlayground()
		if err != nil {
			utils.Errf("Fail to clean up: %v", err)
		}
	}()

	// Step.1 Set up K3s as dbaas control plane cluster
	err = installer.Install()
	if err != nil {
		return errors.Wrap(err, "Fail to set up k3d cluster")
	}

	// Step.2 Deal with KUBECONFIG
	err = installer.GenKubeconfig()
	if err != nil {
		return errors.Wrap(err, "Fail to generate kubeconfig")
	}
	err = installer.SetKubeconfig()
	if err != nil {
		return errors.Wrap(err, "Fail to set kubeconfig")
	}

	// Step.3 Install dependencies
	err = installer.InstallDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to install dependencies")
	}

	// Step.4 print guide information
	printGuide()

	return nil
}

func destroyPlayground(streams genericclioptions.IOStreams) error {
	if err := installer.Uninstall(); err != nil {
		return err
	}
	utils.Info("Successfully destroyed playground cluster.")
	return nil
}

func statusCmd(streams genericclioptions.IOStreams) {
	utils.Info("Checking cluster status...")
	status := installer.GetStatus()
	stop := printClusterStatusK3d(status)
	if stop {
		return
	}
	utils.Info("Checking database cluster status...")
}

func printGuide() {

}
