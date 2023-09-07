package cluster

import (
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type pullOption struct {
	clusterType cluster.ClusterType
	url         string
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	//
}
