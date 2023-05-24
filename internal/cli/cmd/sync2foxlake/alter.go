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

	"github.com/docker/cli/cli"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	pauseExample = templates.Examples(`
		# pause a sync2foxlake task named mytask
		kbcli sync2foxlake pause mytask
	`)
	resumeExample = templates.Examples(`
		# resume a sync2foxlake task named mytask
		kbcli sync2foxlake resume mytask	
	`)
)

func NewSync2FoxLakePauseCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newSync2FoxLakeExecOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "pause NAME",
		Short:   "Pause database synchronization.",
		Args:    cli.ExactArgs(1),
		Example: pauseExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run(args[0], func(database string) string {
				return "alter synchronized database " + database + " pause;"
			}))
			fmt.Fprintf(o.Stdout, "Sync2foxlake task %s paused.\n", args[0])
		},
	}
	return cmd
}

func NewSync2FoxLakeResumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newSync2FoxLakeExecOptions(f, streams)
	cmd := &cobra.Command{
		Use:     "resume NAME",
		Short:   "Resume database synchronization.",
		Args:    cli.ExactArgs(1),
		Example: resumeExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.run(args[0], func(database string) string {
				return "alter synchronized database " + database + " unpause;"
			}))
			fmt.Fprintf(o.Stdout, "Sync2foxlake task %s unpaused.\n", args[0])
		},
	}
	return cmd
}
