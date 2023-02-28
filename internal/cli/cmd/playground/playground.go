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

package playground

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	cmdcluster "github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/kubeblocks"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	"github.com/apecloud/kubeblocks/version"
)

const (
	yesStr = "yes"
)

type baseOptions struct {
	cloudProvider string
	region        string
}

type initOptions struct {
	genericclioptions.IOStreams
	helmCfg *action.Configuration

	clusterDef     string
	verbose        bool
	kbVersion      string
	clusterVersion string

	baseOptions
}

type destroyOptions struct {
	genericclioptions.IOStreams
	baseOptions
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
	cmd.Flags().StringVar(&o.kbVersion, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringVar(&o.cloudProvider, "cloud-provider", defaultCloudProvider, fmt.Sprintf("Cloud provider type, one of [%s]", strings.Join(cp.CloudProviders(), ",")))
	cmd.Flags().StringVar(&o.region, "region", "", "Cloud provider region")
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "Output more log info")

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cloud-provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return cp.CloudProviders(), cobra.ShellCompDirectiveNoFileComp
		}))
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

	cmd.Flags().StringVar(&o.cloudProvider, "cloud-provider", defaultCloudProvider, fmt.Sprintf("Cloud provider type, one of [%s]", strings.Join(cp.CloudProviders(), ",")))
	cmd.Flags().StringVar(&o.region, "region", "", "Cloud provider region")
	return cmd
}

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			printGuide(false, "")
		},
	}
	return cmd
}

func (o *initOptions) validate() error {
	if o.clusterDef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one")
	}

	if !slices.Contains(cp.CloudProviders(), o.cloudProvider) {
		return fmt.Errorf("%s is not a valid cloud provider", o.cloudProvider)
	}

	if o.cloudProvider != cp.Local && o.cloudProvider != cp.AWS {
		return fmt.Errorf("cloud provider %s is not supported yet", o.cloudProvider)
	}
	return nil
}

func (o *initOptions) run() error {
	if err := initPlaygroundDir(); err != nil {
		return err
	}

	if o.cloudProvider == cp.Local {
		return o.local()
	}
	return o.cloud()
}

// local bootstraps a playground in the local host
func (o *initOptions) local() error {
	var err error
	provider := cp.NewLocalCloudProvider(o.Out, o.ErrOut)
	provider.VerboseLog(o.verbose)

	// Set up K3s as KubeBlocks control plane cluster
	spinner := util.Spinner(o.Out, "Create playground k3d cluster: %s", k8sClusterName)
	defer spinner(false)
	if err = provider.CreateK8sCluster(k8sClusterName, true); err != nil {
		return errors.Wrap(err, "failed to set up k3d cluster")
	}
	spinner(true)

	return o.installKBAndCluster(k8sClusterName)
}

func (o *initOptions) installKBAndCluster(k8sClusterName string) error {
	var err error
	configPath := util.ConfigPath("config")

	// Init helm client
	if o.helmCfg, err = helm.NewActionConfig("", configPath); err != nil {
		return errors.Wrap(err, "failed to init helm client")
	}

	// Install KubeBlocks
	if err = o.installKubeBlocks(); err != nil {
		return errors.Wrap(err, "failed to CreateK8sCluster KubeBlocks")
	}

	// Install database cluster
	if err = o.getClusterVersion(); err != nil {
		return err
	}

	spinner := util.Spinner(o.Out, "Create cluster %s (ClusterDefinition: %s, ClusterVersion: %s)",
		kbClusterName, o.clusterDef, o.clusterVersion)
	defer spinner(false)
	if err = o.installCluster(); err != nil {
		return err
	}
	spinner(true)

	// Print guide information
	printGuide(true, k8sClusterName)
	return nil
}

