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
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const ComponentClassTemplate = `
- # component resource constraint name, such as general, memory-optimized, cpu-optimized etc.
  resourceConstraintRef: kb-resource-constraint-general
  # class schema template, you can set default resource values here
  template: |
    cpu: "{{ or .cpu 1 }}"
    memory: "{{ or .memory 4 }}Gi"
    storage:
    - name: data
      size: "{{ or .dataStorageSize 10 }}Gi"
    - name: log
      size: "{{ or .logStorageSize 1 }}Gi"
  # class schema template variables
  vars: [cpu, memory, dataStorageSize, logStorageSize]
  series:
  - # class name generator, you can reference variables in class schema template
    # it's also ok to define static class name in following class definitions
    name: "custom-{{ .cpu }}c{{ .memory }}g"

    # class definitions, we support two kinds of class definitions:
    # 1. define arguments for class schema variables, class schema will be dynamically generated
    # 2. statically define complete class schema
    classes:
    # arguments for dynamically generated class
    - args: [1, 1, 100, 10]
`

type TemplateOptions struct {
	genericclioptions.IOStreams

	outputFile string
}

func NewTemplateCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &TemplateOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Generate class definition template",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.run())
		},
	}
	cmd.Flags().StringVarP(&o.outputFile, "output", "o", "", "Output class definition template to a file")
	return cmd
}

func (o *TemplateOptions) run() error {
	if o.outputFile != "" {
		return os.WriteFile(o.outputFile, []byte(ComponentClassTemplate), 0644)
	}

	_, err := fmt.Fprint(o.Out, ComponentClassTemplate)
	return err
}
