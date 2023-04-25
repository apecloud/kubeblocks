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

package class

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
)

type ListOptions struct {
	ClusterDefRef string
	Factory       cmdutil.Factory
	dynamic       dynamic.Interface
	genericclioptions.IOStreams
}

var listClassExamples = templates.Examples(`
    # List all components classes in cluster definition apecloud-mysql
    kbcli class list --cluster-definition apecloud-mysql
`)

func NewListCommand(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List classes",
		Example: listClassExamples,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	flags.AddClusterDefinitionFlag(f, cmd, &o.ClusterDefRef)
	util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))
	return cmd
}

func (o *ListOptions) complete(f cmdutil.Factory) error {
	var err error
	o.dynamic, err = f.DynamicClient()
	return err
}

func (o *ListOptions) run() error {
	clsMgr, err := class.GetManager(o.dynamic, o.ClusterDefRef)
	if err != nil {
		return err
	}
	constraintClassMap := make(map[string]map[string][]*class.ComponentClassWithRef)
	for compName, items := range clsMgr.GetClasses() {
		for _, item := range items {
			if _, ok := constraintClassMap[item.ResourceConstraintRef]; !ok {
				constraintClassMap[item.ResourceConstraintRef] = make(map[string][]*class.ComponentClassWithRef)
			}
			constraintClassMap[item.ResourceConstraintRef][compName] = append(constraintClassMap[item.ResourceConstraintRef][compName], item)
		}
	}
	var constraintNames []string
	for name := range constraintClassMap {
		constraintNames = append(constraintNames, name)
	}
	sort.Strings(constraintNames)
	for _, constraintName := range constraintNames {
		for compName, classes := range constraintClassMap[constraintName] {
			o.printClass(constraintName, compName, classes)
		}
		_, _ = fmt.Fprint(o.Out, "\n")
	}
	return nil
}

func (o *ListOptions) printClass(constraintName string, compName string, classes []*class.ComponentClassWithRef) {
	tbl := printer.NewTablePrinter(o.Out)
	_, _ = fmt.Fprintf(o.Out, "\nConstraint %s:\n", constraintName)
	tbl.SetHeader("COMPONENT", "CLASS", "CPU", "MEMORY")
	sort.Sort(class.ByClassResource(classes))
	for _, cls := range classes {
		tbl.AddRow(compName, cls.Name, cls.CPU.String(), normalizeMemory(cls.Memory))
	}
	tbl.Print()
}

func normalizeMemory(mem resource.Quantity) string {
	if !strings.HasSuffix(mem.String(), "m") {
		return mem.String()
	}

	var (
		value  float64
		suffix string
		bytes  = float64(mem.MilliValue()) / 1000
	)
	switch {
	case bytes < 1024:
		value = bytes / 1024
		suffix = "Ki"
	case bytes < 1024*1024:
		value = bytes / 1024 / 1024
		suffix = "Mi"
	case bytes < 1024*1024*1024:
		value = bytes / 1024 / 1024 / 1024
		suffix = "Gi"
	default:
		value = bytes / 1024 / 1024 / 1024 / 1024
		suffix = "Ti"
	}
	return strings.TrimRight(fmt.Sprintf("%.3f", value), "0") + suffix
}
