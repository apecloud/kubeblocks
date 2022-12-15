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
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

type DeleteFlags struct {
	*cmddelete.DeleteFlags

	// ClusterName only used when delete resources not cluster, ClusterName
	// is the owner of the resources, it will be used to construct a label
	// selector to filter the resource. If ClusterName is empty, command will
	// delete resources according to the ResourceNames without any label selector.
	ClusterName string

	// ResourceNames the resource names that will be deleted, if it is empty,
	// and ClusterName is specified, use label selector to delete all resource
	// belonging to the ClusterName
	ResourceNames []string
}

// Build a delete command
func Build(c *builder.Command) *cobra.Command {
	deleteFlags := newDeleteCommandFlags()
	cmd := &cobra.Command{
		Use:               c.Use,
		Short:             c.Short,
		Example:           c.Example,
		ValidArgsFunction: utilcomp.ResourceNameCompletionFunc(c.Factory, util.GVRToString(c.GVR)),
		Run: func(cmd *cobra.Command, args []string) {
			// If delete resources belonging to cluster, custom complete function
			// should fill the ResourceName or construct the label selector based
			// on the ClusterName
			if c.CustomComplete != nil {
				util.CheckErr(c.CustomComplete(c))
			}

			util.CheckErr(validate(deleteFlags, args, c.IOStreams.In))

			o, err := deleteFlags.ToOptions(nil, c.IOStreams)
			util.CheckErr(err)

			// build args that will be used to
			args = buildArgs(c, deleteFlags, args)

			// call kubectl delete options methods
			util.CheckErr(o.Complete(c.Factory, args, cmd))
			util.CheckErr(o.Validate())
			util.CheckErr(o.RunDelete(c.Factory))
		},
	}

	c.Options = deleteFlags
	c.Cmd = cmd
	if c.CustomFlags != nil {
		c.CustomFlags(c)
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
		// use the cluster label selector to select the resources that should
		// be deleted, so args should be empty, the original args should have
		// been used to construct the label selector.
		args = []string{}
	}
	args = append([]string{util.GVRToString(c.GVR)}, args...)
	return args
}

func validate(deleteFlags *DeleteFlags, args []string, in io.Reader) error {
	// build resource to delete.
	// if resource names is specified, use it first, otherwise use the args.
	if len(deleteFlags.ResourceNames) > 0 {
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
