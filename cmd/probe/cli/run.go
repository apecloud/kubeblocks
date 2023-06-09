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
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dapr/cli/pkg/print"
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
			fmt.Println(print.WhiteBold("WARNING: no application command found."))
		}

		commands, err := newCommands(&Options{
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
			print.FailureStatusEvent(os.Stderr, err.Error())
			os.Exit(1)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		sqlchannelRunning := make(chan bool, 1)

		go func() {
			var startInfo string
			startInfo = fmt.Sprintf(
				"Starting SQLChannel HTTP Port: %v. gRPC Port: %v",
				commands.SQLChannelHTTPPort,
				commands.SQLChannelGRPCPort)
			print.InfoStatusEvent(os.Stdout, startInfo)

			commands.SQLChannelCMD.Stdout = os.Stdout
			commands.SQLChannelCMD.Stderr = os.Stderr

			err = commands.SQLChannelCMD.Start()
			if err != nil {
				print.FailureStatusEvent(os.Stderr, err.Error())
				os.Exit(1)
			}

			go func() {
				sqlchannelErr := commands.SQLChannelCMD.Wait()

				if sqlchannelErr != nil {
					commands.SQLChannelErr = sqlchannelErr
					print.FailureStatusEvent(os.Stderr, "The daprd process exited with error code: %s", sqlchannelErr.Error())
				} else {
					print.SuccessStatusEvent(os.Stdout, "Exited SQLChannel successfully")
				}
				sigCh <- os.Interrupt
			}()

			sqlchannelRunning <- true
		}()

		<-sqlchannelRunning

		go commands.StartDBService()

		appRunStatus := <-commands.AppStarted
		go commands.RestartDBServiceIfExited()

		if !appRunStatus {
			// Start App failed, try to stop SQLChannel and exit.
			err = commands.SQLChannelCMD.Process.Kill()
			if err != nil {
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Start App failed, try to stop SQLChannel Error: %s", err))
			} else {
				print.SuccessStatusEvent(os.Stdout, "Start App failed, try to stop SQLChannel successfully")
			}
			os.Exit(1)
		}

		if commands.AppCMD != nil {
			appCommand := strings.Join(args, " ")
			print.SuccessStatusEvent(os.Stdout, fmt.Sprintf("start DB Service with %s.\n", appCommand))
			print.SuccessStatusEvent(os.Stdout, "SQLChannel logs and DB logs will appear here.\n")
		} else {
			print.SuccessStatusEvent(os.Stdout, "You're up and running! SQLChannel logs will appear here.\n")
		}

		<-sigCh
		commands.IsStopped = true
		print.InfoStatusEvent(os.Stdout, "\nterminated signal received: shutting down")

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