// bootstraps a playground in the remote cloud
func (o *initOptions) cloud() error {
	printer.Warning(o.Out, `This action will create a kubernetes cluster on the cloud that may
  incur charges. Be sure to delete your infrastructure promptly to avoid
  additional charges. We are not responsible for any charges you may incur.
`)

	// confirm to run
	fmt.Fprintf(o.Out, "\nDo you want to perform this actions?\n  Only 'yes' will be accepted to approve.\n\n")
	entered, err := prompt.NewPrompt("", "Enter a value:", o.In).GetInput()
	if err != nil {
		return err
	}
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground init cancelled.\n")
		return nil
	}

	fmt.Fprintln(o.Out)

	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	// clone apecloud/cloud-provider repo to local path
	spinner := util.Spinner(o.Out, "Clone cloud provider terraform script to %s", cpPath)
	defer spinner(false)
	if err = util.CloneGitRepo(cp.GitRepoURL, cpPath); err != nil {
		return err
	}
	spinner(true)

	// create cloud kubernetes cluster
	provider, err := cp.New(o.cloudProvider, o.region, cpPath, o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	var init bool
	// check if previous cluster exists
	clusterName, _ := getExistedCluster(provider, cpPath)

	// if cluster exists, continue or not, if not, user should destroy the old cluster first
	if clusterName != "" {
		fmt.Fprintf(o.Out, "Found an existed cluster %s, do you want to continue to initialize this cluster?\n  Only 'yes' will be accepted to confirm.\n\n", clusterName)
		entered, err = prompt.NewPrompt("", "Enter a value:", o.In).GetInput()
		if err != nil {
			return err
		}
		if entered != yesStr {
			fmt.Fprintf(o.Out, "\nPlayground init cancelled, please destroy the old cluster first.\n")
			return nil
		}
		fmt.Fprintf(o.Out, "Continue to initialize %s %s cluster %s ... \n", o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	} else {
		init = true
		clusterName = fmt.Sprintf("%s-%s", k8sClusterName, rand.String(5))
		fmt.Fprintf(o.Out, "Creating %s %s cluster %s ... \n", o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	}

	if err = provider.CreateK8sCluster(clusterName, init); err != nil {
		return err
	}

	// CreateK8sCluster KubeBlocks and create cluster
	return o.installKBAndCluster(clusterName)
}

func (o *destroyOptions) destroyPlayground() error {
	if o.cloudProvider == cp.Local {
		return o.destroyLocal()
	}
	return o.destroyCloud()
}

func (o *destroyOptions) destroyLocal() error {
	provider := cp.NewLocalCloudProvider(o.Out, o.ErrOut)
	provider.VerboseLog(false)

	spinner := util.Spinner(o.Out, "Destroy KubeBlocks playground k3d cluster %s", k8sClusterName)
	defer spinner(false)
	// DeleteK8sCluster k3d cluster
	if err := provider.DeleteK8sCluster(k8sClusterName); err != nil {
		return err
	}
	spinner(true)
	return nil
}

func (o *destroyOptions) destroyCloud() error {
	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	// create cloud kubernetes cluster
	provider, err := cp.New(o.cloudProvider, o.region, cpPath, o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	// get cluster name to delete
	name, err := getExistedCluster(provider, cpPath)
	if err != nil {
		return err
	}

	// do not find any existed cluster
	if name == "" {
		fmt.Fprintf(o.Out, "Do not find playground %s %s cluster in %s\n", o.cloudProvider, cp.K8sService(o.cloudProvider), cpPath)
		return nil
	}

	// start to destroy cluster
	printer.Warning(o.Out, "Do you really want to destroy the cluster %s?\n  This is no undo. Only 'yes' will be accepted to confirm.\n\n", name)

	// confirm to destroy
	entered, err := prompt.NewPrompt("", "Enter a value:", o.In).GetInput()
	if err != nil {
		return err
	}
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground destroy cancelled.\n")
		return nil
	}

	fmt.Fprintf(o.Out, "Destroy %s %s cluster %s...\n", o.cloudProvider, cp.K8sService(o.cloudProvider), name)
	if err = provider.DeleteK8sCluster(name); err != nil {
		return err
	}

	return nil
}

func printGuide(init bool, k8sClusterName string) {
	if init {
		fmt.Fprintf(os.Stdout, "\nKubeBlocks playground init SUCCESSFULLY!\n\n")
		if k8sClusterName != "" {
			fmt.Fprintf(os.Stdout, "Kubernetes cluster \"%s\" has been created.\n", k8sClusterName)
		}
		fmt.Fprintf(os.Stdout, "Cluster \"%s\" has been created.\n", kbClusterName)
	}
	fmt.Fprintf(os.Stdout, guideStr, kbClusterName)
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
		Check:   true,
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
	o.clusterVersion, err = cluster.GetLatestVersion(dynamic, o.clusterDef)
	return err
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
			PodAntiAffinity:   "Preferred",
			Tenancy:           "SharedNode",
		},
		ClusterDefRef:     cd,
		ClusterVersionRef: version,
	}

	if err = options.Complete(); err != nil {
		return nil, err
	}
	return options, nil
}

func initPlaygroundDir() error {
	dir, err := playgroundDir()
	if err != nil {
		return err
	}

	if _, err = os.Stat(dir); err != nil && os.IsNotExist(err) {
		return os.MkdirAll(dir, 0750)
	}

	return nil
}

// getExistedCluster get existed playground kubernetes cluster, we should only have one cluster
func getExistedCluster(provider cp.Interface, path string) (string, error) {
	clusterNames, err := provider.GetExistedClusters()
	if err != nil {
		return "", fmt.Errorf("failed to find the existed %s %s cluster in %s", provider.Name(), cp.K8sService(provider.Name()), path)
	}
	if len(clusterNames) > 1 {
		return "", fmt.Errorf("found more than one cluster have been created, check it again, %v", clusterNames)
	}
	if len(clusterNames) == 0 {
		return "", nil
	}
	return clusterNames[0], nil
}
