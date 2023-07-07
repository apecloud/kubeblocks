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

package bench

import (
	"fmt"
	"sort"

	"github.com/docker/cli/cli"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

const (
	all              = "all"
	prepareOperation = "prepare"
	runOperation     = "run"
	cleanupOperation = "cleanup"
)

type BenchBaseOptions struct {
	Driver   string `json:"driver"`
	Database string `json:"database"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func (o *BenchBaseOptions) BaseValidate() error {
	if o.Driver == "" {
		return fmt.Errorf("driver is required")
	}

	if o.Database == "" {
		return fmt.Errorf("database name should be specified")
	}

	if o.Host == "" {
		return fmt.Errorf("host is required")
	}

	if o.Port == 0 {
		return fmt.Errorf("port is required")
	}

	if o.User == "" {
		return fmt.Errorf("user is required")
	}

	if o.Password == "" {
		return fmt.Errorf("password is required")
	}

	return nil
}

func (o *BenchBaseOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&o.Database, "database", "", "database name")
	cmd.PersistentFlags().StringVar(&o.Host, "host", "", "the host of database")
	cmd.PersistentFlags().StringVar(&o.User, "user", "", "the user of database")
	cmd.PersistentFlags().StringVar(&o.Password, "password", "", "the password of database")
	cmd.PersistentFlags().IntVar(&o.Port, "port", 0, "the port of database")
}

// NewBenchCmd creates the bench command
func NewBenchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run a benchmark.",
	}

	// add subcommands
	cmd.AddCommand(
		NewSysBenchCmd(f, streams),
		NewPgBenchCmd(f, streams),
		newListCmd(f, streams),
	)

	return cmd
}

type benchListOptions struct {
	Factory       cmdutil.Factory
	LabelSelector string
	Format        string

	genericclioptions.IOStreams
}

func newListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &benchListOptions{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all benchmarks.",
		Aliases: []string{"ls"},
		Args:    cli.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(benchListRun(o))
		},
	}

	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	return cmd
}

func benchListRun(o *benchListOptions) error {
	benchGVRList := []schema.GroupVersionResource{
		types.PgBenchGVR(),
		types.SysbenchGVR(),
	}

	var infos []*resource.Info
	for _, gvr := range benchGVRList {
		bench := list.NewListOptions(o.Factory, o.IOStreams, gvr)

		bench.Print = false
		bench.LabelSelector = o.LabelSelector
		result, err := bench.Run()
		if err != nil {
			return err
		}

		benchInfos, err := result.Infos()
		if err != nil {
			return err
		}
		infos = append(infos, benchInfos...)
	}

	if len(infos) == 0 {
		fmt.Fprintf(o.Out, "No benchmarks found.\n")
		return nil
	}

	printRows := func(tbl *printer.TablePrinter) error {
		// sort bench with kind, then .status.phase, finally .metadata.name
		sort.SliceStable(infos, func(i, j int) bool {
			iKind := infos[i].Object.(*unstructured.Unstructured).GetKind()
			jKind := infos[j].Object.(*unstructured.Unstructured).GetKind()
			iPhase := infos[i].Object.(*unstructured.Unstructured).Object["status"].(map[string]interface{})["phase"]
			jPhase := infos[j].Object.(*unstructured.Unstructured).Object["status"].(map[string]interface{})["phase"]
			iName := infos[i].Object.(*unstructured.Unstructured).GetName()
			jName := infos[j].Object.(*unstructured.Unstructured).GetName()

			if iKind != jKind {
				return iKind < jKind
			}
			if iPhase != jPhase {
				return iPhase.(string) < jPhase.(string)
			}
			return iName < jName
		})

		for _, info := range infos {
			obj := info.Object.(*unstructured.Unstructured)
			tbl.AddRow(
				obj.GetName(),
				obj.GetKind(),
				obj.Object["status"].(map[string]interface{})["phase"],
				obj.Object["status"].(map[string]interface{})["completions"],
			)
		}
		return nil
	}

	if err := printer.PrintTable(o.Out, nil, printRows, "NAME", "KIND", "STATUS", "COMPLETIONS"); err != nil {
		return err
	}
	return nil
}
