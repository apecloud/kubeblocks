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

package playground

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	cmdcluster "github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/kubeblocks"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

type initOptions struct {
	genericclioptions.IOStreams
	helmCfg *action.Configuration

	clusterDef     string
	verbose        bool
	kbVersion      string
	clusterVersion string

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
		Use:   "playground [init | destroy | guide]",
		Short: "Bootstrap a KubeBlocks in local host",
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
		Short: "Bootstrap a KubeBlocks for playground",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate())
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.clusterDef, "cluster-definition", defaultClusterDef, "Cluster definition")
	cmd.Flags().StringVar(&o.clusterVersion, "cluster-version", "", "Cluster definition")
	cmd.Flags().StringVar(&o.kbVersion, "kubeblocks-version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringVar(&o.CloudProvider, "cloud-provider", defaultCloudProvider, "Cloud provider type")
	cmd.Flags().StringVar(&o.AccessKey, "access-key", "", "Cloud provider access key")
	cmd.Flags().StringVar(&o.AccessSecret, "access-secret", "", "Cloud provider access secret")
	cmd.Flags().StringVar(&o.Region, "region", "", "Cloud provider region")
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "Output more log info")
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
			util.CheckErr(o.destroyPlayground())
		},
	}
	return cmd
}

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(runGuide())
		},
	}
	return cmd
}

func (o *initOptions) validate() error {
	if o.clusterDef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one")
	}

	return nil
}

func (o *initOptions) run() error {
	if err := initPlaygroundDir(); err != nil {
		return err
	}

	if o.CloudProvider != cloudprovider.Local {
		return o.remote()
	}
	return o.local()
}

// local bootstraps a playground in the local host
func (o *initOptions) local() error {
	var err error
	installer := &installer{
		ctx:         context.Background(),
		clusterName: k8sClusterName,
		IOStreams:   o.IOStreams,
	}
	installer.verboseLog(o.verbose)

	// Set up K3s as KubeBlocks control plane cluster
	spinner := util.Spinner(o.Out, "Create playground k3d cluster: %s", k8sClusterName)
	defer spinner(false)
	if err = installer.install(); err != nil {
		return errors.Wrap(err, "failed to set up k3d cluster")
	}
	spinner(true)

	// Deal with KUBECONFIG
	configPath := util.ConfigPath(k8sClusterName)
	spinner = util.Spinner(o.Out, "Generate kubernetes config %s", configPath)
	defer spinner(false)
	if err = installer.genKubeconfig(); err != nil {
		return errors.Wrap(err, "failed to generate kubeconfig")
	}

	if err = util.SetKubeConfig(configPath); err != nil {
		return errors.Wrap(err, "failed to set KUBECONFIG env")
	}
	spinner(true)

	// Init helm client
	if o.helmCfg, err = helm.NewActionConfig("", util.ConfigPath(k8sClusterName)); err != nil {
		return errors.Wrap(err, "failed to init helm client")
	}

	// Install KubeBlocks
	if err = o.installKubeBlocks(); err != nil {
		return errors.Wrap(err, "failed to install KubeBlocks")
	}

	// Install database cluster
	if err = o.getClusterVersion(); err != nil {
		return err
	}

	spinner = util.Spinner(o.Out, "Create cluster %s (ClusterDefinition: %s, ClusterVersion: %s)",
		kbClusterName, o.clusterDef, o.clusterVersion)
	defer spinner(false)
	if err = o.installCluster(); err != nil {
		return err
	}
	spinner(true)

	// Print guide information
	if err = printGuide(defaultCloudProvider, localHost, true); err != nil {
		return errors.Wrap(err, "failed to print user guide")
	}

	return nil
}

// remote bootstraps a playground in the remote cloud
func (o *initOptions) remote() error {
	// apply changes
	cp, err := cloudprovider.InitProvider(o.CloudProvider, o.AccessKey, o.AccessSecret, o.Region)
	if err != nil {
		return errors.Wrap(err, "failed to create cloud provider")
	}
	if err = cp.Apply(false); err != nil {
		return errors.Wrap(err, "failed to apply changes")
	}
	instance, err := cp.Instance()
	if err != nil {
		return errors.Wrap(err, "failed to query cloud instance")
	}
	kubeConfig := strings.ReplaceAll(kubeConfig, "${KUBERNETES_API_SERVER_ADDRESS}", instance.GetIP())
	kubeConfigPath := path.Join(util.GetKubeconfigDir(), "kubeblocks-playground")
	if err = ioutils.AtomicWriteFile(kubeConfigPath, []byte(kubeConfig), 0700); err != nil {
		return errors.Wrap(err, "failed to update kube config")
	}
	if err = printGuide(cp.Name(), instance.GetIP(), true); err != nil {
		return errors.Wrap(err, "failed to print user guide")
	}
	return nil
}

