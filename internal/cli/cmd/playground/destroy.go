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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/kubeblocks"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

var (
	destroyExample = templates.Examples(`
		# destroy playground cluster
		kbcli playground destroy`)
)

type destroyOptions struct {
	genericclioptions.IOStreams
	baseOptions
}

func newDestroyCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &destroyOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "destroy",
		Short:   "Destroy the playground kubernetes cluster.",
		Example: destroyExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.validate())
			util.CheckErr(o.destroy())
		},
	}
	return cmd
}

func newGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Display playground cluster user guide.",
		Run: func(cmd *cobra.Command, args []string) {
			printGuide()
		},
	}
	return cmd
}

func (o *destroyOptions) destroy() error {
	if o.prevCluster == nil {
		return fmt.Errorf("no playground cluster found")
	}

	if o.prevCluster.CloudProvider == cp.Local {
		return o.destroyLocal()
	}
	return o.destroyCloud()
}

// destroyLocal destroy local k3d cluster that will destroy all resources
func (o *destroyOptions) destroyLocal() error {
	// for test
	if err := o.deleteClustersAndUninstallKB(); err != nil {
		return err
	}

	provider := cp.NewLocalCloudProvider(o.Out, o.ErrOut)
	spinner := printer.Spinner(o.Out, "%-50s", "Delete playground k3d cluster "+o.prevCluster.ClusterName)
	defer spinner(false)
	if err := provider.DeleteK8sCluster(o.prevCluster); err != nil {
		if !strings.Contains(err.Error(), "no cluster found") &&
			!strings.Contains(err.Error(), "does not exist") {
			return err
		}
	}
	spinner(true)

	if err := o.removeKubeConfig(); err != nil {
		return err
	}
	return o.removeStateFile()
}

// destroyCloud destroy cloud kubernetes cluster, before destroy, we should delete
// all clusters created by KubeBlocks, uninstall KubeBlocks and remove the KubeBlocks
// namespace that will destroy all resources created by KubeBlocks, avoid to leave
// some resources
func (o *destroyOptions) destroyCloud() error {
	var err error

	// start to destroy cluster
	printer.Warning(o.Out, `This action will uninstall KubeBlocks and delete the kubernetes cluster,
  there may be residual resources, please confirm and manually clean up related
  resources after this action.

`)

	fmt.Fprintf(o.Out, "Do you really want to destroy the kubernetes cluster %s?\n%s\n\n  This is no undo. Only 'yes' will be accepted to confirm.\n\n",
		o.prevCluster.ClusterName, o.prevCluster.String())

	// confirm to destroy
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground destroy cancelled.\n")
		return cmdutil.ErrExit
	}

	o.startTime = time.Now()

	// for cloud provider, we should delete all clusters created by KubeBlocks first,
	// uninstall KubeBlocks and remove the KubeBlocks namespace, then destroy the
	// playground cluster, avoid to leave some resources.
	// delete all clusters created by KubeBlocks, MUST BE VERY CAUTIOUS, use the right
	// kubeconfig and context, otherwise, it will delete the wrong cluster.
	if err = o.deleteClustersAndUninstallKB(); err != nil {
		return err
	}

	// destroy playground kubernetes cluster
	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	provider, err := cp.New(o.prevCluster.CloudProvider, cpPath, o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Destroy %s %s cluster %s...\n",
		o.prevCluster.CloudProvider, cp.K8sService(o.prevCluster.CloudProvider), o.prevCluster.ClusterName)
	if err = provider.DeleteK8sCluster(o.prevCluster); err != nil {
		return err
	}

	// remove the cluster kubeconfig from the use default kubeconfig
	if err = o.removeKubeConfig(); err != nil {
		return err
	}

	// at last, remove the state file
	if err = o.removeStateFile(); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Playground destroy completed in %s.\n", time.Since(o.startTime).Truncate(time.Second))
	return nil
}

func (o *destroyOptions) deleteClustersAndUninstallKB() error {
	var err error

	if o.prevCluster.KubeConfig == "" {
		fmt.Fprintf(o.Out, "No kubeconfig found for kubernetes cluster %s in %s \n",
			o.prevCluster.ClusterName, o.stateFilePath)
		return nil
	}

	// write kubeconfig content to a temporary file and use it
	if err = writeAndUseKubeConfig(o.prevCluster.KubeConfig, o.kubeConfigPath, o.Out); err != nil {
		return err
	}

	// delete all clusters created by KubeBlocks
	if err = o.deleteClusters(); err != nil {
		return err
	}

	// uninstall KubeBlocks and remove namespace created by KubeBlocks
	return o.uninstallKubeBlocks(o.kubeConfigPath)
}

