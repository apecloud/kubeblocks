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

package cluster

import (
	"fmt"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/list"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

var listExample = templates.Examples(`
		# list all clusters
		dbctl cluster list

		# list a single cluster with specified NAME
		dbctl cluster list my-cluster

		# list a single cluster in YAML output format
		dbctl cluster list my-cluster -o yaml

		# list a single cluster in JSON output format
		dbctl cluster list my-cluster -o json

		# list a single cluster in wide output format
		dbctl cluster list my-cluster -o wide	

		# list all instances of all clusters
		dbctl cluster list --show-instance

		# list all instances of a specified cluster
		dbctl cluster list my-cluster --show-instance

		# list all components of all clusters
		dbctl cluster list --show-component

		# list all components of a specified cluster
		dbctl cluster list my-cluster --show-component`)

type listOptions struct {
	// showInstance if true, list instance info
	showInstance bool

	// showComponent if true, list component info
	showComponent bool
}

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Short("List all cluster.").
		Example(listExample).
		Options(&listOptions{}).
		Factory(f).
		GVR(types.ClusterGVR()).
		IOStreams(streams).
		CustomFlags(customFlags).
		CustomRun(customRun).
		Build(list.Build)
}

func customFlags(options builder.Options, cmd *cobra.Command) {
	o := options.(*listOptions)

	cmd.Flags().BoolVar(&o.showInstance, "show-instance", false, "Show instance info")
	cmd.Flags().BoolVar(&o.showComponent, "show-component", false, "Show component info")
}

// If show-instance, show-component or -o wide is set, output corresponding information,
// if these flags are set on the same time, only one is valid, their priority order is
// show-instance, show-component and -o wide.
func customRun(c *builder.Command) (bool, error) {
	var printer cluster.Printer

	o := c.Options.(*listOptions)
	output := c.Cmd.Flags().Lookup("output").Value.String()
	outputWide := output == "wide"
	if !o.showInstance && !o.showComponent && !outputWide {
		return true, nil
	}

	dynamic, err := c.Factory.DynamicClient()
	if err != nil {
		return false, err
	}

	client, err := c.Factory.KubernetesClientSet()
	if err != nil {
		return false, err
	}

	namespace, _, err := c.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return false, err
	}

	switch {
	case o.showInstance:
		printer = &cluster.InstancePrinter{Tbl: uitable.New()}
	case o.showComponent:
		printer = &cluster.ComponentPrinter{Tbl: uitable.New()}
	case outputWide:
		printer = &cluster.ClusterPrinter{Tbl: uitable.New()}
	}

	if printer != nil {
		return false, show(dynamic, client, namespace, c.Args, c.IOStreams, printer)
	}
	return true, nil
}

func show(d dynamic.Interface, client *kubernetes.Clientset, namespace string, names []string,
	streams genericclioptions.IOStreams, printer cluster.Printer) error {

	// add output table header
	printer.AddHeader()

	// cluster names are specified by command args
	for _, name := range names {
		if err := addRow(d, client, namespace, name, printer); err != nil {
			return err
		}
	}

	if len(names) > 0 {
		return printer.Print(streams.Out)
	}

	// do not specify any cluster name, we will get all clusters
	clusters := &dbaasv1alpha1.ClusterList{}
	if err := cluster.GetAllCluster(d, namespace, clusters); err != nil {
		return err
	}

	// no clusters found
	if len(clusters.Items) == 0 {
		fmt.Fprintln(streams.ErrOut, "No resources found")
		return nil
	}

	for _, c := range clusters.Items {
		if err := addRow(d, client, namespace, c.Name, printer); err != nil {
			return err
		}
	}
	return printer.Print(streams.Out)
}

func addRow(d dynamic.Interface, client *kubernetes.Clientset, namespace string, name string, printer cluster.Printer) error {
	clusterObjs := cluster.NewClusterObjects()
	getter := &cluster.ObjectsGetter{
		Name:          name,
		Namespace:     namespace,
		ClientSet:     client,
		DynamicClient: d,
	}

	if err := getter.Get(clusterObjs); err != nil {
		return err
	}

	printer.AddRow(clusterObjs)
	return nil
}
