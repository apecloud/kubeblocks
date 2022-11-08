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
	"io"

	"github.com/spf13/cobra"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/prompt"
)

// Build a delete command
func Build(c *builder.Command) *cobra.Command {
	deleteFlags := newDeleteCommandFlags()
	cmd := &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Example: c.Example,
		Run: func(cmd *cobra.Command, args []string) {
			o, err := deleteFlags.ToOptions(nil, c.IOStreams)
			cmdutil.CheckErr(err)
			cmdutil.CheckErr(validate(args, c.IOStreams.In))

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

func validate(args []string, in io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("missing name")
	}

	// confirm the name
	name, err := prompt.NewPrompt("You should enter the name", "Please enter the name again:", in).GetInput()
	if err != nil {
		return err
	}
	if name != args[0] {
		return fmt.Errorf("the entered name \"%s\" does not match \"%s\"", name, args[0])
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
