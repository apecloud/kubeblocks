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
	"os"

	hclog "github.com/hashicorp/go-hclog"
	dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/spf13/cobra"

	"github.com/apecloud/kubeblocks/pkg/lorry/vault"
)

// Run instantiates a lorry vault plugin object, and runs the RPC server for the plugin
func Run() error {
	db, err := vault.New()
	if err != nil {
		return err
	}

	dbplugin.Serve(db.(dbplugin.Database))

	return nil
}

type VaultPluginOptions struct {
	primary   string
	candidate string
	lorryAddr string
}

var vaultPluginOptions = &VaultPluginOptions{}
var VaultPluginCmd = &cobra.Command{
	Use:   "vault-plugin",
	Short: "run a vault-plugin service.",
	Example: `
lorryctl vault-plugin  --primary xxx --candidate xxx
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		err := Run()
		if err != nil {
			logger := hclog.New(&hclog.LoggerOptions{})

			logger.Error("plugin shutting down", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	VaultPluginCmd.Flags().StringVarP(&vaultPluginOptions.primary, "primary", "l", "", "The primary pod name")
	VaultPluginCmd.Flags().StringVarP(&vaultPluginOptions.candidate, "candidate", "c", "", "The candidate pod name")
	VaultPluginCmd.Flags().StringVarP(&vaultPluginOptions.lorryAddr, "lorry-addr", "", "localhost:3501", "The addr of lorry to request")
	VaultPluginCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(VaultPluginCmd)
}
