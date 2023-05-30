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
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const ComponentClassTemplate = `
- resourceConstraintRef: kb-resource-constraint-general
  # class template, you can declare variables and set default values here
  template: |
    cpu: "{{ or .cpu 1 }}"
    memory: "{{ or .memory 4 }}Gi"
  # template variables used to define classes
  vars: [cpu, memory]
  series:
  - # class naming template, you can reference variables in class template
    # it's also ok to define static class name in the following class definitions
    namingTemplate: "custom-{{ .cpu }}c{{ .memory }}g"

    # class definitions, we support two kinds of class definitions:
    # 1. define values for template variables and the full class definition will be dynamically rendered
    # 2. statically define the complete class
    classes:
    - args: [1, 1]
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
