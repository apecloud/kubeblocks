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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listComponentsExample = templates.Examples(`
		# List all components belong to the cluster definition.
		kbcli clusterdefinition list-components apecloud-mysql`)
)

func NewListComponentsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterDefGVR())
	o.AllNamespaces = true
	cmd := &cobra.Command{
		Use:               "list-components",
		Short:             "List cluster definition components.",
		Example:           listComponentsExample,
		Aliases:           []string{"ls-comps"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(validate(args))
			o.Names = args
			util.CheckErr(run(o))
		},
	}
	return cmd
}

func validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing clusterdefinition name")
	}
	return nil
}

func run(o *list.ListOptions) error {
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
	p.SetHeader("NAME", "WORKLOAD-TYPE", "CHARACTER-TYPE", "CLUSTER-DEFINITION", "IS-MAIN")
	p.SortBy(4, 1)
	for _, info := range infos {
		var cd v1alpha1.ClusterDefinition
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cd); err != nil {
			return err
		}
		for i, comp := range cd.Spec.ComponentDefs {
			if i == 0 {
				p.AddRow(printer.BoldGreen(comp.Name), comp.WorkloadType, comp.CharacterType, cd.Name, "true")
			} else {
				p.AddRow(comp.Name, comp.WorkloadType, comp.CharacterType, cd.Name, "false")
			}

		}
	}
	p.Print()
	return nil
}
