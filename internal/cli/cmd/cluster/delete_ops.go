/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

func NewDeleteOpsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.OpsGVR())
	cmd := &cobra.Command{
		Use:               "delete-ops",
		Short:             "Delete an OpsRequest.",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(completeForDeleteOps(o, args))
			util.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringSliceVar(&o.Names, "name", []string{}, "OpsRequest names")
	o.AddFlags(cmd)
	return cmd
}

// completeForDeleteOps complete cmd for delete OpsRequest, if resource name
// is not specified, construct a label selector based on the cluster name to
// delete all OpeRequest belonging to the cluster.
func completeForDeleteOps(o *delete.DeleteOptions, args []string) error {
	// If resource name is not empty, delete these resources by name, do not need
	// to construct the label selector.
	if len(o.Names) > 0 {
		o.ConfirmedNames = o.Names
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("missing cluster name")
	}

	if len(args) > 1 {
		return fmt.Errorf("only support to delete the OpsRequests of one cluster")
	}

	o.ConfirmedNames = args
	// If no specify OpsRequest name and cluster name is specified, delete all OpsRequest belonging to the cluster
	o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
	return nil
}
