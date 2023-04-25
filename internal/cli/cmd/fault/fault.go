package fault

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "inject fault.",
	}
	cmd.AddCommand(
		NewFaultPodCmd(f, streams),
	)
	return cmd
}
