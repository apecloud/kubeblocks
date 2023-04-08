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

package class

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli cluster-definition list\" to show all available cluster definition")
	util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))
	return cmd
}

func (o *ListOptions) complete(f cmdutil.Factory) error {
	var err error
	o.dynamic, err = f.DynamicClient()
	return err
}

func (o *ListOptions) run() error {
	componentClasses, err := class.GetClasses(o.dynamic, o.ClusterDefRef)
	if err != nil {
		return err
	}
	familyClassMap := make(map[string]map[string][]*class.ComponentClassInstance)
	for compName, items := range componentClasses {
		for _, item := range items {
			if _, ok := familyClassMap[item.Family]; !ok {
				familyClassMap[item.Family] = make(map[string][]*class.ComponentClassInstance)
			}
			familyClassMap[item.Family][compName] = append(familyClassMap[item.Family][compName], item)
		}
	}
	var familyNames []string
	for name := range familyClassMap {
		familyNames = append(familyNames, name)
	}
	sort.Strings(familyNames)
	for _, family := range familyNames {
		for compName, classes := range familyClassMap[family] {
			o.printClassFamily(family, compName, classes)
		}
		_, _ = fmt.Fprint(o.Out, "\n")
	}
	return nil
}

func (o *ListOptions) printClassFamily(family string, compName string, classes []*class.ComponentClassInstance) {
	tbl := printer.NewTablePrinter(o.Out)
	_, _ = fmt.Fprintf(o.Out, "\nFamily %s:\n", family)
	tbl.SetHeader("COMPONENT", "CLASS", "CPU", "MEMORY", "STORAGE")
	sort.Sort(class.ByClassCPUAndMemory(classes))
	for _, class := range classes {
		var disks []string
		for _, disk := range class.Storage {
			disks = append(disks, disk.String())
		}
		tbl.AddRow(compName, class.Name, class.CPU.String(), class.Memory.String(), strings.Join(disks, ","))
	}
	tbl.Print()
}
