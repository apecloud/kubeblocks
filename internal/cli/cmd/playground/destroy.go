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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/kubeblocks"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/spinner"
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

	// purge resources, before destroy kubernetes cluster we should delete cluster and
	// uninstall KubeBlocks
	purge   bool
	timeout time.Duration
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

	cmd.Flags().BoolVar(&o.purge, "purge", true, "Purge all resources before destroy kubernetes cluster, delete all clusters created by KubeBlocks and uninstall KubeBlocks.")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for installing KubeBlocks, such as --timeout=10m")

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
	provider, _ := cp.New(cp.Local, "", o.Out, o.ErrOut)
	s := spinner.New(o.Out, spinnerMsg("Delete playground k3d cluster "+o.prevCluster.ClusterName))
	defer s.Fail()
	if err := provider.DeleteK8sCluster(o.prevCluster); err != nil {
		if !strings.Contains(err.Error(), "no cluster found") &&
			!strings.Contains(err.Error(), "does not exist") {
			return err
		}
	}
	s.Success()

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

	printer.Warning(o.Out, `This action will destroy the kubernetes cluster, there may be residual resources,
  please confirm and manually clean up related resources after this action.

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
		if strings.Contains(err.Error(), kubeClusterUnreachableErr.Error()) {
			printer.Warning(o.Out, err.Error())
		} else {
			return err
		}
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

	if !o.purge {
		klog.V(1).Infof("Skip to delete all clusters created by KubeBlocks and uninstall KubeBlocks")
		return nil
	}

	if o.prevCluster.KubeConfig == "" {
		fmt.Fprintf(o.Out, "No kubeconfig found for kubernetes cluster %s in %s \n",
			o.prevCluster.ClusterName, o.stateFilePath)
		return nil
	}

	// write kubeconfig content to a temporary file and use it
	if err = writeAndUseKubeConfig(o.prevCluster.KubeConfig, o.kubeConfigPath, o.Out); err != nil {
		return err
	}

	client, dynamic, err := getKubeClient()
	if err != nil {
		return err
	}

	// delete all clusters created by KubeBlocks
	if err = o.deleteClusters(dynamic); err != nil {
		return err
	}

	// uninstall KubeBlocks and remove namespace created by KubeBlocks
	return o.uninstallKubeBlocks(client, dynamic)
}

// delete all clusters created by KubeBlocks
func (o *destroyOptions) deleteClusters(dynamic dynamic.Interface) error {
	var err error
	ctx := context.Background()

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

	s := spinner.New(o.Out, spinnerMsg("Delete clusters created by KubeBlocks"))
	defer s.Fail()

	// get all clusters
	clusters, err := getClusters()
	if clusters == nil || len(clusters.Items) == 0 {
		s.Success()
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

	s.Success()
	return nil
}

func (o *destroyOptions) uninstallKubeBlocks(client kubernetes.Interface, dynamic dynamic.Interface) error {
	var err error
	uninstall := kubeblocks.UninstallOptions{
		Options: kubeblocks.Options{
			IOStreams: o.IOStreams,
			Client:    client,
			Dynamic:   dynamic,
			Wait:      true,
		},
		AutoApprove:     true,
		RemoveNamespace: true,
		Quiet:           true,
	}

	uninstall.HelmCfg = helm.NewConfig("", o.kubeConfigPath, "", klog.V(1).Enabled())
	if err = uninstall.PreCheck(); err != nil {
		return err
	}
	if err = uninstall.Uninstall(); err != nil {
		return err
	}
	return nil
}

func (o *destroyOptions) removeKubeConfig() error {
	s := spinner.New(o.Out, spinnerMsg("Remove kubeconfig from "+defaultKubeConfigPath))
	defer s.Fail()
	if err := kubeConfigRemove(o.prevCluster.KubeConfig, defaultKubeConfigPath); err != nil {
		if os.IsNotExist(err) {
			s.Success()
			return nil
		} else {
			return err
		}
	}
	s.Success()

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
	s := spinner.New(o.Out, spinnerMsg("Remove state file %s", o.stateFilePath))
	defer s.Fail()
	if err := removeStateFile(o.stateFilePath); err != nil {
		return err
	}
	s.Success()
	return nil
}
