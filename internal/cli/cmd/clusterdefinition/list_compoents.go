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
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
			o.FieldSelector = fmt.Sprintf("metadata.name=%s", args[0])
			util.CheckErr(run(o, args[0]))
		},
	}
	return cmd
}

func validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing clusterdefinition name")
	} else if len(args) > 1 {
		return fmt.Errorf("only support one clusterdefinition name")
	}
	return nil
}

func run(o *list.ListOptions, resource string) error {
	o.Print = false
	var allErrs []error
	r, err := o.Run()
	if err != nil {
		allErrs = append(allErrs, err)
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if len(infos) == 0 {
		o.PrintNotFoundResources()
		return utilerrors.NewAggregate(allErrs)
	}
	p := printer.NewTablePrinter(o.Out)
	p.SetHeader("NAME", "WORKLOAD-TYPE", "CHARACTER-TYPE")
	for _, info := range infos {
		var cd v1alpha1.ClusterDefinition
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cd); err != nil {
			allErrs = append(allErrs, err)
		}
		for _, comp := range cd.Spec.ComponentDefs {
			p.AddRow(comp.Name, comp.WorkloadType, comp.CharacterType)
		}
	}
	p.Print()
	return utilerrors.NewAggregate(allErrs)
}
