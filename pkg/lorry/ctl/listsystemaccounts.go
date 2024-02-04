/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package ctl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
)

var (
	lorryAddr1 string
)

var ListSystemAccountsCmd = &cobra.Command{
	Use:   "listsystemaccounts",
	Short: "list system accounts.",
	Example: `
lorryctl  listsystemaccounts 
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		lorryClient, err := client.NewHTTPClientWithURL(lorryAddr1)
		if err != nil {
			fmt.Printf("new lorry http client failed: %v\n", err)
			return
		}

		systemaccounts, err := lorryClient.ListSystemAccounts(context.TODO())
		if err != nil {
			fmt.Printf("list systemaccounts failed: %v\n", err)
			return
		}
		fmt.Printf("list systemaccounts:\n")
		for _, m := range systemaccounts {
			fmt.Println("-------------------------")
			for k, v := range m {
				fmt.Printf("%s: %v\n", k, v)
			}
		}
	},
}

func init() {
	ListSystemAccountsCmd.Flags().StringVarP(&lorryAddr1, "lorry-addr", "", "http://localhost:3501/v1.0/", "The addr of lorry to request")
	ListSystemAccountsCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(ListSystemAccountsCmd)
}
