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

type initOptions struct {
	genericclioptions.IOStreams
	Engine   string
	Version  string
	Replicas int8

	CloudProvider string
	AccessKey     string
	AccessSecret  string
	Region        string
}

type destroyOptions struct {
	genericclioptions.IOStreams
}

// NewPlaygroundCmd creates the playground command
func NewPlaygroundCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playground [init | destroy]",
		Short: "Bootstrap a dbaas in local host",
	}

	// add subcommands
	cmd.AddCommand(
		newInitCmd(streams),
		newDestroyCmd(streams),
		newGuideCmd(),
	)

	return cmd
}

func newInitCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &initOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a DBaaS",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.CloudProvider, "cloud-provider", DefaultCloudProvider, "Cloud provider type")
	cmd.Flags().StringVar(&o.AccessKey, "access-key", "", "Cloud provider access key")
	cmd.Flags().StringVar(&o.AccessSecret, "access-secret", "", "Cloud provider access secret")
	cmd.Flags().StringVar(&o.Region, "region", "", "Cloud provider region")
	cmd.Flags().StringVar(&o.Version, "version", DefaultVersion, "Database engine version")
	cmd.Flags().Int8Var(&o.Replicas, "replicas", DefaultReplicas, "Database cluster replicas")
	return cmd
}

func newDestroyCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &destroyOptions{
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
	installer := &installer{
		clusterName: ClusterName,
		namespace:   ClusterNamespace,
		dbCluster:   DBClusterName,
	}

	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			cp, _ := cloudprovider.Get()
			instance, err := cp.Instance()
			if err != nil {
				util.Errf("%v", err)
				return
			}
			if err := installer.printGuide(cp.Name(), instance.GetIP()); err != nil {
				util.Errf("%v", err)
			}
		},
	}
	return cmd
}

func (o *initOptions) validate() error {
	if o.Replicas <= 0 {
		return errors.New("replicas should greater than 0")
	}
	return nil
}

func (o *initOptions) run() error {
	util.Info("Initializing playground cluster...")

	installer := &installer{
		ctx:         context.Background(),
		clusterName: ClusterName,
		namespace:   ClusterNamespace,
		dbCluster:   DBClusterName,
		wesql: Wesql{
			serverVersion: o.Version,
			replicas:      o.Replicas,
		},
	}

	// remote playground
	if o.CloudProvider != cloudprovider.Local {
		// apply changes
		cp, err := cloudprovider.InitProvider(o.CloudProvider, o.AccessKey, o.AccessSecret, o.Region)
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
		if err := installer.printGuide(cp.Name(), instance.GetIP()); err != nil {
			return errors.Wrap(err, "Failed to print user guide")
		}
		return nil
	}

	// local playGround
	// Step.1 Set up K3s as dbaas control plane cluster
	err := installer.install()
	if err != nil {
		return errors.Wrap(err, "Fail to set up k3d cluster")
	}

	// Step.2 Deal with KUBECONFIG
	err = installer.genKubeconfig()
	if err != nil {
		return errors.Wrap(err, "Fail to generate kubeconfig")
	}
	err = installer.setKubeconfig()
	if err != nil {
		return errors.Wrap(err, "Fail to set kubeconfig")
	}

	// Step.3 Install dependencies
	err = installer.installDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to install dependencies")
	}

	// Step.4 print guide information
	err = installer.printGuide(DefaultCloudProvider, LocalHost)
	if err != nil {
		return errors.Wrap(err, "Failed to print user guide")
	}

	return nil
}

func (o *destroyOptions) destroyPlayground() error {
	installer := &installer{
		ctx:         context.Background(),
		clusterName: ClusterName,
	}

	// remote playground, just destroy all cloud resources
	cp, err := cloudprovider.Get()
	if err != nil {
		return err
	}

	if cp.Name() != cloudprovider.Local {
		// remove playground cluster kubeconfig
		if err := util.RemoveConfig(ClusterName); err != nil {
			return errors.Wrap(err, "Failed to remove playground kubeconfig file")
		}
		cp, err = cloudprovider.Get()
		if err != nil {
			return err
		}
		return cp.Apply(true)
	}

	// local playground
	if err := installer.uninstall(); err != nil {
		return err
	}
	util.Info("Successfully destroyed playground cluster.")
	return nil
}
