/*
Copyright © 2022 The OpenCli Authors

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

package bench

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewBenchCmd creates the bench command
func NewBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Run a benchmark",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("bench called")
		},
	}

	// add subcommands
	cmd.AddCommand(
		newTpccCmd(),
		newTpchCmd(),
	)
	return cmd
}

func newTpccCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tpcc",
		Short: "Run a TPCC benchmark",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("bench called")
		},
	}

	return cmd
}

func newTpchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tpch",
		Short: "Run a TPCH benchmark",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("bench called")
		},
	}

	return cmd
}
