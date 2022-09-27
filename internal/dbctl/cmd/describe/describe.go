/*
Copyright 2022.

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

package describe

import (
	"embed"
	"fmt"

	"github.com/leaanthony/debme"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/get"
)

var (
	//go:embed template/*
	template embed.FS
)

type PrintExtra func() error

// Command used to construct a describe command
type Command struct {
	Factory cmdutil.Factory
	Short   string

	GroupKind  []schema.GroupKind
	Template   []string
	Name       string
	PrintExtra PrintExtra

	Streams genericclioptions.IOStreams
}

// Build return a describe command
func (c *Command) Build() *cobra.Command {
	o := get.NewOptions(c.Streams, []string{})

	cmd := &cobra.Command{
		Use:   "describe",
		Short: c.Short,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(c.complete(args))
			cmdutil.CheckErr(c.run(o, c.Factory))
		},
	}

	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)
	return cmd
}

func (c *Command) complete(args []string) error {
	if len(args) == 0 {
		return errors.New("You must specify the name of resource to describe.")
	}

	if len(c.GroupKind) == 0 {
		return errors.New("You must specify the resource type to describe.")
	}

	if len(c.GroupKind) != len(c.Template) {
		return errors.New("The number of resource type is not equal to template.")
	}
	c.Name = args[0]
	return nil
}

func (c *Command) run(o *get.Options, f cmdutil.Factory) error {
	if err := o.Complete(f); err != nil {
		return err
	}

	// Get object from k8s and render the template
	tmplFs, _ := debme.FS(template, "template")
	for i := 0; i < len(c.GroupKind); i++ {
		tmplBytes, err := tmplFs.ReadFile(c.Template[i])
		if err != nil {
			fmt.Fprintln(o.ErrOut, "build describe command error")
			return nil
		}
		tmplStr := fmt.Sprintf("go-template=%s", string(tmplBytes))
		o.PrintFlags.OutputFormat = &tmplStr
		o.BuildArgs = []string{c.GroupKind[i].String(), c.Name}

		if err = o.Run(f); err != nil {
			return err
		}
	}

	// execute the custom print function to print other infos
	if c.PrintExtra != nil {
		err := c.PrintExtra()
		if err != nil {
			return err
		}
	}

	return nil
}