func (o *destroyOptions) destroyPlayground() error {
	ins := &installer{
		ctx:         context.Background(),
		clusterName: k8sClusterName,
	}

	ins.verboseLog(false)
	spinner := util.Spinner(o.Out, "Destroy KubeBlocks playground")
	defer spinner(false)

	// remote playground, just destroy all cloud resources
	cp, _ := cloudprovider.Get()
	if cp.Name() != cloudprovider.Local {
		var err error
		// remove playground cluster kubeconfig
		if err = util.RemoveConfig(k8sClusterName); err != nil {
			return errors.Wrap(err, "failed to remove playground kubeconfig file")
		}
		if cp, err = cloudprovider.Get(); err != nil {
			return err
		}
		if err = cp.Apply(true); err != nil {
			return err
		}
		spinner(true)
		return nil
	}

	// uninstall k3d cluster
	if err := ins.uninstall(); err != nil {
		return err
	}

	// remove playground directory
	if dir, err := removePlaygroundDir(); err != nil {
		fmt.Fprintf(o.ErrOut, "Failed to remove playground temporary directory %s, you can remove it munally", dir)
	}
	spinner(true)
	return nil
}

func runGuide() error {
	cp, _ := cloudprovider.Get()
	instance, err := cp.Instance()
	if err != nil {
		return err
	}
	return printGuide(cp.Name(), instance.GetIP(), false)
}

func printGuide(cloudProvider string, hostIP string, init bool) error {
	var info = &clusterInfo{
		HostIP:        hostIP,
		CloudProvider: cloudProvider,
		KubeConfig:    util.ConfigPath(k8sClusterName),
		Name:          kbClusterName,
	}

	// check if config file exists
	if _, err := os.Stat(info.KubeConfig); err != nil && os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Try to initialize a playground cluster by running:\n"+
			"\tkbcli playground init\n")
		return err
	}

	if init {
		fmt.Fprintf(os.Stdout, "\nKubeBlocks playground init SUCCESSFULLY!\n"+
			"Cluster \"%s\" has been CREATED!\n", kbClusterName)
	}
	return util.PrintGoTemplate(os.Stdout, guideTmpl, info)
}

func (o *initOptions) installKubeBlocks() error {
	f := util.NewFactory()
	client, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	dynamic, err := f.DynamicClient()
	if err != nil {
		return err
	}
	insOpts := kubeblocks.InstallOptions{
		Options: kubeblocks.Options{
			HelmCfg:   o.helmCfg,
			Namespace: defaultNamespace,
			IOStreams: o.IOStreams,
			Client:    client,
			Dynamic:   dynamic,
		},
		Version: o.kbVersion,
		Monitor: true,
		Quiet:   true,
	}
	return insOpts.Install()
}

func (o *initOptions) installCluster() error {
	// construct a cluster create options and run
	options, err := newCreateOptions(o.clusterDef, o.clusterVersion)
	if err != nil {
		return err
	}

	inputs := create.Inputs{
		BaseOptionsObj:  &options.BaseOptions,
		Options:         options,
		CueTemplateName: cmdcluster.CueTemplateName,
		ResourceName:    types.ResourceClusters,
	}

	return options.Run(inputs)
}

func (o *initOptions) getClusterVersion() error {
	if len(o.clusterVersion) > 0 {
		return nil
	}

	f := util.NewFactory()
	dynamic, err := f.DynamicClient()
	if err != nil {
		return err
	}

	// get component versions that reference this cluster definition
	versionList, err := cluster.GetVersionByClusterDef(dynamic, o.clusterDef)
	if err != nil {
		return err
	}

	// find the latest version to use
	version := findLatestVersion(versionList)
	if version == nil {
		return fmt.Errorf("failed to find component version referencing current cluster definition %s", o.clusterDef)
	}
	o.clusterVersion = version.Name
	return nil
}

func newCreateOptions(cd string, version string) (*cmdcluster.CreateOptions, error) {
	dynamicClient, err := util.NewFactory().DynamicClient()
	if err != nil {
		return nil, err
	}
	options := &cmdcluster.CreateOptions{
		BaseOptions: create.BaseOptions{
			IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			Namespace: defaultNamespace,
			Name:      kbClusterName,
			Client:    dynamicClient,
		},
		UpdatableFlags: cmdcluster.UpdatableFlags{
			TerminationPolicy: "WipeOut",
			Monitor:           true,
		},
		ClusterDefRef:     cd,
		ClusterVersionRef: version,
	}

	if err = options.Complete(); err != nil {
		return nil, err
	}
	return options, nil
}

func findLatestVersion(versions *dbaasv1alpha1.ClusterVersionList) *dbaasv1alpha1.ClusterVersion {
	if len(versions.Items) == 0 {
		return nil
	}
	if len(versions.Items) == 1 {
		return &versions.Items[0]
	}

	var version *dbaasv1alpha1.ClusterVersion
	for i, v := range versions.Items {
		if version == nil {
			version = &versions.Items[i]
			continue
		}
		if v.CreationTimestamp.Time.After(version.CreationTimestamp.Time) {
			version = &versions.Items[i]
		}
	}
	return version
}

func initPlaygroundDir() error {
	playgroundDir, err := util.PlaygroundDir()
	if err != nil {
		return err
	}

	if _, err = os.Stat(playgroundDir); err != nil && os.IsNotExist(err) {
		return os.MkdirAll(playgroundDir, 0750)
	}

	return nil
}

func removePlaygroundDir() (string, error) {
	playgroundDir, err := util.PlaygroundDir()
	if err != nil {
		return playgroundDir, err
	}

	if _, err = os.Stat(playgroundDir); err != nil && os.IsNotExist(err) {
		return playgroundDir, nil
	}

	return playgroundDir, os.RemoveAll(playgroundDir)
}
