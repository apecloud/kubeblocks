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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const cliVersionTemplateString = "CLI version: %s \nRuntime version: %s\n"

var RootCmd = &cobra.Command{
	Use:   "lorryctl",
	Short: "LORRY CLI",
	Long: `
   _____ ____    __       ________                           __
  / ___// __ \  / /      / ____/ /_  ____ _____  ____  ___  / /
  \__ \/ / / / / /      / /   / __ \/ __ \/ __ \/ __ \/ _ \/ /
 ___/ / /_/ / / /___   / /___/ / / / /_/ / / / / / / /  __/ /
/____/\___\_\/_____/   \____/_/ /_/\__,_/_/ /_/_/ /_/\___/_/
									   
===============================
SQL Channel client`,
	Run: func(cmd *cobra.Command, _ []string) {
		if versionFlag {
			printVersion()
		} else {
			_ = cmd.Help()
		}
	},
}

type sqlChannelVersion struct {
	CliVersion     string `json:"Cli version"`
	RuntimeVersion string `json:"Runtime version"`
}

var (
	cliVersion            string
	versionFlag           bool
	sqlChannelVer         sqlChannelVersion
	sqlChannelRuntimePath string
)

// Execute adds all child commands to the root command.
func Execute(cliVersion, apiVersion string) {
	sqlChannelVer = sqlChannelVersion{
		CliVersion:     cliVersion,
		RuntimeVersion: apiVersion,
	}

	cobra.OnInitialize(initConfig)

	setVersion()

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func setVersion() {
	template := fmt.Sprintf(cliVersionTemplateString, sqlChannelVer.CliVersion, sqlChannelVer.RuntimeVersion)
	RootCmd.SetVersionTemplate(template)
}

func printVersion() {
	fmt.Printf(cliVersionTemplateString, sqlChannelVer.CliVersion, sqlChannelVer.RuntimeVersion)
}

func initConfig() {
	// err intentionally ignored since sqlChanneld may not yet be installed.
	runtimeVer := GetRuntimeVersion()

	sqlChannelVer = sqlChannelVersion{
		// Set in Execute() method in this file before initConfig() is called by cmd.Execute().
		CliVersion:     cliVersion,
		RuntimeVersion: strings.ReplaceAll(runtimeVer, "\n", ""),
	}

	viper.SetEnvPrefix("sqlChannel")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&sqlChannelRuntimePath, "kb-runtime-dir", "", "/kubeblocks/", "The directory of kubeblocks binaries")
}

// GetRuntimeVersion returns the version for the local sqlChannel runtime.
func GetRuntimeVersion() string {
	sqlchannelCMD := filepath.Join(sqlChannelRuntimePath, "probe")

	out, err := exec.Command(sqlchannelCMD, "--version").Output()
	if err != nil {
		return "n/a\n"
	}
	return string(out)
}
