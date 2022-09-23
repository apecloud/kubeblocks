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

package playground

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

var (
	installer = &Installer{
		Ctx:         context.Background(),
		ClusterName: ClusterName,
		Namespace:   ClusterNamespace,
		DBCluster:   DBClusterName,
	}

	rootOptions = &RootOptions{}
)

type RootOptions struct {
	CloudProvider string
	AccessKey     string
	AccessSecret  string
	Region        string
}

type InitOptions struct {
	genericclioptions.IOStreams
	Engine     string
	Version    string
	DBReplicas string
	DryRun     bool
}

type DestroyOptions struct {
	genericclioptions.IOStreams
	Engine   string
	Provider string
	Version  string
	DryRun   string
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
		newGuideCmd(),
		newPortForward(),
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
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&rootOptions.CloudProvider, "cloud-provider", DefaultCloudProvider, "Cloud provider type")
	cmd.Flags().StringVar(&rootOptions.AccessKey, "access-key", "", "Cloud provider access key")
	cmd.Flags().StringVar(&rootOptions.AccessSecret, "access-secret", "", "Cloud provider access secret")
	cmd.Flags().StringVar(&rootOptions.Region, "region", "", "Cloud provider region")
	cmd.Flags().StringVar(&o.Version, "version", DefaultVersion, "Database engine version")
	cmd.Flags().StringVar(&o.DBReplicas, "replicas", DefaultDBReplicas, "Database cluster replicas")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Dry run the playground init")
	return cmd
}

func newDestroyCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &DestroyOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy the playground cluster.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.destroyPlayground(); err != nil {
				util.Errf("%v", err)
			}
		},
	}
	return cmd
}

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			cp := cloudprovider.Get()
			instance, err := cp.Instance()
			if err != nil {
				util.Errf("%v", err)
				return
			}
			if err := installer.PrintGuide(cp.Name(), instance.GetIP()); err != nil {
				util.Errf("%v", err)
			}
		},
	}
	return cmd
}

func newPortForward() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port-forward",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := util.PortForward(fmt.Sprintf("service/%s", DBClusterName), "3306"); err != nil {
				util.Errf("%v", err)
			}
		},
	}
	return cmd
}

func (o *InitOptions) Complete() error {
	installer.wesql = Wesql{
		serverVersion: o.Version,
		dbReplicas:    o.DBReplicas,
	}
	return nil
}

func (o *InitOptions) Validate() error {
	return nil
}

func (o *InitOptions) Run() error {
	util.Info("Initializing playground cluster...")

	var err error

	defer func() {
		err := util.CleanUpPlayground()
		if err != nil {
			util.Errf("Fail to clean up: %v", err)
		}
	}()

	// remote playground
	if rootOptions.CloudProvider != cloudprovider.Local {
		// apply changes
		cp, err := cloudprovider.InitProvider(rootOptions.CloudProvider, rootOptions.AccessKey, rootOptions.AccessSecret, rootOptions.Region)
		if err != nil {
			return errors.Wrap(err, "Failed to create cloud provider")
		}
		if err := cp.Apply(false); err != nil {
			return errors.Wrap(err, "Failed to apply change")
		}
		instance, err := cp.Instance()
		if err != nil {
			return errors.Wrap(err, "Failed to query cloud instance")
		}
		kubeConfig := strings.ReplaceAll(kubeConfig, "${KUBERNETES_API_SERVER_ADDRESS}", instance.GetIP())
		kubeConfigPath := path.Join(util.GetKubeconfigDir(), "dbctl-playground")
		if err := ioutils.AtomicWriteFile(kubeConfigPath, []byte(kubeConfig), 0700); err != nil {
			return errors.Wrap(err, "Failed to update kube config")
		}
		if err := installer.PrintGuide(cp.Name(), instance.GetIP()); err != nil {
			return errors.Wrap(err, "Failed to print user guide")
		}
		return nil
	}

	// local playGround
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
	err = installer.PrintGuide(DefaultCloudProvider, LocalHost)
	if err != nil {
		return errors.Wrap(err, "Failed to print user guide")
	}

	return nil
}

func (o *DestroyOptions) destroyPlayground() error {
	// remote playground, just destroy all cloud resources
	cp := cloudprovider.Get()
	if cp.Name() != cloudprovider.Local {
		// remove playground cluster kubeconfig
		if err := util.RemoveConfig(ClusterName); err != nil {
			return errors.Wrap(err, "Failed to remove playground kubeconfig file")
		}
		return cloudprovider.Get().Apply(true)
	}

	// local playground
	if err := installer.Uninstall(); err != nil {
		return err
	}
	util.Info("Successfully destroyed playground cluster.")
	return nil
}
