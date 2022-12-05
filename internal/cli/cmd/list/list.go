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

package list

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/get"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/builder"
)

// Build return a list command, if the resource is not cluster, construct a label
// selector based on cluster name to select resource to list.
func Build(c *builder.Command) *cobra.Command {
	o := get.NewOptions(c.IOStreams, []string{util.GVRToString(c.GVR)})

	use := c.Use
	var alias string
	if len(use) == 0 {
		use = "list"
		alias = "ls"
	}

	cmd := &cobra.Command{
		Use:     use,
		Short:   c.Short,
		Example: c.Example,
		Aliases: []string{alias},
		Run: func(cmd *cobra.Command, args []string) {
			var (
				goon = true
				err  error
			)
			c.Args = args
			c.Cmd = cmd

			complete(c, o)
			if c.CustomComplete != nil {
				util.CheckErr(c.CustomComplete(o, args))
			}
			util.CheckErr(o.Complete(c.Factory))

			if c.CustomRun != nil {
				goon, err = c.CustomRun(c)
			}
			if goon && err == nil {
				util.CheckErr(o.Run(c.Factory))
			}
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)

	if c.CustomFlags != nil {
		c.CustomFlags(c.Options, cmd)
	}
	return cmd
}

func complete(c *builder.Command, o *get.Options) {
	o.NoHeaders = cmdutil.GetFlagBool(c.Cmd, "no-headers")
	outputOption := c.Cmd.Flags().Lookup("output").Value.String()
	if strings.Contains(outputOption, "custom-columns") || outputOption == "yaml" || strings.Contains(outputOption, "json") {
		o.ServerPrint = false
	}

	templateArg := ""
	if o.PrintFlags.TemplateFlags != nil && o.PrintFlags.TemplateFlags.TemplateArgument != nil {
		templateArg = *o.PrintFlags.TemplateFlags.TemplateArgument
	}

	if (len(*o.PrintFlags.OutputFormat) == 0 && len(templateArg) == 0) || *o.PrintFlags.OutputFormat == "wide" {
		o.IsHumanReadablePrinter = true
	}

	buildListArgs(c, o)
}

// buildListArgs build resource to list, if Resource is not Cluster, use cluster name to
// construct label selector.
func buildListArgs(c *builder.Command, o *get.Options) {
	switch c.GVR {
	case types.ClusterGVR():
		// args are the cluster names
		o.BuildArgs = append(o.BuildArgs, c.Args...)
	default:
		// for other resources, use cluster name to construct the label selector,
		// the label selector is like "instance-key in (cluster1, cluster2)"
		if len(c.Args) == 0 {
			return
		}

		label := fmt.Sprintf("%s in (%s)", types.InstanceLabelKey, strings.Join(c.Args, ","))
		if len(o.LabelSelector) == 0 {
			o.LabelSelector = label
		} else {
			o.LabelSelector += "," + label
		}
	}
}
