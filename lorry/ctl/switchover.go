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

package ctl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

var (
	primary        string
	candidate      string
	sqlchannelAddr string
)

var SwitchCmd = &cobra.Command{
	Use:   "switchover",
	Short: "execute a switchover request.",
	Example: `
lorryctl switchover  --primary xxx --candidate xxx
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		var characterType string
		if characterType = os.Getenv(constant.KBEnvCharacterType); characterType == "" {
			fmt.Println("KB_SERVICE_CHARACTER_TYPE must be set")
			return
		}

		url := "http://" + sqlchannelAddr + "/v1.0/bindings/" + characterType
		if primary == "" && candidate == "" {
			fmt.Println("Primary or Candidate must be specified")
			return
		}

		payload := fmt.Sprintf(`{"operation": "switchover", "metadata": {"leader": "%s", "candidate": "%s"}}`, primary, candidate)
		// fmt.Println(payload)

		client := http.Client{}
		// Insert order using Dapr output binding via HTTP Post
		req, err := http.NewRequest("POST", url, strings.NewReader(payload))
		if err != nil {
			fmt.Printf("New request error: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Request SQLChannel error: %v", err)
			return
		}
		fmt.Println("SQLChannel Response:")
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("request error: %v", err)
		}
		fmt.Println(string(bodyBytes))
	},
}

func init() {
	SwitchCmd.Flags().StringVarP(&primary, "primary", "l", "", "The primary pod name")
	SwitchCmd.Flags().StringVarP(&candidate, "candidate", "c", "", "The candidate pod name")
	SwitchCmd.Flags().StringVarP(&sqlchannelAddr, "sqlchannel-addr", "", "localhost:3501", "The addr of sqlchannel to request")
	SwitchCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(SwitchCmd)
}
