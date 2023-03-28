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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

var (
	destroyExample = templates.Examples(`
		# destroy local host playground cluster
		kbcli playground destroy

		# destroy the AWS EKS cluster, the region is required
		kbcli playground destroy --cloud-provider aws --region cn-northwest-1`)
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
	provider := cp.NewLocalCloudProvider(o.Out, o.ErrOut)
	spinner := printer.Spinner(o.Out, "Delete playground k3d cluster %s", o.prevCluster.ClusterName)
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
	cpPath, err := cloudProviderRepoDir()
	if err != nil {
		return err
	}

	provider, err := cp.New(o.prevCluster.CloudProvider, cpPath, o.Out, o.ErrOut)
	if err != nil {
		return err
	}

	// start to destroy cluster
	printer.Warning(o.Out, `This action will directly delete the kubernetes cluster, which may
  result in some residual resources, such as Volume, please confirm and manually
  clean up related resources after this action.

  In order to minimize resource residue, you can use the following commands
  to clean up the clusters and uninstall KubeBlocks before this action.
  
  # list all clusters created by KubeBlocks
  kbcli cluster list -A

  # delete clusters
  kbcli cluster delete <cluster-1> <cluster-2>

  # uninstall KubeBlocks and remove PVC and PV
  kbcli kubeblocks uninstall --remove-pvcs --remove-pvs

`)

	fmt.Fprintf(o.Out, "Do you really want to destroy the kubernetes cluster %s?\n  This is no undo. Only 'yes' will be accepted to confirm.\n\n", o.prevCluster.ClusterName)

	// confirm to destroy
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground destroy cancelled.\n")
		return cmdutil.ErrExit
	}

	o.startTime = time.Now()

	fmt.Fprintf(o.Out, "Destroy %s %s cluster %s...\n",
		o.prevCluster.CloudProvider, cp.K8sService(o.prevCluster.CloudProvider), o.prevCluster.ClusterName)
	if err = provider.DeleteK8sCluster(o.prevCluster); err != nil {
		return err
	}

	if err = o.removeKubeConfig(); err != nil {
		return err
	}

	if err = o.removeStateFile(); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "\nPlayground destroy completed in %s.\n", time.Since(o.startTime).Truncate(time.Second))
	return nil
}

func (o *destroyOptions) removeKubeConfig() error {
	configPath := util.ConfigPath("config")
	spinner := printer.Spinner(o.Out, "Remove kubeconfig from %s", configPath)
	defer spinner(false)
	if err := kubeConfigRemove(o.prevCluster.KubeConfig, configPath); err != nil {
		return err
	}
	spinner(true)
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
