/*
Copyright © 2022 The OpenCli Authors

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

package cmd

import (
	"fmt"
	"os"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/bench"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/dbaas"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/dbcluster"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/playground"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "opencli",
		Short: "A Command Line Interface(CLI) library for dbaas.",
		Run:   runHelp,
	}

	flags := rootCmd.PersistentFlags()

	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)

	//f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	flags.ParseErrorsWhitelist.UnknownFlags = true

	// Add subcommands
	rootCmd.AddCommand(
		playground.NewPlaygroundCmd(ioStreams),
		dbaas.NewDbaasCmd(),
		dbcluster.NewDbclusterCmd(),
		bench.NewBenchCmd(),
	)

	cobra.OnInitialize(initConfig)
	return rootCmd
}

func runHelp(cmd *cobra.Command, args []string) {
	//nolint
	cmd.Help()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".opencli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".opencli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
