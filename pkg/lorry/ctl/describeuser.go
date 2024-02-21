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

type DescribeUserOptions struct {
	lorryAddr string
	userName  string
}

var describeUserOptions = &DescribeUserOptions{}

var DescribeUserCmd = &cobra.Command{
	Use:   "describeuser",
	Short: "describe user.",
	Example: `
lorryctl  describeuser --username xxx 
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		lorryClient, err := client.NewHTTPClientWithURL(describeUserOptions.lorryAddr)
		if err != nil {
			fmt.Printf("new lorry http client failed: %v\n", err)
			return
		}

		user, err := lorryClient.DescribeUser(context.TODO(), describeUserOptions.userName)
		if err != nil {
			fmt.Printf("describe user failed: %v\n", err)
			return
		}
		fmt.Println("describe user success:")
		for k, v := range user {
			fmt.Printf("%s: %v\n", k, v)
		}
	},
}

func init() {
	DescribeUserCmd.Flags().StringVarP(&describeUserOptions.userName, "username", "", "", "The name of user to describe")
	DescribeUserCmd.Flags().StringVarP(&describeUserOptions.lorryAddr, "lorry-addr", "", "http://localhost:3501/v1.0/", "The addr of lorry to request")
	DescribeUserCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(DescribeUserCmd)
}
