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

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/get"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/list"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
)

func NewOpsListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return builder.NewCmdBuilder().
		Use("list-ops").
		Short("List all opsRequest.").
		Factory(f).
		GVR(types.OpsGVR()).
		CustomComplete(completeForListOps).
		IOStreams(streams).
		Build(list.Build)
}

// completeForListOps complete the cmd for list OpsRequest
func completeForListOps(option interface{}, args []string) error {
	var (
		o  *get.Options
		ok bool
	)
	if o, ok = option.(*get.Options); !ok {
		return nil
	}
	// if cluster name is not nil, covert to label for list OpsRequest
	if len(args) > 0 {
		labelString := fmt.Sprintf("%s=%s", types.InstanceLabelKey, args[0])
		if len(o.LabelSelector) == 0 {
			o.LabelSelector = labelString
		} else {
			o.LabelSelector += "," + labelString
		}
	}
	return nil
}
