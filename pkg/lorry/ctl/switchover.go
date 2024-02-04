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
	"time"

	"github.com/spf13/cobra"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
)

type SwitchOptions struct {
	primary   string
	candidate string
	lorryAddr string
	force     bool
}

var switchOptions = &SwitchOptions{}
var SwitchCmd = &cobra.Command{
	Use:   "switchover",
	Short: "execute a switchover request.",
	Example: `
lorryctl switchover  --primary xxx --candidate xxx
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if switchOptions.primary == "" && switchOptions.candidate == "" {
			fmt.Println("Primary or Candidate must be specified")
			return
		}

		lorryClient, err := client.NewHTTPClientWithURL(switchOptions.lorryAddr)
		if err != nil {
			fmt.Printf("new lorry http client failed: %v\n", err)
			return
		}

		lorryClient.ReconcileTimeout = 30 * time.Second
		err = lorryClient.Switchover(context.TODO(), switchOptions.primary, switchOptions.candidate, switchOptions.force)
		if err != nil {
			fmt.Printf("switchover failed: %v\n", err)
			return
		}
		fmt.Printf("switchover success\n")
	},
}

func init() {
	SwitchCmd.Flags().StringVarP(&switchOptions.primary, "primary", "p", "", "The primary pod name")
	SwitchCmd.Flags().StringVarP(&switchOptions.candidate, "candidate", "c", "", "The candidate pod name")
	SwitchCmd.Flags().BoolVarP(&switchOptions.force, "force", "f", false, "force to swithover if failed")
	SwitchCmd.Flags().StringVarP(&switchOptions.lorryAddr, "lorry-addr", "", "http://localhost:3501/v1.0/", "The addr of lorry to request")
	SwitchCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(SwitchCmd)
}
