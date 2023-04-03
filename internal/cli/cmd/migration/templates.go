package migration

import (
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewMigrationTemplatesCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.MigrationTemplateGVR())
	cmd := &cobra.Command{
		Use:               "templates [NAME]",
		Short:             "List migration templates.",
		Example:           MigrationTemplateExample,
		Aliases:           []string{"tp", "template"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			_, validErr := IsMigrationCrdValidWithFactory(&o.Factory)
			util.CheckErr(validErr)
			o.Names = args
			_, err := o.Run()
			util.CheckErr(err)
		},
	}
	o.AddFlags(cmd)
	return cmd
}
