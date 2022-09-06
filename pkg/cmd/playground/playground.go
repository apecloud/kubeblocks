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

	"github.com/apecloud/kubeblocks/pkg/cloudprovider"
	"github.com/apecloud/kubeblocks/pkg/cluster"
	"github.com/apecloud/kubeblocks/pkg/provider"
	"github.com/apecloud/kubeblocks/pkg/utils"
)

var installer = &cluster.PlaygroundInstaller{
	Ctx:         context.Background(),
	ClusterName: ClusterName,
	// control plane will install in this namespace, database cluster will
	// install in the default namespace
	Namespace: ClusterNamespace,
	DBCluster: DBClusterName,
}

var rootOptions = &RootOptions{}

type RootOptions struct {
	CloudProvider string
	AccessKey     string
	AccessSecret  string
	Region        string
}

type InitOptions struct {
	genericclioptions.IOStreams
	Engine     string
	Provider   string
	Version    string
	DryRun     bool
	Backup     bool
	S3Endpoint string
	S3Region   string
	S3Bucket   string
}

type DestroyOptions struct {
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
		newStatusCmd(),
		newGuideCmd(),
		newPortForward(),
		newInstallCmd(streams),
		newUnInstallCmd(streams),
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
	cmd.Flags().StringVar(&o.Engine, "engine", DefaultEngine, "Database engine type")
	cmd.Flags().StringVar(&o.Provider, "provider", defaultProvider, "Database provider")
	cmd.Flags().StringVar(&o.Version, "version", DefaultVersion, "Database engine version")
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
				utils.Errf("%v", err)
			}
		},
	}
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display playground cluster status.",
		Run: func(cmd *cobra.Command, args []string) {
			statusCmd()
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
				utils.Errf("%v", err)
				return
			}
			if err := installer.PrintGuide(cp.Name(), instance.GetIP()); err != nil {
				utils.Errf("%v", err)
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
			if err := utils.PortForward(fmt.Sprintf("service/%s", DBClusterName), "3306"); err != nil {
				utils.Errf("%v", err)
			}
		},
	}
	return cmd
}

func (o *InitOptions) Complete() error {
	if o.Backup {
		installer.Provider = &provider.DataProtection{
			ServerVersion: o.Version,
			AccessKey:     rootOptions.AccessKey,
			SecretKey:     rootOptions.AccessSecret,
			S3Endpoint:    o.S3Endpoint,
			S3Bucket:      o.S3Bucket,
			Region:        o.S3Region,
		}
	} else {
		installer.Provider = provider.NewProvider(o.Engine, o.Version)
	}
	return nil
}

func (o *InitOptions) Validate() error {
	if o.Backup {
		if rootOptions.AccessKey == "" {
			return errors.New("--access-key is required")
		}
		if rootOptions.AccessSecret == "" {
			return errors.New("--access-secret is required")
		}
		if o.S3Region == "" {
			return errors.New("--s3-region is required")
		}
		if o.S3Endpoint == "" {
			return errors.New("--s3-endpoint is required")
		}
		if o.S3Bucket == "" {
			return errors.New("--s3-bucket is required")
		}
	}
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
		kubeConfig := strings.ReplaceAll(utils.KubeConfig, "${KUBERNETES_API_SERVER_ADDRESS}", instance.GetIP())
		kubeConfigPath := path.Join(utils.GetKubeconfigDir(), "dbctl-playground")
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
		if err := utils.RemoveConfig(ClusterName); err != nil {
			return errors.Wrap(err, "Failed to remove playground kubeconfig file")
		}
		return cloudprovider.Get().Apply(true)
	}

	// local playground
	if err := installer.Uninstall(); err != nil {
		return err
	}
	utils.Info("Successfully destroyed playground cluster.")
	return nil
}

func statusCmd() {
	utils.Info("Checking cluster status...")
	status := installer.GetStatus()
	stop := utils.PrintClusterStatus(status)
	if stop {
		return
	}
	// TODO
	utils.Info("Checking database cluster status...")
}

func newInstallCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &InitOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install database cluster resource",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.InstallPlayground())
		},
	}

	cmd.Flags().BoolVar(&o.Backup, "backup", false, "playground init the backup module")
	cmd.Flags().StringVar(&rootOptions.AccessKey, "access-key", "", "Cloud provider access key")
	cmd.Flags().StringVar(&rootOptions.AccessSecret, "access-secret", "", "Cloud provider access secret")
	cmd.Flags().StringVar(&rootOptions.Region, "region", "", "Cloud provider region")
	cmd.Flags().StringVar(&o.Engine, "engine", DefaultEngine, "Database engine type")
	cmd.Flags().StringVar(&o.Version, "version", DefaultVersion, "Database engine version")
	cmd.Flags().StringVar(&o.S3Endpoint, "s3-endpoint", DefaultS3Endpoint, "s3 endpoint for backup init")
	cmd.Flags().StringVar(&o.S3Region, "s3-region", DefaultS3Region, "s3 region for backup init")
	cmd.Flags().StringVar(&o.S3Bucket, "s3-bucket", DefaultS3Region, "s3 bucket name for backup init")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Dry run the playground init")
	return cmd
}

func (o *InitOptions) InstallPlayground() error {
	utils.Info("Installing playground database cluster...")

	var err error

	defer func() {
		err := utils.CleanUpPlayground()
		if err != nil {
			utils.Errf("Fail to clean up: %v", err)
		}
	}()

	// Step.1 Install dependencies
	err = installer.InstallDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to install dependencies")
	}

	// Step.2 print guide information
	err = installer.PrintGuide(DefaultCloudProvider, LocalHost)
	if err != nil {
		return errors.Wrap(err, "Failed to print user guide")
	}

	return nil
}

func (o *InitOptions) UnInstallPlayground() error {
	utils.Info("UnInstalling playground database cluster...")

	var err error

	defer func() {
		err := utils.CleanUpPlayground()
		if err != nil {
			utils.Errf("Fail to clean up: %v", err)
		}
	}()

	// UnInstall data protection and mysql operator
	installer.Provider = &provider.DataProtection{}
	err = installer.UnInstallDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to uninstall dependencies")
	}

	// UnInstall bitnami mysql
	installer.Provider = &provider.BitnamiMysql{}
	err = installer.UnInstallDeps()
	if err != nil {
		return errors.Wrap(err, "Failed to uninstall dependencies")
	}
	return nil
}

func newUnInstallCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &InitOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall database cluster resource",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.UnInstallPlayground())
		},
	}
	cmd.Flags().StringVar(&o.Engine, "engine", DefaultEngine, "Database engine type")
	cmd.Flags().StringVar(&o.Version, "version", DefaultVersion, "Database engine version")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Dry run the playground uninstall")

	return cmd
}
