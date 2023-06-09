/*
Copyright 2021 The Dapr Authors
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

package cli

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
	configFile         string
	port               int
	grpcPort           int
	internalGRPCPort   int
	logLevel           string
	componentsPath     string
	metricsPort        int
	maxRequestBodySize int
	readBufferSize     int
	enableAppHealth    bool
)

const (
	runtimeWaitTimeoutInSeconds = 60
)

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run sqlchannel and db service side by side.",
	Example: `
sqlchannelctr run  -- mysqld
  `,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("WARNING: no DB Service found.")
		}
		ctx, cancel := context.WithCancel(context.Background())

		commands, err := newCommands(ctx, &Options{
			HTTPPort:           port,
			GRPCPort:           grpcPort,
			ConfigFile:         configFile,
			Arguments:          args,
			LogLevel:           logLevel,
			ComponentsPath:     componentsPath,
			MetricsPort:        metricsPort,
			MaxRequestBodySize: maxRequestBodySize,
			HTTPReadBufferSize: readBufferSize,
			EnableAppHealth:    enableAppHealth,
			InternalGRPCPort:   internalGRPCPort,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

		go commands.StartSQLChannel()
		<-commands.SQLChannelStarted

		go commands.StartDBService()
		<-commands.AppStarted
		go commands.RestartDBServiceIfExited()

		if commands.AppCMD != nil {
			appCommand := strings.Join(args, " ")
			fmt.Fprintf(os.Stdout, "Start DB Service with %s.\n", appCommand)
			fmt.Fprintf(os.Stdout, "SQLChannel logs and DB logs will appear here.\n")
		} else {
			fmt.Fprintf(os.Stdout, "SQLChannel logs will appear here.\n")
		}

		sig := <-sigCh
		fmt.Printf("\n %v signal received: shutting down\n", sig)
		cancel()
		commands.WaitGroup.Wait()

		exitWithError := commands.StopSQLChannel() || commands.StopDBService()
		if exitWithError {
			os.Exit(1)
		}
	},
}

func init() {
	RunCmd.Flags().StringVarP(&configFile, "config", "c", "/kubeblocks/probe/config.yaml", "Dapr configuration file")
	RunCmd.Flags().IntVarP(&port, "dapr-http-port", "H", -1, "The HTTP port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&grpcPort, "dapr-grpc-port", "G", -1, "The gRPC port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&internalGRPCPort, "dapr-internal-grpc-port", "I", 56471, "The gRPC port for the Dapr internal API to listen on")
	RunCmd.Flags().StringVarP(&logLevel, "log-level", "", "info", "The log verbosity. Valid values are: debug, info, warn, error, fatal, or panic")
	RunCmd.Flags().StringVarP(&componentsPath, "components-path", "d", "/kubeblocks/probe/components", "The path for components directory")
	RunCmd.Flags().IntVarP(&metricsPort, "metrics-port", "M", -1, "The port of metrics on dapr")
	RunCmd.Flags().BoolP("help", "h", false, "Print this help message")
	RunCmd.Flags().IntVarP(&maxRequestBodySize, "dapr-http-max-request-size", "", -1, "Max size of request body in MB")
	RunCmd.Flags().IntVarP(&readBufferSize, "dapr-http-read-buffer-size", "", -1, "HTTP header read buffer in KB")

	RootCmd.AddCommand(RunCmd)
}
