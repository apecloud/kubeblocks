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
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/dbaas"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/playground/engine"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

type initOptions struct {
	genericclioptions.IOStreams
	helmCfg *action.Configuration

	Engine   string
	Replicas int
	Verbose  bool

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
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.Engine, "engine", defaultEngine, "Database cluster engine")
	cmd.Flags().StringVar(&o.CloudProvider, "cloud-provider", defaultCloudProvider, "Cloud provider type")
	cmd.Flags().StringVar(&o.AccessKey, "access-key", "", "Cloud provider access key")
	cmd.Flags().StringVar(&o.AccessSecret, "access-secret", "", "Cloud provider access secret")
	cmd.Flags().StringVar(&o.Region, "region", "", "Cloud provider region")
	cmd.Flags().IntVar(&o.Replicas, "replicas", defaultReplicas, "Database cluster replicas")
	cmd.Flags().BoolVar(&o.Verbose, "verbose", false, "Output more log info")
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
			cmdutil.CheckErr(o.destroyPlayground())
		},
	}
	return cmd
}

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(runGuide())
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
		clusterName: clusterName,
		IOStreams:   o.IOStreams,
	}
	installer.verboseLog(o.Verbose)

	// Set up K3s as dbaas control plane cluster
	spinner := util.Spinner(o.Out, "Create playground k3d cluster: %s", clusterName)
	defer spinner(false)
	if err = installer.install(); err != nil {
		return errors.Wrap(err, "Fail to set up k3d cluster")
	}
	spinner(true)

	// Deal with KUBECONFIG
	configPath := util.ConfigPath(clusterName)
	spinner = util.Spinner(o.Out, "Generate kubernetes config %s", configPath)
	defer spinner(false)
	if err = installer.genKubeconfig(); err != nil {
		return errors.Wrap(err, "Fail to generate kubeconfig")
	}

	if err = util.SetKubeConfig(configPath); err != nil {
		return errors.Wrap(err, "Fail to set KUBECONFIG env")
	}
	spinner(true)

	// Init helm client
	if o.helmCfg, err = helm.NewActionConfig("", util.ConfigPath(clusterName)); err != nil {
		return errors.Wrap(err, "Failed to init helm client")
	}

	// Install KubeBlocks
	fmt.Fprintf(o.Out, "Install KubeBlocks %s\n", version.DefaultKubeBlocksVersion)
	if err = o.installKubeBlocks(); err != nil {
		return errors.Wrap(err, "Failed to install KubeBlocks")
	}

	// Install database cluster
	fmt.Fprintf(o.Out, "Install playground cluster %s\n", dbClusterName)
	if err = o.installCluster(); err != nil {
		return errors.Wrap(err, "Failed to install WeSQL")
	}

	// Print guide information
	if err = printGuide(defaultCloudProvider, localHost, int(o.Replicas)); err != nil {
		return errors.Wrap(err, "Failed to print user guide")
	}

	return nil
}

// remote bootstraps a playground in the remote cloud
func (o *initOptions) remote() error {
	// apply changes
	cp, err := cloudprovider.InitProvider(o.CloudProvider, o.AccessKey, o.AccessSecret, o.Region)
	if err != nil {
		return errors.Wrap(err, "Failed to create cloud provider")
	}
	if err = cp.Apply(false); err != nil {
		return errors.Wrap(err, "Failed to apply changes")
	}
	instance, err := cp.Instance()
	if err != nil {
		return errors.Wrap(err, "Failed to query cloud instance")
	}
	kubeConfig := strings.ReplaceAll(kubeConfig, "${KUBERNETES_API_SERVER_ADDRESS}", instance.GetIP())
	kubeConfigPath := path.Join(util.GetKubeconfigDir(), "dbctl-playground")
	if err = ioutils.AtomicWriteFile(kubeConfigPath, []byte(kubeConfig), 0700); err != nil {
		return errors.Wrap(err, "Failed to update kube config")
	}
	if err = printGuide(cp.Name(), instance.GetIP(), o.Replicas); err != nil {
		return errors.Wrap(err, "Failed to print user guide")
	}
	return nil
}

