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

	cmd.Flags().StringVar(&o.cloudProvider, "cloud-provider", defaultCloudProvider, fmt.Sprintf("Cloud provider type, one of [%s]", strings.Join(cp.CloudProviders(), ",")))
	cmd.Flags().StringVar(&o.region, "region", "", "The region to create kubernetes cluster")
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
	// do not find any existed cluster
	if name == "" {
		fmt.Fprintf(o.Out, "Failed to find playground %s %s cluster in %s\n", o.cloudProvider, cp.K8sService(o.cloudProvider), cpPath)
		if err != nil {
			fmt.Fprintf(o.Out, "  error: %s", err.Error())
		}
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

	fmt.Fprintf(o.Out, "Do you really want to destroy the kubernetes cluster %s?\n  This is no undo. Only 'yes' will be accepted to confirm.\n\n", name)

	// confirm to destroy
	entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
	if entered != yesStr {
		fmt.Fprintf(o.Out, "\nPlayground destroy cancelled.\n")
		return cmdutil.ErrExit
	}

	o.startTime = time.Now()
	fmt.Fprintf(o.Out, "Destroy %s %s cluster %s...\n", o.cloudProvider, cp.K8sService(o.cloudProvider), name)
	if err = provider.DeleteK8sCluster(name); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Playground destroy completed in %s.\n", time.Since(o.startTime))

	return nil
}
