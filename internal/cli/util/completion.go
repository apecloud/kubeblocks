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

package util

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
)

func ResourceNameCompletionFunc(f cmdutil.Factory, gvr schema.GroupVersionResource) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		comps := utilcomp.CompGetResource(f, cmd, GVRToString(gvr), toComplete)
		seen := make(map[string]bool)

		var availableComps []string
		for _, arg := range args {
			seen[arg] = true
		}
		for _, comp := range comps {
			if !seen[comp] {
				availableComps = append(availableComps, comp)
			}
		}
		return availableComps, cobra.ShellCompDirectiveNoFileComp
	}
}

// CompGetResourceWithLabels gets the list of the resource specified which begin with `toComplete` and have the specified labels.
func CompGetResourceWithLabels(f cmdutil.Factory, cmd *cobra.Command, resourceName string, labels []string, toComplete string) []string {
	template := "{{ range .items  }}{{ .metadata.name }} {{ end }}"
	return CompGetFromTemplate(&template, f, "", cmd, []string{resourceName}, labels, toComplete)
}

// CompGetFromTemplate executes a Get operation using the specified template and args and returns the results
// which begin with `toComplete` and have the specified labels.
func CompGetFromTemplate(template *string, f cmdutil.Factory, namespace string, cmd *cobra.Command, args []string, labels []string, toComplete string) []string {
	buf := new(bytes.Buffer)
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: buf, ErrOut: ioutil.Discard}
	o := get.NewGetOptions("kubectl", streams)

	// Get the list of names of the specified resource
	o.PrintFlags.TemplateFlags.GoTemplatePrintFlags.TemplateArgument = template
	format := "go-template"
	o.PrintFlags.OutputFormat = &format

	// Do the steps Complete() would have done.
	// We cannot actually call Complete() or Validate() as these function check for
	// the presence of flags, which, in our case won't be there
	if namespace != "" {
		o.Namespace = namespace
		o.ExplicitNamespace = true
	} else {
		var err error
		o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return nil
		}
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, outputObjects *bool, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) {
		printer, err := o.PrintFlags.ToPrinter()
		if err != nil {
			return nil, err
		}
		return printer.PrintObj, nil
	}

	if len(labels) > 0 {
		o.LabelSelector = strings.Join(labels, ",")
	}

	_ = o.Run(f, cmd, args)

	var comps []string
	resources := strings.Split(buf.String(), " ")
	for _, res := range resources {
		if res != "" && strings.HasPrefix(res, toComplete) {
			comps = append(comps, res)
		}
	}
	return comps
}