func (o *destroyOptions) destroyPlayground() error {
	installer := &installer{
		ctx:         context.Background(),
		clusterName: clusterName,
	}

	installer.verboseLog(false)
	spinner := util.Spinner(o.Out, "Destroy playground cluster")
	defer spinner(false)

	// remote playground, just destroy all cloud resources
	cp, _ := cloudprovider.Get()
	if cp.Name() != cloudprovider.Local {
		var err error
		// remove playground cluster kubeconfig
		if err = util.RemoveConfig(clusterName); err != nil {
			return errors.Wrap(err, "Failed to remove playground kubeconfig file")
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

	// local playgroundG
	if err := installer.uninstall(); err != nil {
		return err
	}

	// remove playground directory
	if dir, err := removePlaygroundDir(); err != nil {
		fmt.Fprintf(o.ErrOut, "failed to remove playground temporary directory %s, you can remove it munally", dir)
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
	return printGuide(cp.Name(), instance.GetIP(), 0)
}

func printGuide(cloudProvider string, hostIP string, replicas int) error {
	var (
		clusterInfo = &clusterInfo{
			HostIP:         hostIP,
			CloudProvider:  cloudProvider,
			KubeConfig:     util.ConfigPath(clusterName),
			GrafanaPort:    "9100",
			GrafanaUser:    "admin",
			GrafanaPasswd:  "prom-operator",
			ClusterObjects: cluster.NewClusterObjects(),
		}
		err error
	)

	// set env KUBECONFIG to playground kubernetes cluster config
	if err = util.SetKubeConfig(util.ConfigPath(clusterName)); err != nil {
		return err
	}

	f := util.NewFactory()
	clientSet, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}

	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}

	// get cluster info that will be used to render the guide template
	clusterGetter := &cluster.ObjectsGetter{
		ClientSet:     clientSet,
		DynamicClient: dynamicClient,
		Namespace:     dbClusterNamespace,
		Name:          dbClusterName,
	}
	if err = clusterGetter.Get(clusterInfo.ClusterObjects); err != nil {
		return err
	}

	// get the real number of replica for existed database cluster
	if replicas == 0 {
		cluster := clusterInfo.Cluster
		if cluster == nil {
			return fmt.Errorf("failed to get cluster \"%s\" info", dbClusterName)
		}
		replicas = int(cluster.Spec.Components[0].Replicas)
	}

	// build host port to access database
	clusterInfo.HostPorts = make([]string, replicas)
	for i := 0; i < replicas; i++ {
		clusterInfo.HostPorts[i] = strconv.Itoa(3306 + i)
	}

	return util.PrintGoTemplate(os.Stdout, guideTmpl, clusterInfo)
}

func (o *initOptions) installKubeBlocks() error {
	installer := dbaas.Installer{
		HelmCfg:   o.helmCfg,
		Namespace: dbClusterNamespace,
		Version:   version.DefaultKubeBlocksVersion,
	}

	_, err := installer.Install()
	return err
}

func (o *initOptions) installCluster() error {
	engine, err := engine.New(o.Engine)
	if err != nil {
		return err
	}

	return engine.Install(o.Replicas, dbClusterName, dbClusterNamespace)
}

func initPlaygroundDir() error {
	playgroundDir, err := util.PlaygroundDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(playgroundDir); err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(playgroundDir, 0750)
		return err
	}

	return nil
}

func removePlaygroundDir() (string, error) {
	playgroundDir, err := util.PlaygroundDir()
	if err != nil {
		return playgroundDir, err
	}

	if _, err := os.Stat(playgroundDir); err != nil && os.IsNotExist(err) {
		return playgroundDir, nil
	}

	if err := os.RemoveAll(playgroundDir); err != nil {
		return playgroundDir, err
	}

	return playgroundDir, nil
}
