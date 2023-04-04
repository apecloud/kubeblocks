package migration

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/spf13/cobra"
)

func NewMigrationTerminateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.MigrationTaskGVR())
	cmd := &cobra.Command{
		Use:               "terminate NAME",
		Short:             "Delete migration task.",
		Example:           MigrationDeleteExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.MigrationTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			_, validErr := IsMigrationCrdValidWithFactory(&o.Factory)
			util.CheckErr(validErr)
			util.CheckErr(deleteMigrationTask(o, args))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func deleteMigrationTask(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing migration task name")
	}
	o.Names = args
	return o.Run()
}
