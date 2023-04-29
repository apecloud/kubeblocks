/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package playground

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
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
		kbcli playground init --cloud-provider aws --region cn-northwest-1

		# create an Alibaba cloud ACK cluster and install KubeBlocks, the region is required
		kbcli playground init --cloud-provider alicloud --region cn-hangzhou

		# create a Tencent cloud TKE cluster and install KubeBlocks, the region is required
		kbcli playground init --cloud-provider tencentcloud --region ap-chengdu

		# create a Google cloud GKE cluster and install KubeBlocks, the region is required
		kbcli playground init --cloud-provider gcp --region us-central1`)

	supportedCloudProviders = []string{cp.Local, cp.AWS, cp.GCP, cp.AliCloud, cp.TencentCloud}
)

type initOptions struct {
	genericclioptions.IOStreams
	helmCfg        *helm.Config
	clusterDef     string
	kbVersion      string
	clusterVersion string
	cloudProvider  string
	region         string

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
	cmd.Flags().StringVar(&o.cloudProvider, "cloud-provider", defaultCloudProvider, fmt.Sprintf("Cloud provider type, one of %v", supportedCloudProviders))
	cmd.Flags().StringVar(&o.region, "region", "", "The region to create kubernetes cluster")

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"cloud-provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return cp.CloudProviders(), cobra.ShellCompDirectiveNoFileComp
		}))
	return cmd
}

func (o *initOptions) validate() error {
	if !slices.Contains(supportedCloudProviders, o.cloudProvider) {
		return fmt.Errorf("cloud provider %s is not supported, only support %v", o.cloudProvider, supportedCloudProviders)
	}

	if o.cloudProvider != cp.Local && o.region == "" {
		return fmt.Errorf("region should be specified when cloud provider %s is specified", o.cloudProvider)
	}

	if o.clusterDef == "" {
		return fmt.Errorf("a valid cluster definition is needed, use --cluster-definition to specify one")
	}

	if err := o.baseOptions.validate(); err != nil {
		return err
	}
	return o.checkExistedCluster()
}

func (o *initOptions) run() error {
	if o.cloudProvider == cp.Local {
		return o.local()
	}
	return o.cloud()
}

// local bootstraps a playground in the local host
func (o *initOptions) local() error {
	provider, err := cp.New(o.cloudProvider, "", o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	o.startTime = time.Now()

	var clusterInfo *cp.K8sClusterInfo
	if o.prevCluster != nil {
		clusterInfo = o.prevCluster
	} else {
		clusterInfo = &cp.K8sClusterInfo{
			CloudProvider: provider.Name(),
			ClusterName:   types.K3dClusterName,
		}
	}

	if err = writeClusterInfo(o.stateFilePath, clusterInfo); err != nil {
		return errors.Wrapf(err, "failed to write kubernetes cluster info to state file %s:\n  %v", o.stateFilePath, clusterInfo)
	}

	// create a local kubernetes cluster (k3d cluster) to deploy KubeBlocks
	spinner := printer.Spinner(o.Out, "%-50s", "Create k3d cluster: "+clusterInfo.ClusterName)
	defer spinner(false)
	if err = provider.CreateK8sCluster(clusterInfo); err != nil {
		return errors.Wrap(err, "failed to set up k3d cluster")
	}
	spinner(true)

	clusterInfo, err = o.writeStateFile(provider)
	if err != nil {
		return err
	}

	if err = o.setKubeConfig(clusterInfo); err != nil {
		return err
	}

	// install KubeBlocks and create a database cluster
	return o.installKBAndCluster(clusterInfo)
}

// bootstraps a playground in the remote cloud
func (o *initOptions) cloud() error {
	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	var clusterInfo *cp.K8sClusterInfo

	// if kubernetes cluster exists, confirm to continue or not, if not, user should
	// destroy the old cluster first
	if o.prevCluster != nil {
		clusterInfo = o.prevCluster
		if err = o.confirmToContinue(); err != nil {
			return err
		}
	} else {
		clusterName := fmt.Sprintf("%s-%s", cloudClusterNamePrefix, rand.String(5))
		clusterInfo = &cp.K8sClusterInfo{
			ClusterName:   clusterName,
			CloudProvider: o.cloudProvider,
			Region:        o.region,
		}
		if err = o.confirmInitNewKubeCluster(); err != nil {
			return err
		}

		fmt.Fprintf(o.Out, "\nWrite cluster info to state file %s\n", o.stateFilePath)
		if err := writeClusterInfo(o.stateFilePath, clusterInfo); err != nil {
			return errors.Wrapf(err, "failed to write kubernetes cluster info to state file %s:\n  %v", o.stateFilePath, clusterInfo)
		}

		fmt.Fprintf(o.Out, "Creating %s %s cluster %s ... \n", o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	}

	o.startTime = time.Now()
	printer.PrintBlankLine(o.Out)

	// clone apecloud/cloud-provider repo to local path
	fmt.Fprintf(o.Out, "Clone ApeCloud cloud-provider repo to %s...\n", cpPath)
	branchName := "kb-playground"
	if version.Version != "" && version.Version != "edge" {
		branchName = fmt.Sprintf("%s-%s", branchName, strings.Split(version.Version, "-")[0])
	}
	if err = util.CloneGitRepo(cp.GitRepoURL, branchName, cpPath); err != nil {
		return err
	}

	provider, err := cp.New(o.cloudProvider, cpPath, o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	// create a kubernetes cluster in the cloud
	if err = provider.CreateK8sCluster(clusterInfo); err != nil {
		return err
	}
	printer.PrintBlankLine(o.Out)

	// write cluster info to state file and get new cluster info with kubeconfig
	clusterInfo, err = o.writeStateFile(provider)
	if err != nil {
		return err
	}

	// write cluster kubeconfig to default kubeconfig file and switch current context to it
	if err = o.setKubeConfig(clusterInfo); err != nil {
		return err
	}

	// install KubeBlocks and create a database cluster
	return o.installKBAndCluster(clusterInfo)
}

// confirmToContinue confirms to continue init or not if there is an existed kubernetes cluster
func (o *initOptions) confirmToContinue() error {
	clusterName := o.prevCluster.ClusterName
	printer.Warning(o.Out, "Found an existed cluster %s, do you want to continue to initialize this cluster?\n  Only 'yes' will be accepted to confirm.\n\n", clusterName)
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground init cancelled, please destroy the old cluster first.\n")
		return cmdutil.ErrExit
	}
	fmt.Fprintf(o.Out, "Continue to initialize %s %s cluster %s... \n",
		o.cloudProvider, cp.K8sService(o.cloudProvider), clusterName)
	return nil
}

func (o *initOptions) confirmInitNewKubeCluster() error {
	printer.Warning(o.Out, `This action will create a kubernetes cluster on the cloud that may
  incur charges. Be sure to delete your infrastructure promptly to avoid
  additional charges. We are not responsible for any charges you may incur.
