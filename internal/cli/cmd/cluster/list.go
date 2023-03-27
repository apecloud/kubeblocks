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

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listExample = templates.Examples(`
		# list all clusters
		kbcli cluster list

		# list a single cluster with specified NAME
		kbcli cluster list mycluster

		# list a single cluster in YAML output format
		kbcli cluster list mycluster -o yaml

		# list a single cluster in JSON output format
		kbcli cluster list mycluster -o json

		# list a single cluster in wide output format
		kbcli cluster list mycluster -o wide`)

	listInstancesExample = templates.Examples(`
		# list all instances of all clusters in current namespace
		kbcli cluster list-instances

		# list all instances of a specified cluster
		kbcli cluster list-instances mycluster`)

	listComponentsExample = templates.Examples(`
		# list all components of all clusters in current namespace
		kbcli cluster list-components

		# list all components of a specified cluster
		kbcli cluster list-components mycluster`)

	listEventsExample = templates.Examples(`
		# list all events of all clusters in current namespace
		kbcli cluster list-events

		# list all events of a specified cluster
		kbcli cluster list-events mycluster`)
)

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterGVR())
	cmd := &cobra.Command{
		Use:               "list [NAME]",
		Short:             "List clusters.",
		Example:           listExample,
		Aliases:           []string{"ls"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			if o.Format == printer.Wide {
				util.CheckErr(run(o, cluster.PrintWide))
			} else {
				util.CheckErr(run(o, cluster.PrintClusters))
			}
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewListInstancesCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterGVR())
	cmd := &cobra.Command{
		Use:               "list-instances",
		Short:             "List cluster instances.",
		Example:           listInstancesExample,
		Aliases:           []string{"ls-instances"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			util.CheckErr(run(o, cluster.PrintInstances))
		},
	}
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	return cmd
}

func NewListComponentsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterGVR())
	cmd := &cobra.Command{
		Use:               "list-components",
		Short:             "List cluster components.",
		Example:           listComponentsExample,
		Aliases:           []string{"ls-components"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			util.CheckErr(run(o, cluster.PrintComponents))
		},
	}
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	return cmd
}

func NewListEventsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterGVR())
	cmd := &cobra.Command{
		Use:               "list-events",
		Short:             "List cluster events.",
		Example:           listEventsExample,
		Aliases:           []string{"ls-events"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			o.Names = args
			util.CheckErr(run(o, cluster.PrintEvents))
		},
	}
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	return cmd
}

func run(o *list.ListOptions, printType cluster.PrintType) error {
	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		_, err := o.Run()
		return err
	}

	// get and output the result
	o.Print = false
	r, err := o.Run()
	if err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(o.IOStreams.Out, "No cluster found")
		return nil
	}

	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}

	client, err := o.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	p := cluster.NewPrinter(o.IOStreams.Out, printType)
	for _, info := range infos {
		if err = addRow(dynamic, client, info.Namespace, info.Name, p); err != nil {
			return err
		}
	}
	p.Print()
	return nil
}

func addRow(dynamic dynamic.Interface, client *kubernetes.Clientset,
	namespace string, name string, printer *cluster.Printer) error {
	getter := &cluster.ObjectsGetter{
		Name:       name,
		Namespace:  namespace,
		Client:     client,
		Dynamic:    dynamic,
		GetOptions: printer.GetterOptions(),
	}

	clusterObjs, err := getter.Get()
	if err != nil {
		return err
	}

	printer.AddRow(clusterObjs)
	return nil
}
