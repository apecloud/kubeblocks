/*
Copyright ApeCloud Inc.

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

package delete

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// Command used to build a delete command
type Command struct {
	Use       string
	Short     string
	Example   string
	GroupKind schema.GroupKind
	Factory   cmdutil.Factory
	genericclioptions.IOStreams
}

// Build a delete command
func (c *Command) Build() *cobra.Command {
	deleteFlags := newDeleteCommandFlags()
	cmd := &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Example: c.Example,
		Run: func(cmd *cobra.Command, args []string) {
			o, err := deleteFlags.ToOptions(nil, c.IOStreams)
			cmdutil.CheckErr(err)
			cmdutil.CheckErr(c.validate(args))

			// build resource to delete
			args = append([]string{c.GroupKind.String()}, args...)

			// call kubectl delete options methods
			cmdutil.CheckErr(o.Complete(c.Factory, args, cmd))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunDelete(c.Factory))
		},
	}

	deleteFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (c *Command) validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing name")
	}
	return nil
}

// newDeleteCommandFlags return a kubectl delete command flags, disable some flags that
// we do not supported.
func newDeleteCommandFlags() *cmddelete.DeleteFlags {
	deleteFlags := cmddelete.NewDeleteCommandFlags("containing the resource to delete.")

	// disable some flags
	deleteFlags.FieldSelector = nil
	deleteFlags.Raw = nil
	deleteFlags.All = nil
	deleteFlags.IgnoreNotFound = nil
	deleteFlags.FileNameFlags = nil

	return deleteFlags
}
