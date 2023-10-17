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

package clusterdefinition

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listServiceRefExample = templates.Examples(`
		# List cluster references name declared in a cluster definition.
		kbcli clusterdefinition list-service-reference apecloud-mysql`)
)

func NewListServiceReferenceCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterDefGVR())
	o.AllNamespaces = true
	cmd := &cobra.Command{
		Use:               "list-service-reference",
		Short:             "List cluster references declared in a cluster definition.",
		Example:           listServiceRefExample,
		Aliases:           []string{"ls-sr"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(validate(args))
			o.Names = args
			util.CheckErr(listServiceRef(o))
		},
	}
	return cmd
}

func listServiceRef(o *list.ListOptions) error {
	o.Print = false

	r, err := o.Run()
	if err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}

	p := printer.NewTablePrinter(o.Out)
	p.SetHeader("NAME", "COMPONENT", "SERVICE-KIND", "SERVICE-VERSION")
	p.SortBy(1)
	for _, info := range infos {
		var cd v1alpha1.ClusterDefinition
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cd); err != nil {
			return err
		}
		for _, comp := range cd.Spec.ComponentDefs {
			if comp.ServiceRefDeclarations == nil {
				continue
			}
			for _, serviceDec := range comp.ServiceRefDeclarations {
				for _, ref := range serviceDec.ServiceRefDeclarationSpecs {
					p.AddRow(serviceDec.Name, comp.Name, ref.ServiceKind, ref.ServiceVersion)
				}
			}
		}
	}

	if p.Tbl.Length() == 0 {
		fmt.Printf("No service references are declared in cluster definition %s", o.Names)
	} else {
		p.Print()
	}
	return nil
}