`)

	fmt.Fprintf(o.Out, `
The whole process will take about %s, please wait patiently,
if it takes a long time, please check the network environment and try again.
`, printer.BoldRed("20 minutes"))

	// confirm to run
	fmt.Fprintf(o.Out, "\nDo you want to perform this action?\n  Only 'yes' will be accepted to approve.\n\n")
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground init cancelled.\n")
		return cmdutil.ErrExit
	}
	return nil
}

func printGuide() {
	fmt.Fprintf(os.Stdout, guideStr, kbClusterName)
}

// writeStateFile writes cluster info to state file and return the new cluster info with kubeconfig
func (o *initOptions) writeStateFile(provider cp.Interface) (*cp.K8sClusterInfo, error) {
	clusterInfo, err := provider.GetClusterInfo()
	if err != nil {
		return nil, err
	}
	if clusterInfo.KubeConfig == "" {
		return nil, errors.New("failed to get kubernetes cluster kubeconfig")
	}
	if err = writeClusterInfo(o.stateFilePath, clusterInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to write kubernetes cluster info to state file %s:\n  %v",
			o.stateFilePath, clusterInfo)
	}
	return clusterInfo, nil
}

// merge created kubernetes cluster kubeconfig to ~/.kube/config and set it as default
func (o *initOptions) setKubeConfig(info *cp.K8sClusterInfo) error {
	spinner := printer.Spinner(o.Out, "%-50s", "Merge kubeconfig to "+defaultKubeConfigPath)
	defer spinner(false)

	// check if the default kubeconfig file exists, if not, create it
	if _, err := os.Stat(defaultKubeConfigPath); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(defaultKubeConfigPath), 0755); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", filepath.Dir(defaultKubeConfigPath))
		}
		if err = os.WriteFile(defaultKubeConfigPath, []byte{}, 0644); err != nil {
			return errors.Wrapf(err, "failed to create file %s", defaultKubeConfigPath)
		}
	}

	if err := kubeConfigWrite(info.KubeConfig, defaultKubeConfigPath,
		writeKubeConfigOptions{UpdateExisting: true, UpdateCurrentContext: true}); err != nil {
		return errors.Wrapf(err, "failed to write cluster %s kubeconfig", info.ClusterName)
	}
	spinner(true)

	currentContext, err := kubeConfigCurrentContext(info.KubeConfig)
	spinner = printer.Spinner(o.Out, "%-50s", "Switch current context to "+currentContext)
	defer spinner(false)
	if err != nil {
		return err
	}
	spinner(true)

	return nil
}

func (o *initOptions) installKBAndCluster(info *cp.K8sClusterInfo) error {
	var err error

	// when the kubernetes cluster is not ready, the runtime will output the error
	// message like "couldn't get resource list for", we ignore it
	runtime.ErrorHandlers[0] = func(err error) {
		if klog.V(1).Enabled() {
			klog.ErrorDepth(2, err)
		}
	}

	// write kubeconfig content to a temporary file and use it
	if err = writeAndUseKubeConfig(info.KubeConfig, o.kubeConfigPath, o.Out); err != nil {
		return err
	}

	// create helm config
	o.helmCfg = helm.NewConfig("", o.kubeConfigPath, "", klog.V(1).Enabled())

	// install KubeBlocks
	if err = o.installKubeBlocks(info.ClusterName); err != nil {
		return errors.Wrap(err, "failed to install KubeBlocks")
	}

	// install database cluster
	clusterInfo := "ClusterDefinition: " + o.clusterDef
	if o.clusterVersion != "" {
		clusterInfo += ", ClusterVersion: " + o.clusterVersion
	}
	spinner := printer.Spinner(o.Out, "Create cluster %s (%s)", kbClusterName, clusterInfo)
	defer spinner(false)
	if err = o.createCluster(); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "failed to create cluster %s", kbClusterName)
	}
	spinner(true)

	fmt.Fprintf(os.Stdout, "\nKubeBlocks playground init SUCCESSFULLY!\n\n")
	fmt.Fprintf(os.Stdout, "Kubernetes cluster \"%s\" has been created.\n", info.ClusterName)
	fmt.Fprintf(os.Stdout, "Cluster \"%s\" has been created.\n", kbClusterName)

	// output elapsed time
	if !o.startTime.IsZero() {
		fmt.Fprintf(o.Out, "Elapsed time: %s\n", time.Since(o.startTime).Truncate(time.Second))
	}

	printGuide()
	return nil
}

func (o *initOptions) installKubeBlocks(k8sClusterName string) error {
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
			// use hostpath csi driver to support snapshot
			"snapshot-controller.enabled=true",
			"csi-hostpath-driver.enabled=true",

			// enable aws loadbalancer controller addon automatically on playground
			"aws-loadbalancer-controller.enabled=true",
			fmt.Sprintf("aws-loadbalancer-controller.clusterName=%s", k8sClusterName),

			// disable the persistent volume of prometheus, if not, the prometheus
			// will dependent the hostpath csi driver ready to create persistent
			// volume, but the order of addon installation is not guaranteed that
			// will cause the prometheus PVC pending forever.
			"prometheus.server.persistentVolume.enabled=false",
			"prometheus.server.statefulSet.enabled=false",
			"prometheus.alertmanager.persistentVolume.enabled=false",
			"prometheus.alertmanager.statefulSet.enabled=false")
	} else if o.cloudProvider == cp.AWS {
		insOpts.ValueOpts.Values = append(insOpts.ValueOpts.Values,
			// enable aws-load-balancer-controller addon automatically on playground
			"aws-load-balancer-controller.enabled=true",
			fmt.Sprintf("aws-load-balancer-controller.clusterName=%s", k8sClusterName),
		)
	}

	return insOpts.Install()
}

// createCluster construct a cluster create options and run
func (o *initOptions) createCluster() error {
	// construct a cluster create options and run
	options, err := o.newCreateOptions()
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

// checkExistedCluster check playground kubernetes cluster exists or not, playground
// only supports one kubernetes cluster exists at the same time
func (o *initOptions) checkExistedCluster() error {
	if o.prevCluster == nil {
		return nil
	}

	warningMsg := fmt.Sprintf("playground only supports one kubernetes cluster at the same time,\n  one cluster already existed, please destroy it first.\n%s\n", o.prevCluster.String())
	// if cloud provider is not same with the exited cluster cloud provider, informer
	// user to destroy the previous cluster first
	if o.prevCluster.CloudProvider != o.cloudProvider {
		printer.Warning(o.Out, warningMsg)
		return cmdutil.ErrExit
	}

	if o.prevCluster.CloudProvider == cp.Local {
		return nil
	}

	// previous kubernetes cluster is a cloud provider cluster, check if the region
	// is same with the new cluster region, if not, informer user to destroy the previous
	// cluster first
	if o.prevCluster.Region != o.region {
		printer.Warning(o.Out, warningMsg)
		return cmdutil.ErrExit
	}
	return nil
}

func (o *initOptions) newCreateOptions() (*cmdcluster.CreateOptions, error) {
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
		ClusterDefRef:     o.clusterDef,
		ClusterVersionRef: o.clusterVersion,
	}

	// if we are running on cloud, create cluster with three replicas
	if o.cloudProvider != cp.Local {
		options.Values = append(options.Values, "replicas=3")
	}

	if err = options.Complete(); err != nil {
		return nil, err
	}
	return options, nil
}
