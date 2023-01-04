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
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var listOpsExample = templates.Examples(`
		# list all opsRequests
		kbcli cluster list-ops

		# list all opsRequests of specified cluster
		kbcli cluster list-ops my-cluster`)

func NewListOpsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.OpsGVR())
	cmd := &cobra.Command{
		Use:     "list-ops",
		Short:   "Liat all opsRequests",
		Aliases: []string{"ls-ops"},
		Example: listOpsExample,
		Run: func(cmd *cobra.Command, args []string) {
			// build label selector that used to get ops
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)

			// args are the cluster names, for ops, we only use the label selector to get ops, so resources names
			// is not needed.
			o.Names = nil
			_, err := o.Run()
			util.CheckErr(err)
		},
	}
	return cmd
}
