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
		GroupKind(types.OpsGK()).
		Factory(f).
		IOStreams(streams).
		CustomComplete(completeForDeleteOps).
		CustomFlags(customFlagsForDeleteOps).
		Build(delete.Build)
}

func customFlagsForDeleteOps(option interface{}, cmd *cobra.Command) {
	var (
		o  *delete.DeleteFlags
		ok bool
	)
	if o, ok = option.(*delete.DeleteFlags); !ok {
		return
	}
	cmd.Flags().StringVarP(&o.Name, "name", "", "", "OpsRequest name")
}

// completeForDeleteOps complete cmd for delete OpsRequest
func completeForDeleteOps(option interface{}, args []string) []string {
	var (
		flag        *delete.DeleteFlags
		ok          bool
		clusterName string
	)
	if flag, ok = option.(*delete.DeleteFlags); !ok {
		return args
	}
	// covert OpsRequest name to delete args, then we can delete the OpsRequest
	if len(flag.Name) > 0 {
		args = []string{flag.Name}
	} else if len(args) > 0 {
		clusterName = (args)[0]
		args = []string{}
	}
	if len(clusterName) == 0 {
		return args
	}
	// if no specify OpsRequest name and cluster name is specified. it will delete all OpsRequest with the cluster
	labelString := fmt.Sprintf("%s=%s", types.InstanceLabelKey, clusterName)
	if flag.LabelSelector == nil || len(*flag.LabelSelector) == 0 {
		flag.LabelSelector = &labelString
	} else {
		// merge label
		newLabelSelector := *flag.LabelSelector + "," + labelString
		flag.LabelSelector = &newLabelSelector
	}
	return args
}
