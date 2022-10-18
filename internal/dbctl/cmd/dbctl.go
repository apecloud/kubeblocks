/*
Copyright 2022 The KubeBlocks Authors

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

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/backup"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/bench"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/dbaas"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/options"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/playground"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
)

// RootFlags describes a struct that holds flags that can be set on root level of the command
type RootFlags struct {
	version bool
}

var cfgFile string

var rootFlags = RootFlags{}

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dbctl",
		Short: "KubeBlocks CLI",
		Long: `
=========================================
       __ __                  __     __
      |  \  \                |  \   |  \
  ____| ▓▓ ▓▓____   _______ _| ▓▓_  | ▓▓
 /      ▓▓ ▓▓    \ /       \   ▓▓ \ | ▓▓
|  ▓▓▓▓▓▓▓ ▓▓▓▓▓▓▓\  ▓▓▓▓▓▓▓\▓▓▓▓▓▓ | ▓▓
| ▓▓  | ▓▓ ▓▓  | ▓▓ ▓▓       | ▓▓ __| ▓▓
| ▓▓__| ▓▓ ▓▓__/ ▓▓ ▓▓_____  | ▓▓|  \ ▓▓
 \▓▓    ▓▓ ▓▓    ▓▓\▓▓     \  \▓▓  ▓▓ ▓▓
  \▓▓▓▓▓▓▓\▓▓▓▓▓▓▓  \▓▓▓▓▓▓▓   \▓▓▓▓ \▓▓

=========================================
A database management tool for KubeBlocks`,

		Run: func(cmd *cobra.Command, args []string) {
			if rootFlags.version {
				util.PrintVersion()
			} else {
				_ = cmd.Help()
			}
		},
	}

	// add local flags
	rootCmd.Flags().BoolVar(&rootFlags.version, "version", false, "Show version")

	// From this point and forward we get warnings on flags that contain "_" separators
	// when adding them with hyphen instead of the original name.
	rootCmd.SetGlobalNormalizationFunc(cliflag.WarnWordSepNormalizeFunc)

	flags := rootCmd.PersistentFlags()

	// add kubernetes flags like kubectl
	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	// Add subcommands
	rootCmd.AddCommand(
		playground.NewPlaygroundCmd(ioStreams),
		dbaas.NewDbaasCmd(f, ioStreams),
		cluster.NewClusterCmd(f, ioStreams),
		bench.NewBenchCmd(),
		backup.NewBackupCmd(f, ioStreams),
		options.NewCmdOptions(ioStreams.Out),
	)

	filters := []string{"options"}
	templates.ActsAsRootCommand(rootCmd, filters, []templates.CommandGroup{}...)

	cobra.OnInitialize(initConfig)
	return rootCmd
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

		// Search config in home directory with name ".dbctl" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".dbctl")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
