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
	"strings"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/get"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
)

// Build return a list command
func Build(c *builder.Command) *cobra.Command {
	o := get.NewOptions(c.IOStreams, []string{util.GVRToString(c.GVR)})

	use := c.Use
	if len(use) == 0 {
		use = "list"
	}

	cmd := &cobra.Command{
		Use:     use,
		Short:   c.Short,
		Example: c.Example,
		Run: func(cmd *cobra.Command, args []string) {
			complete(o, cmd)
			if c.CustomComplete != nil {
				cmdutil.CheckErr(c.CustomComplete(o, args))
			}
			cmdutil.CheckErr(o.Complete(c.Factory))
			cmdutil.CheckErr(o.Run(c.Factory))
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)
	return cmd
}

func complete(o *get.Options, cmd *cobra.Command) {
	o.NoHeaders = cmdutil.GetFlagBool(cmd, "no-headers")
	outputOption := cmd.Flags().Lookup("output").Value.String()
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
}
