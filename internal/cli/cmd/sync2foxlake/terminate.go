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
	"context"
	"fmt"

	"github.com/docker/cli/cli"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var terminateExample = templates.Examples(`
	# terminate a sync2foxlake task named mytask, this operation will delete the database in the foxlake cluster
	kbcli sync2foxlake terminate mytask
`)

func NewSync2FoxLakeTerminateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newSync2FoxLakeExecOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "terminate NAME",
		Short:   "Delete sync2foxlake tasks.",
		Args:    cli.ExactArgs(1),
		Example: terminateExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(deleteSync2FoxLakeTask(o, args[0]))
		},
	}
	return cmd
}

func deleteSync2FoxLakeTask(o *Sync2FoxLakeExecOptions, name string) error {
	if err := o.run(name, func(database string) string {
		return "Drop database " + database + ";"
	}); err != nil {
		return err
	}
	delete(o.Cm.Data, name)

	if len(o.Cm.Data) == 0 {
		if err := o.Client.CoreV1().ConfigMaps(o.Namespace).Delete(context.TODO(), o.Cm.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	} else {
		if _, err := o.Client.CoreV1().ConfigMaps(o.Namespace).Update(context.TODO(), o.Cm, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	fmt.Fprintf(o.Stdout, "Sync2foxlake task %s terminated.\n", name)

	return nil
}
