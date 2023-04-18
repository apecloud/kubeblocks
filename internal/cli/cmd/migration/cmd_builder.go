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

package migration

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

// NewMigrationCmd creates the cluster command
func NewMigrationCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migration",
		Short: "Data migration between two data sources.",
	}

	groups := templates.CommandGroups{
		{
			Message: "Basic Migration Commands:",
			Commands: []*cobra.Command{
				NewMigrationCreateCmd(f, streams),
				NewMigrationTemplatesCmd(f, streams),
				NewMigrationListCmd(f, streams),
				NewMigrationTerminateCmd(f, streams),
			},
		},
		{
			Message: "Migration Operation Commands:",
			Commands: []*cobra.Command{
				NewMigrationDescribeCmd(f, streams),
				NewMigrationLogsCmd(f, streams),
			},
		},
	}

	// add subcommands
	groups.Add(cmd)
	templates.ActsAsRootCommand(cmd, nil, groups...)

	return cmd
}
