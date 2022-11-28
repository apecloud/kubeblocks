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

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/delete"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
)

func NewDeleteOpsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("delete-ops").
		Short("Delete a OpsRequest").
		GVR(types.OpsGVR()).
		Factory(f).
		IOStreams(streams).
		CustomComplete(completeForDeleteOps).
		CustomFlags(customFlagsForDeleteOps).
		Build(delete.Build)
}

func customFlagsForDeleteOps(option builder.Options, cmd *cobra.Command) {
	var (
		o  *delete.DeleteFlags
		ok bool
	)
	if o, ok = option.(*delete.DeleteFlags); !ok {
		return
	}
	cmd.Flags().StringSliceVar(&o.ResourceNames, "name", []string{}, "OpsRequest names")
}

// completeForDeleteOps complete cmd for delete OpsRequest, if resource name
// is not specified, construct a label selector based on the cluster name to
// delete all OpeRequest belonging to the cluster.
func completeForDeleteOps(option builder.Options, args []string) error {
	var (
		flag *delete.DeleteFlags
		ok   bool
	)
	if flag, ok = option.(*delete.DeleteFlags); !ok {
		return nil
	}

	// If resource name is not empty, delete these resources by name, do not need
	// to construct the label selector.
	if len(flag.ResourceNames) > 0 || len(args) == 0 {
		return nil
	}

	if len(args) > 1 {
		return fmt.Errorf("only support to delete the OpsRequests of one cluster")
	}

	flag.ClusterName = args[0]

	// If no specify OpsRequest name and cluster name is specified, delete all OpsRequest belonging to the cluster
	labelString := fmt.Sprintf("%s=%s", types.InstanceLabelKey, flag.ClusterName)
	if flag.LabelSelector == nil || len(*flag.LabelSelector) == 0 {
		flag.LabelSelector = &labelString
	} else {
		// merge label
		newLabelSelector := *flag.LabelSelector + "," + labelString
		flag.LabelSelector = &newLabelSelector
	}
	return nil
}
