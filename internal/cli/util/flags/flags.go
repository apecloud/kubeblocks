package flags

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// AddClusterDefinitionFlag add a flag "cluster-definition" for the cmd and store the value of the flag
// in string p
func AddClusterDefinitionFlag(f cmdutil.Factory, cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "cluster-definition", *p, "list the resources related to the specified cluster definition")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}
