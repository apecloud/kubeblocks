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
	"context"
	"fmt"
	"github.com/apecloud/kubeblocks/pkg/cloudprovider"
	"github.com/apecloud/kubeblocks/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	installer = &Installer{
		Ctx:         context.Background(),
		ClusterName: ClusterName,
		Namespace:   ClusterNamespace,
		DBCluster:   DBClusterName,
	}
)

type InitOptions struct {
	genericclioptions.IOStreams
	Engine  string
	Version string
	DryRun  bool
}

type DestroyOptions struct {
	genericclioptions.IOStreams
	Engine   string
	Provider string
	Version  string
	DryRun   bool
}

// NewDbaasCmd creates the dbaas command
func NewDbaasCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dbaas",
		Short: "DBaaS operation commands",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("dbaas called")
		},
	}
	cmd.AddCommand(
		newInstallCmd(streams),
		newUninstallCmd(streams),
	)
	return cmd
}

func (o *InitOptions) Complete() error {
	return nil
}

func (o *InitOptions) Validate() error {
	return nil
}

func (o *InitOptions) Run() error {
	utils.Info("Initializing dbaas...")

	var err error

	defer func() {
		err := utils.CleanUpPlayground()
		if err != nil {
			utils.Errf("Fail to clean up: %v", err)
		}
	}()

	// Step.1 Install
	err = installer.InstallDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to install dependencies")
	}

	return nil
}

func (o *DestroyOptions) uninstallDBaaS() error {
	cp := cloudprovider.Get()
	if cp.Name() != cloudprovider.Local {
		// remove playground cluster kubeconfig
		if err := utils.RemoveConfig(ClusterName); err != nil {
			return errors.Wrap(err, "Failed to remove playground kubeconfig file")
		}
		return cloudprovider.Get().Apply(true)
	}

	// local playground
	if err := installer.Uninstall(); err != nil {
		return err
	}
	utils.Info("Successfully uninstall dbaas.")
	return nil
}

func newInstallCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &InitOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Bootstrap a DBaaS",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&installer.KubeConfig, "kube-config", "config", "KubeConfig path")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Dry run the playground init")
	return cmd
}

func newUninstallCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &DestroyOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall dbaas operator.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.uninstallDBaaS(); err != nil {
				utils.Errf("%v", err)
			}
		},
	}
	return cmd
}
