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
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
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

var (
	initExample = templates.Examples(`
		# create a k3d cluster on local host and install KubeBlocks 
		kbcli playground init

		# create an AWS EKS cluster and install KubeBlocks, the region is required
		kbcli playground init --cloud-provider aws --region cn-northwest-1`)
)

type baseOptions struct {
	cloudProvider string
	region        string
	startTime     time.Time
}

type initOptions struct {
	genericclioptions.IOStreams
	helmCfg        *helm.Config
	clusterDef     string
	verbose        bool
	kbVersion      string
	clusterVersion string

	baseOptions
}

func newInitCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &initOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Bootstrap a kubernetes cluster and install KubeBlocks for playground.",
		Example: initExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate())
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.clusterDef, "cluster-definition", defaultClusterDef, "Cluster definition")
	cmd.Flags().StringVar(&o.clusterVersion, "cluster-version", "", "Cluster definition")
	cmd.Flags().StringVar(&o.kbVersion, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringVar(&o.cloudProvider, "cloud-provider", defaultCloudProvider, fmt.Sprintf("Cloud provider type, one of [%s]", strings.Join(cp.CloudProviders(), ",")))
	cmd.Flags().StringVar(&o.region, "region", "", "The region to create kubernetes cluster")
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "Output more log info")

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cloud-provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return cp.CloudProviders(), cobra.ShellCompDirectiveNoFileComp
		}))
	return cmd
}

func (o *baseOptions) validate() error {
	if !slices.Contains(cp.CloudProviders(), o.cloudProvider) {
		return fmt.Errorf("%s is not a valid cloud provider", o.cloudProvider)
	}

	if o.cloudProvider != cp.Local && o.cloudProvider != cp.AWS {
		return fmt.Errorf("cloud provider %s is not supported yet", o.cloudProvider)
	}

	if o.cloudProvider != cp.Local && o.region == "" {
		return fmt.Errorf("when cloud provider %s is specified, region should be specified", o.cloudProvider)
	}
	return nil
}

func (o *initOptions) validate() error {
	if o.clusterDef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one")
	}

	return o.baseOptions.validate()
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

	o.startTime = time.Now()
	// Set up K3s as KubeBlocks control plane cluster
	spinner := util.Spinner(o.Out, "%-40s", "Create k3d cluster: "+k8sClusterName)
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

	// create helm config
	o.helmCfg = helm.NewConfig("", configPath, "", o.verbose)

	// Install KubeBlocks
	if err = o.installKubeBlocks(); err != nil {
		return errors.Wrap(err, "failed to install KubeBlocks")
	}

	// Install database cluster
	spinner := util.Spinner(o.Out, "Create cluster %s (ClusterDefinition: %s, ClusterVersion: %s)",
		kbClusterName, o.clusterDef, o.clusterVersion)
	defer spinner(false)
	if err = o.createCluster(); err != nil {
		return errors.Wrapf(err, "failed to create cluster %s", kbClusterName)
	}
	spinner(true)

	// Print guide information
	fmt.Fprintf(os.Stdout, "\nKubeBlocks playground init SUCCESSFULLY!\n\n")
	if k8sClusterName != "" {
		fmt.Fprintf(os.Stdout, "Kubernetes cluster \"%s\" has been created.\n", k8sClusterName)
	}
	fmt.Fprintf(os.Stdout, "Cluster \"%s\" has been created.\n", kbClusterName)

	// output elapsed time
	if !o.startTime.IsZero() {
		fmt.Fprintf(o.Out, "Elapsed time: %s\n", time.Since(o.startTime).Truncate(time.Second))
	}

	printGuide()

	return nil
}

// bootstraps a playground in the remote cloud
func (o *initOptions) cloud() error {
	printer.Warning(o.Out, `This action will create a kubernetes cluster on the cloud that may
  incur charges. Be sure to delete your infrastructure promptly to avoid
  additional charges. We are not responsible for any charges you may incur.
`)

	fmt.Fprintf(o.Out, `
The whole process takes about %s, please wait patiently,
if it takes a long time, please check the network environment and try again.
`, printer.BoldRed("20~30 minutes"))

	// confirm to run
	fmt.Fprintf(o.Out, "\nDo you want to perform this action?\n  Only 'yes' will be accepted to approve.\n\n")
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground init cancelled.\n")
		return cmdutil.ErrExit
	}

	o.startTime = time.Now()
	printer.PrintBlankLine(o.Out)
	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	// clone apecloud/cloud-provider repo to local path
	fmt.Fprintf(o.Out, "Clone cloud provider terraform script to %s...\n", cpPath)
	if err = util.CloneGitRepo(cp.GitRepoURL, cpPath); err != nil {
		return err
	}

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
		entered, _ = prompt.NewPrompt("Enter a value:", nil, o.In).Run()
		if entered != yesStr {
			fmt.Fprintf(o.Out, "\nPlayground init cancelled, please destroy the old cluster first.\n")
			return cmdutil.ErrExit
		}
		fmt.Fprintf(o.Out, "Continue to initialize %s %s cluster %s... \n", o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	} else {
		init = true
		clusterName = fmt.Sprintf("%s-%s", k8sClusterName, rand.String(5))
		fmt.Fprintf(o.Out, "Creating %s %s cluster %s ... \n", o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	}

	if err = provider.CreateK8sCluster(clusterName, init); err != nil {
		return err
	}

	printer.PrintBlankLine(o.Out)
	// CreateK8sCluster KubeBlocks and create cluster
	return o.installKBAndCluster(clusterName)
}

func printGuide() {
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
	if o.cloudProvider == cp.Local {
		insOpts.ValueOpts.Values = append(insOpts.ValueOpts.Values,
			"snapshot-controller.enabled=true", "csi-hostpath-driver.enabled=true")
	}
	return insOpts.Install()
}

func (o *initOptions) createCluster() error {
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
			Dynamic:   dynamicClient,
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
	if err = options.Validate(); err != nil {
		return nil, err
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
