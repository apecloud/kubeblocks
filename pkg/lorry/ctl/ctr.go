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
   / /   ____  ____________  __   / ____/ /______/ /
  / /   / __ \/ ___/ ___/ / / /  / /   / __/ ___/ / 
 / /___/ /_/ / /  / /  / /_/ /  / /___/ /_/ /  / /  
/_____/\____/_/  /_/   \__, /   \____/\__/_/  /_/   
                      /____/                        
===============================
Lorry service client`,
	Run: func(cmd *cobra.Command, _ []string) {
		if versionFlag {
			printVersion()
		} else {
			_ = cmd.Help()
		}
	},
}

type lorryVersion struct {
	CliVersion     string `json:"Cli version"`
	RuntimeVersion string `json:"Runtime version"`
}

var (
	cliVersion       string
	versionFlag      bool
	lorryVer         lorryVersion
	lorryRuntimePath string
)

// Execute adds all child commands to the root command.
func Execute(cliVersion, apiVersion string) {
	lorryVer = lorryVersion{
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
	template := fmt.Sprintf(cliVersionTemplateString, lorryVer.CliVersion, lorryVer.RuntimeVersion)
	RootCmd.SetVersionTemplate(template)
}

func printVersion() {
	fmt.Printf(cliVersionTemplateString, lorryVer.CliVersion, lorryVer.RuntimeVersion)
}

func initConfig() {
	// err intentionally ignored since lorry may not yet be installed.
	runtimeVer := GetRuntimeVersion()

	lorryVer = lorryVersion{
		// Set in Execute() method in this file before initConfig() is called by cmd.Execute().
		CliVersion:     cliVersion,
		RuntimeVersion: strings.ReplaceAll(runtimeVer, "\n", ""),
	}

	viper.SetEnvPrefix("lorry")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&lorryRuntimePath, "kb-runtime-dir", "", "/kubeblocks/", "The directory of kubeblocks binaries")
}

// GetRuntimeVersion returns the version for the local lorry runtime.
func GetRuntimeVersion() string {
	lorryCMD := filepath.Join(lorryRuntimePath, "lorry")

	out, err := exec.Command(lorryCMD, "--version").Output()
	if err != nil {
		return "n/a\n"
	}
	return string(out)
}
