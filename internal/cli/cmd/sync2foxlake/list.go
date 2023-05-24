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
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listExample = templates.Examples(`
	# list all sync2foxlake tasks
	kbcli sync2foxlake list
	# list a single sync2foxlake task with specified NAME
	kbcli sync2foxlake list mytask
`)

	describeExample = templates.Examples(`
		# describe a sync2foxlake task named mytask
		kbcli sync2foxlake describe mytask
	`)
)

type PrintSync2FoxLakeOptions struct {
	Name    string
	printer func() error
	*Sync2FoxLakeExecOptions
}

func NewSync2FoxLakeListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PrintSync2FoxLakeOptions{
		Sync2FoxLakeExecOptions: newSync2FoxLakeExecOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:     "list [NAME]",
		Short:   "List sync2foxlake tasks.",
		Example: listExample,
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			o.printer = func() error {
				if len(o.Outputs) == 0 {
					fmt.Fprintf(o.Stdout, "No sync2foxlake tasks found.\n")
					return nil
				}
				tbl := printer.NewTablePrinter(o.Stdout)
				tbl.SetHeader("NAME", "DATABASE", "APPLIED-SEQUENCE-ID", "TARGET-SEQUENCE-ID", "STATUS")
				for _, output := range o.Outputs {
					lines := strings.Split(output, "\n")
					if len(lines) < 2 {
						return fmt.Errorf("invalid output: %s", output)
					}
					result := strings.Fields(lines[1])
					if len(result) != 4 {
						return fmt.Errorf("invalid output: %s", lines[1])
					}
					tbl.AddRow(o.Name, result[0], result[1], result[2], result[3])
				}
				tbl.Print()
				return nil
			}
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run(func(database string) string {
				return "show synchronized database " + database + " status;"
			}))
		},
	}
	return cmd
}

func NewSync2FoxLakeDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PrintSync2FoxLakeOptions{
		Sync2FoxLakeExecOptions: newSync2FoxLakeExecOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:     "describe NAME",
		Short:   "Show details of a specific sync2foxlake task.",
		Args:    cobra.ExactArgs(1),
		Example: describeExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.printer = func() error {
				if len(o.Outputs) < 1 {
					return fmt.Errorf("no output")
				}
				output := o.Outputs[0]
				lines := strings.Split(output, "\n")
				if len(lines) < 4 {
					return fmt.Errorf("invalid output: %s", output)
				}
				status := strings.Fields(lines[1])
				if len(status) != 4 {
					return fmt.Errorf("invalid status: %s", lines[1])
				}
				var engine, databaseType, datasourceEndpoint, databaseSelected, lag string
				for i := 3; i < len(lines); i++ {
					attributes := strings.Fields(lines[i])
					if len(attributes) >= 7 && attributes[0] == status[0] {
						engine = attributes[1]
						databaseType = attributes[2]
						datasourceEndpoint = attributes[3]
						databaseSelected = attributes[4]
						lag = attributes[len(attributes)-2] + " " + attributes[len(attributes)-1]
						break
					}
				}

				fmt.Fprintln(o.Stdout, "Name:", o.Name)
				fmt.Fprintln(o.Stdout)
				attrTbl := printer.NewTablePrinter(o.Stdout)
				attrTbl.AddRow("Database:", status[0])
				attrTbl.AddRow("Engine:", engine)
				attrTbl.AddRow("DatabaseType:", databaseType)
				attrTbl.AddRow("DatasourceEndpoint:", datasourceEndpoint)
				attrTbl.AddRow("DatabaseSelected:", databaseSelected)
				attrTbl.AddRow("Lag:", lag)
				attrTbl.Print()
				fmt.Fprintln(o.Stdout)
				statusTbl := printer.NewTablePrinter(o.Stdout)
				statusTbl.SetHeader("DATABASE", "APPLIED-SEQUENCE-ID", "TARGET-SEQUENCE-ID", "STATUS")
				statusTbl.AddRow(status[0], status[1], status[2], status[3])
				statusTbl.Print()

				return nil
			}
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run(func(database string) string {
				return "show synchronized database " + database + " status;show synchronized databases;"
			}))
		},
	}
	return cmd
}

func (o *PrintSync2FoxLakeOptions) complete(args []string) error {
	if len(args) > 0 {
		o.Name = args[0]
	}

	if err := o.Sync2FoxLakeExecOptions.complete(); err != nil {
		return err
	}

	return nil
}

func (o *PrintSync2FoxLakeOptions) run(buildSQL func(string) string) error {
	var err error
	if o.Name != "" {
		// print only one task
		if err = o.Sync2FoxLakeExecOptions.run(o.Name, buildSQL); err != nil {
			return err
		}
	} else {
		// print all tasks
		for k := range o.Cm.Data {
			o.Name = k
			if err = o.Sync2FoxLakeExecOptions.run(k, buildSQL); err != nil {
				return err
			}
		}
	}

	if o.printer != nil {
		err = o.printer()
	}

	return err
}
