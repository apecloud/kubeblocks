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
	"strings"

	"github.com/spf13/cobra"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/prompt"
)

type DeleteFlags struct {
	*cmddelete.DeleteFlags
	// cluster name. if the resources is not owns by cluster, ignore it
	ClusterName string
	// the resource names of cluster need to be deleted
	ResourceNames []string
}

// Build a delete command
func Build(c *builder.Command) *cobra.Command {
	deleteFlags := newDeleteCommandFlags()
	cmd := &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Example: c.Example,
		Run: func(cmd *cobra.Command, args []string) {
			if c.CustomComplete != nil {
				cmdutil.CheckErr(c.CustomComplete(deleteFlags, args))
			}
			cmdutil.CheckErr(validate(deleteFlags, args, c.IOStreams.In))
			o, err := deleteFlags.ToOptions(nil, c.IOStreams)
			cmdutil.CheckErr(err)
			args = buildArgs(c, deleteFlags, args)
			// call kubectl delete options methods
			cmdutil.CheckErr(o.Complete(c.Factory, args, cmd))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunDelete(c.Factory))
		},
	}
	if c.CustomFlags != nil {
		c.CustomFlags(deleteFlags, cmd)
	}
	deleteFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

// buildArgs build resource to delete
func buildArgs(c *builder.Command, deleteFlags *DeleteFlags, args []string) []string {
	if len(deleteFlags.ResourceNames) > 0 {
		args = deleteFlags.ResourceNames
	} else if deleteFlags.ClusterName != "" {
		// using the cluster label selector, args should be empty
		args = []string{}
	}
	args = append([]string{util.GVRToString(c.GVR)}, args...)
	return args
}

func validate(deleteFlags *DeleteFlags, args []string, in io.Reader) error {
	// build resource to delete.
	// if resource names is specified, use it, otherwise use the args.
	if deleteFlags.ResourceNames != nil && len(deleteFlags.ResourceNames) > 0 {
		args = deleteFlags.ResourceNames
	}
	if len(args) < 1 {
		return fmt.Errorf("missing name")
	}

	// confirm the name
	name, err := prompt.NewPrompt("You should enter the name.", "Please enter the name again(separate with commas when more than one):", in).GetInput()
	if err != nil {
		return err
	}
	if name != strings.Join(args, ",") {
		return fmt.Errorf("the entered name \"%s\" does not match \"%s\"", name, strings.Join(args, ","))
	}

	return nil
}

// newDeleteCommandFlags return a kubectl delete command flags, disable some flags that
// we do not supported.
func newDeleteCommandFlags() *DeleteFlags {
	deleteCmdFlags := cmddelete.NewDeleteCommandFlags("containing the resource to delete.")

	// disable some flags
	deleteCmdFlags.FieldSelector = nil
	deleteCmdFlags.Raw = nil
	deleteCmdFlags.All = nil
	deleteCmdFlags.IgnoreNotFound = nil
	deleteCmdFlags.FileNameFlags = nil

	return &DeleteFlags{DeleteFlags: deleteCmdFlags}
}
