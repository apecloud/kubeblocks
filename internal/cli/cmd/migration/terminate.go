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

package migration

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

func NewMigrationTerminateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := delete.NewDeleteOptions(f, streams, types.MigrationTaskGVR())
	cmd := &cobra.Command{
		Use:               "terminate NAME",
		Short:             "Delete migration task.",
		Example:           DeleteExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.MigrationTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			_, validErr := IsMigrationCrdValidWithFactory(o.Factory)
			PrintCrdInvalidError(validErr)
			util.CheckErr(deleteMigrationTask(o, args))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func deleteMigrationTask(o *delete.DeleteOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing migration task name")
	}
	o.Names = args
	return o.Run()
}
