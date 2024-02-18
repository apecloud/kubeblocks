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
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	configFile       string
	port             int
	grpcPort         int
	internalGRPCPort int
	logLevel         string
	componentsPath   string
	enableAppHealth  bool
)

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Lorry and db service.",
	Example: `
lorryctl run  -- mysqld
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("WARNING: no DB Service found.")
		}
		ctx, cancel := context.WithCancel(context.Background())

		commands, err := newCommands(ctx, &Options{
			HTTPPort:         port,
			GRPCPort:         grpcPort,
			ConfigFile:       configFile,
			Arguments:        args,
			LogLevel:         logLevel,
			ComponentsPath:   componentsPath,
			EnableAppHealth:  enableAppHealth,
			InternalGRPCPort: internalGRPCPort,
		})
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		go commands.StartLorry()
		<-commands.LorryStarted

		go commands.StartDBService()
		<-commands.AppStarted
		go commands.RestartDBServiceIfExited()

		if commands.AppCMD != nil {
			appCommand := strings.Join(args, " ")
			fmt.Fprintf(os.Stdout, "Start DB Service with %s.\n", appCommand)
			fmt.Fprintf(os.Stdout, "Lorry logs and DB logs will appear here.\n")
		} else {
			fmt.Fprintf(os.Stdout, "Lorry logs will appear here.\n")
		}

		sig := <-sigCh
		fmt.Printf("\n %v signal received: shutting down\n", sig)
		cancel()
		commands.WaitGroup.Wait()

		exitWithError := commands.StopLorry() || commands.StopDBService()
		if exitWithError {
			os.Exit(1)
		}
	},
}

func init() {
	RunCmd.Flags().StringVarP(&configFile, "config", "c", "/kubeblocks/config/probe/config.yaml", "Dapr configuration file")
	RunCmd.Flags().IntVarP(&port, "dapr-http-port", "H", -1, "The HTTP port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&grpcPort, "dapr-grpc-port", "G", -1, "The gRPC port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&internalGRPCPort, "dapr-internal-grpc-port", "I", 56471, "The gRPC port for the Dapr internal API to listen on")
	RunCmd.Flags().StringVarP(&logLevel, "log-level", "", "info", "The log verbosity. Valid values are: debug, info, warn, error, fatal, or panic")
	RunCmd.Flags().StringVarP(&componentsPath, "components-path", "d", "/kubeblocks/config/probe/components", "The path for components directory")
	RunCmd.Flags().BoolP("help", "h", false, "Print this help message")

	RootCmd.AddCommand(RunCmd)
}