// delete all clusters created by KubeBlocks
func (o *destroyOptions) deleteClusters() error {
	ctx := context.Background()

	// the caller should ensure the kubeconfig is set
	f := util.NewFactory()
	dynamic, err := f.DynamicClient()
	if err != nil {
		return err
	}

	// get all clusters in all namespaces
	getClusters := func() (*unstructured.UnstructuredList, error) {
		return dynamic.Resource(types.ClusterGVR()).Namespace(metav1.NamespaceAll).
			List(context.Background(), metav1.ListOptions{})
	}

	// get all clusters and check if satisfy the checkFn
	checkClusters := func(checkFn func(cluster *appsv1alpha1.Cluster) bool) (bool, error) {
		res := true
		clusters, err := getClusters()
		if err != nil {
			return false, err
		}
		for _, item := range clusters.Items {
			cluster := &appsv1alpha1.Cluster{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, cluster); err != nil {
				return false, err
			}
			if !checkFn(cluster) {
				res = false
				break
			}
		}
		return res, nil
	}

	// delete all clusters
	deleteClusters := func(clusters *unstructured.UnstructuredList) error {
		for _, cluster := range clusters.Items {
			if err = dynamic.Resource(types.ClusterGVR()).Namespace(cluster.GetNamespace()).
				Delete(ctx, cluster.GetName(), *metav1.NewDeleteOptions(0)); err != nil {
				return err
			}
		}
		return nil
	}

	spinner := printer.Spinner(o.Out, fmt.Sprintf("%-50s", "Delete clusters created by KubeBlocks"))
	defer spinner(false)

	// get all clusters
	clusters, err := getClusters()
	if clusters == nil || len(clusters.Items) == 0 {
		spinner(true)
		return nil
	}

	checkWipeOut := false
	// set all cluster termination policy to WipeOut to delete all resources, otherwise
	// the cluster will be deleted but the resources will be left
	for _, item := range clusters.Items {
		cluster := &appsv1alpha1.Cluster{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, cluster); err != nil {
			return err
		}
		if cluster.Spec.TerminationPolicy == appsv1alpha1.WipeOut {
			continue
		}

		// terminate policy is not WipeOut, set it to WipeOut
		klog.V(1).Infof("Set cluster %s termination policy to WipeOut", cluster.Name)
		if _, err = dynamic.Resource(types.ClusterGVR()).Namespace(cluster.Namespace).Patch(ctx, cluster.Name, apitypes.JSONPatchType,
			[]byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/spec/terminationPolicy\", \"value\": \"%s\" }]",
				appsv1alpha1.WipeOut)), metav1.PatchOptions{}); err != nil {
			return err
		}

		// set some cluster termination policy to WipeOut, need to check again
		checkWipeOut = true
	}

	// check all clusters termination policy is WipeOut
	if checkWipeOut {
		if err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			return checkClusters(func(cluster *appsv1alpha1.Cluster) bool {
				if cluster.Spec.TerminationPolicy != appsv1alpha1.WipeOut {
					klog.V(1).Infof("Cluster %s termination policy is %s", cluster.Name, cluster.Spec.TerminationPolicy)
				}
				return cluster.Spec.TerminationPolicy == appsv1alpha1.WipeOut
			})
		}); err != nil {
			return err
		}
	}

	// delete all clusters
	if err = deleteClusters(clusters); err != nil {
		return err
	}

	// check and wait all clusters are deleted
	if err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		return checkClusters(func(cluster *appsv1alpha1.Cluster) bool {
			// always return false if any cluster is not deleted
			klog.V(1).Infof("Cluster %s is not deleted", cluster.Name)
			return false
		})
	}); err != nil {
		return err
	}

	spinner(true)
	return nil
}

func (o *destroyOptions) uninstallKubeBlocks(configPath string) error {
	var err error
	f := util.NewFactory()
	uninstall := kubeblocks.UninstallOptions{
		Factory: f,
		Options: kubeblocks.Options{
			IOStreams: o.IOStreams,
		},
		AutoApprove:     true,
		RemoveNamespace: true,
		Quiet:           true,
	}

	uninstall.Client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	uninstall.Dynamic, err = f.DynamicClient()
	if err != nil {
		return err
	}

	uninstall.HelmCfg = helm.NewConfig("", configPath, "", klog.V(1).Enabled())
	if err = uninstall.PreCheck(); err != nil {
		return err
	}
	if err = uninstall.Uninstall(); err != nil {
		return err
	}
	return nil
}

func (o *destroyOptions) removeKubeConfig() error {
	spinner := printer.Spinner(o.Out, "%-50s", "Remove kubeconfig from "+defaultKubeConfigPath)
	defer spinner(false)
	if err := kubeConfigRemove(o.prevCluster.KubeConfig, defaultKubeConfigPath); err != nil {
		if os.IsNotExist(err) {
			spinner(true)
			return nil
		} else {
			return err
		}
	}
	spinner(true)

	clusterContext, err := kubeConfigCurrentContext(o.prevCluster.KubeConfig)
	if err != nil {
		return err
	}

	// check if current context in kubeconfig is deleted, if yes, notify user to set current context
	currentContext, err := kubeConfigCurrentContextFromFile(defaultKubeConfigPath)
	if err != nil {
		return err
	}

	// current context is deleted, notify user to set current context like kubectl
	if currentContext == clusterContext {
		printer.Warning(o.Out, "this removed your active context, use \"kubectl config use-context\" to select a different one\n")
	}
	return nil
}

// remove state file
func (o *destroyOptions) removeStateFile() error {
	spinner := printer.Spinner(o.Out, "Remove state file %s", o.stateFilePath)
	defer spinner(false)
	if err := removeStateFile(o.stateFilePath); err != nil {
		return err
	}
	spinner(true)
	return nil
}
