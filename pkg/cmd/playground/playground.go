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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/infracreate/opencli/pkg/cluster"
	"github.com/infracreate/opencli/pkg/utils"
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
		Short: "Bootstrap a dbaas in local host",
		Long:  "Bootstrap a dbaas in local host",
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
		Short: "Destroy the playground in local host",
		Long:  "Destroy the playground in local host",
		Run: func(cmd *cobra.Command, args []string) {
			destroyPlayground(streams)
		},
	}
	return cmd
}

func newStatusCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display playground cluster status",
		Long:  "Display playground cluster status",
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
	fmt.Fprint(o.Out, "Start to init playground\n")

	var err error

	defer func() {
		err := utils.CleanUpPlayground()
		if err != nil {
			fmt.Fprintf(o.ErrOut, "Fail to clean up: %v\n", err)
		}
	}()

	//// Step.1 Set up K3s as dbaas control plane cluster
	//err = installer.Install()
	//if err != nil {
	//	return errors.Wrap(err, "Fail to set up k3d cluster")
	//}
	//
	//// Step.2 Deal with KUBECONFIG
	//err = installer.GenKubeconfig()
	//if err != nil {
	//	return errors.Wrap(err, "Fail to generate kubeconfig")
	//}
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
	fmt.Fprint(streams.Out, "Successfully destroy playground cluster.")
	return nil
}

func statusCmd(streams genericclioptions.IOStreams) {
	fmt.Fprintf(streams.Out, "Checking cluster status...")
	status := installer.GetStatus()
	stop := printClusterStatus(status)
	if stop {
		return
	}
	fmt.Fprintf(streams.Out, "Checking database cluster status...")
}

func printGuide() {

}
