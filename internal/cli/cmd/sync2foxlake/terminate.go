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

package sync2foxlake

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var terminateExample = templates.Examples(`
	# terminate a sync2foxlake task named mytask, this operation will delete the database in the foxlake cluster
	kbcli sync2foxlake terminate mytask
`)

func NewSync2FoxLakeTerminateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.Sync2FoxLakeTaskGVR())
	cmd := &cobra.Command{
		Use:               "terminate NAME",
		Short:             "Delete sync2foxlake tasks.",
		Example:           terminateExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.Sync2FoxLakeTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(deleteSync2FoxLakeTask(o, args))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func deleteSync2FoxLakeTask(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing sync2foxlake task name")
	}
	o.Names = args
	return o.Run()
}
