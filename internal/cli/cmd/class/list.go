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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", "", "Specify cluster definition, run \"kbcli clusterdefinition list\" to show all available cluster definition")
	util.CheckErr(cmd.MarkFlagRequired("cluster-definition"))
	return cmd
}

func (o *ListOptions) complete(f cmdutil.Factory) error {
	var err error
	o.dynamic, err = f.DynamicClient()
	return err
}

func (o *ListOptions) run() error {
	componentClasses, err := class.ListClassesByClusterDefinition(o.dynamic, o.ClusterDefRef)
	if err != nil {
		return err
	}
	constraintClassMap := make(map[string]map[string][]*appsv1alpha1.ComponentClassInstance)
	for compName, items := range componentClasses {
		for _, item := range items {
			if _, ok := constraintClassMap[item.ResourceConstraintRef]; !ok {
				constraintClassMap[item.ResourceConstraintRef] = make(map[string][]*appsv1alpha1.ComponentClassInstance)
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

func (o *ListOptions) printClass(constraintName string, compName string, classes []*appsv1alpha1.ComponentClassInstance) {
	tbl := printer.NewTablePrinter(o.Out)
	_, _ = fmt.Fprintf(o.Out, "\nConstraint %s:\n", constraintName)
	tbl.SetHeader("COMPONENT", "CLASS", "CPU", "MEMORY", "STORAGE")
	sort.Sort(class.ByClassCPUAndMemory(classes))
	for _, cls := range classes {
		var volumes []string
		for _, volume := range cls.Volumes {
			volumes = append(volumes, fmt.Sprintf("name=%s,size=%s", volume.Name, volume.Size.String()))
		}
		tbl.AddRow(compName, cls.Name, cls.CPU.String(), cls.Memory.String(), strings.Join(volumes, ","))
	}
	tbl.Print()
}
