package migration

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/spf13/cobra"
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
