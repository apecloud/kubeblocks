package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	leader         string
	candidate      string
	sqlchannelAddr string
)

var SwitchCmd = &cobra.Command{
	Use:   "switchover",
	Short: "execute a switchover request.",
	Example: `
sqlctl switchover  --leader xxx --candidate xxx
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		var characterType string
		if characterType = os.Getenv("KB_SERVICE_CHARACTER_TYPE"); characterType == "" {
			characterType = "mysql"
		}

		url := "http://" + sqlchannelAddr + "/v1.0/bindings/" + characterType

		payload := fmt.Sprintf(`{"operation": "switchover", "metadata": {"leader": "%s", "candidate": "%s"}}`, leader, candidate)
		//fmt.Println(payload)

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
		fmt.Println(string(bodyBytes))
		return
	},
}

func init() {
	SwitchCmd.Flags().StringVarP(&leader, "leader", "l", "", "The leader pod name")
	SwitchCmd.Flags().StringVarP(&candidate, "candidate", "c", "", "The candidate pod name")
	SwitchCmd.Flags().StringVarP(&sqlchannelAddr, "sqlchannel-addr", "", "localhost:3501", "The addr of sqlchannel to request")
	SwitchCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(SwitchCmd)
}
