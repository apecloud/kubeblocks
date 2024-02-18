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

type CreateUserOptions struct {
	lorryAddr string
	userName  string
	password  string
}

var createUserOptions = &CreateUserOptions{}

var CreateUserCmd = &cobra.Command{
	Use:   "createuser",
	Short: "create user.",
	Example: `
lorryctl  createuser --username xxx --password xxx 
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		lorryClient, err := client.NewHTTPClientWithURL(createUserOptions.lorryAddr)
		if err != nil {
			fmt.Printf("new lorry http client failed: %v\n", err)
			return
		}

		err = lorryClient.CreateUser(context.TODO(), createUserOptions.userName, createUserOptions.password, "")
		if err != nil {
			fmt.Printf("create user failed: %v\n", err)
			return
		}
		fmt.Printf("create user success")
	},
}

func init() {
	CreateUserCmd.Flags().StringVarP(&createUserOptions.userName, "username", "", "", "The name of user to create")
	CreateUserCmd.Flags().StringVarP(&createUserOptions.password, "password", "", "", "The password of user to create")
	CreateUserCmd.Flags().StringVarP(&createUserOptions.lorryAddr, "lorry-addr", "", "http://localhost:3501/v1.0/", "The addr of lorry to request")
	CreateUserCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(CreateUserCmd)
}
