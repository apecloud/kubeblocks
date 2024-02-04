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
	lorryAddr string
)

var ListUsersCmd = &cobra.Command{
	Use:   "listusers",
	Short: "list normal users.",
	Example: `
lorryctl  listusers 
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		lorryClient, err := client.NewHTTPClientWithURL(lorryAddr)
		if err != nil {
			fmt.Printf("new lorry http client failed: %v\n", err)
			return
		}

		users, err := lorryClient.ListUsers(context.TODO())
		if err != nil {
			fmt.Printf("list users failed: %v\n", err)
			return
		}
		fmt.Printf("list users:\n")
		for _, m := range users {
			fmt.Println("-------------------------")
			for k, v := range m {
				fmt.Printf("%s: %v\n", k, v)
			}
		}
	},
}

func init() {
	ListUsersCmd.Flags().StringVarP(&lorryAddr, "lorry-addr", "", "http://localhost:3501/v1.0/", "The addr of lorry to request")
	ListUsersCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(ListUsersCmd)
}
