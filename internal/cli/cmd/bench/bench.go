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
	"embed"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

const (
	CueSysBenchTemplateName = "bench_sysbench_template.cue"
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
		return fmt.Errorf("database is required")
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
	cmd.Flags().StringVar(&o.Driver, "driver", "", "database driver")
	cmd.Flags().StringVar(&o.Database, "database", "", "database name")
	cmd.Flags().StringVar(&o.Host, "host", "", "the host of database")
	cmd.Flags().StringVar(&o.User, "user", "", "the user of database")
	cmd.Flags().StringVar(&o.Password, "password", "", "the password of database")
	cmd.Flags().IntVar(&o.Port, "port", 0, "the port of database")
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
	)

	return cmd
}
