/*
Copyright © 2022 The dbctl Authors

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

package dbcluster

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"jihulab.com/infracreate/dbaas-system/dbctl/pkg/utils"
)

func NewConnectCmd(f cmdutil.Factory) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to the database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				utils.Errf("You must specify a database cluster name to connect.")
				return
			}
		},
	}

	return cmd
}
