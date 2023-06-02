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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dapr/cli/pkg/metadata"
	"github.com/dapr/cli/pkg/print"
	"github.com/dapr/cli/pkg/standalone"
	"github.com/dapr/cli/utils"
)

var (
	appPort            int
	profilePort        int
	appID              string
	configFile         string
	port               int
	grpcPort           int
	internalGRPCPort   int
	maxConcurrency     int
	enableProfiling    bool
	logLevel           string
	protocol           string
	componentsPath     string
	appSSL             bool
	metricsPort        int
	maxRequestBodySize int
	readBufferSize     int
	unixDomainSocket   string
	enableAppHealth    bool
	appHealthPath      string
	appHealthInterval  int
	appHealthTimeout   int
	appHealthThreshold int
	enableAPILogging   bool
)

const (
	runtimeWaitTimeoutInSeconds = 60
)

type Config standalone.RunConfig

var RunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Dapr and (optionally) your application side by side. Supported platforms: Self-hosted",
	Example: `
# Run a .NET application
dapr run --app-id myapp --app-port 5000 -- dotnet run

# Run a Java application
dapr run --app-id myapp -- java -jar myapp.jar

# Run a NodeJs application that listens to port 3000
dapr run --app-id myapp --app-port 3000 -- node myapp.js

# Run a Python application
dapr run --app-id myapp -- python myapp.py

# Run sidecar only
dapr run --app-id myapp

# Run a gRPC application written in Go (listening on port 3000)
dapr run --app-id myapp --app-port 3000 --app-protocol grpc -- go run main.go
  `,
	Args: cobra.MinimumNArgs(0),
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("placement-host-address", cmd.Flags().Lookup("placement-host-address"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println(print.WhiteBold("WARNING: no application command found."))
		}

		if unixDomainSocket != "" {
			// TODO(@daixiang0): add Windows support.
			if runtime.GOOS == "windows" {
				print.FailureStatusEvent(os.Stderr, "The unix-domain-socket option is not supported on Windows")
				os.Exit(1)
			} else {
				// use unix domain socket means no port any more.
				print.WarningStatusEvent(os.Stdout, "Unix domain sockets are currently a preview feature")
				port = 0
				grpcPort = 0
			}
		}

		output, err := newOutput(&standalone.RunConfig{
			AppID:              appID,
			AppPort:            appPort,
			HTTPPort:           port,
			GRPCPort:           grpcPort,
			ConfigFile:         configFile,
			Arguments:          args,
			EnableProfiling:    enableProfiling,
			ProfilePort:        profilePort,
			LogLevel:           logLevel,
			MaxConcurrency:     maxConcurrency,
			Protocol:           protocol,
			PlacementHostAddr:  viper.GetString("placement-host-address"),
			ComponentsPath:     componentsPath,
			AppSSL:             appSSL,
			MetricsPort:        metricsPort,
			MaxRequestBodySize: maxRequestBodySize,
			HTTPReadBufferSize: readBufferSize,
			UnixDomainSocket:   unixDomainSocket,
			EnableAppHealth:    enableAppHealth,
			AppHealthPath:      appHealthPath,
			AppHealthInterval:  appHealthInterval,
			AppHealthTimeout:   appHealthTimeout,
			AppHealthThreshold: appHealthThreshold,
			EnableAPILogging:   enableAPILogging,
			InternalGRPCPort:   internalGRPCPort,
		})
		if err != nil {
			print.FailureStatusEvent(os.Stderr, err.Error())
			os.Exit(1)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		daprRunning := make(chan bool, 1)
		appRunning := make(chan bool, 1)

		go func() {
			var startInfo string
			if unixDomainSocket != "" {
				startInfo = fmt.Sprintf(
					"Starting Dapr with id %s. HTTP Socket: %v. gRPC Socket: %v.",
					output.AppID,
					utils.GetSocket(unixDomainSocket, output.AppID, "http"),
					utils.GetSocket(unixDomainSocket, output.AppID, "grpc"))
			} else {
				startInfo = fmt.Sprintf(
					"Starting Dapr with id %s. HTTP Port: %v. gRPC Port: %v",
					output.AppID,
					output.DaprHTTPPort,
					output.DaprGRPCPort)
			}
			print.InfoStatusEvent(os.Stdout, startInfo)

			output.DaprCMD.Stdout = os.Stdout
			output.DaprCMD.Stderr = os.Stderr

			err = output.DaprCMD.Start()
			if err != nil {
				print.FailureStatusEvent(os.Stderr, err.Error())
				os.Exit(1)
			}

			go func() {
				daprdErr := output.DaprCMD.Wait()

				if daprdErr != nil {
					output.DaprErr = daprdErr
					print.FailureStatusEvent(os.Stderr, "The daprd process exited with error code: %s", daprdErr.Error())
				} else {
					print.SuccessStatusEvent(os.Stdout, "Exited Dapr successfully")
				}
				sigCh <- os.Interrupt
			}()

			if appPort <= 0 {
				// If app does not listen to port, we can check for Dapr's sidecar health before starting the app.
				// Otherwise, it creates a deadlock.
				sidecarUp := true

				if unixDomainSocket != "" {
					httpSocket := utils.GetSocket(unixDomainSocket, output.AppID, "http")
					print.InfoStatusEvent(os.Stdout, "Checking if Dapr sidecar is listening on HTTP socket %v", httpSocket)
					err = utils.IsDaprListeningOnSocket(httpSocket, time.Duration(runtimeWaitTimeoutInSeconds)*time.Second)
					if err != nil {
						sidecarUp = false
						print.WarningStatusEvent(os.Stdout, "Dapr sidecar is not listening on HTTP socket: %s", err.Error())
					}

					grpcSocket := utils.GetSocket(unixDomainSocket, output.AppID, "grpc")
					print.InfoStatusEvent(os.Stdout, "Checking if Dapr sidecar is listening on GRPC socket %v", grpcSocket)
					err = utils.IsDaprListeningOnSocket(grpcSocket, time.Duration(runtimeWaitTimeoutInSeconds)*time.Second)
					if err != nil {
						sidecarUp = false
						print.WarningStatusEvent(os.Stdout, "Dapr sidecar is not listening on GRPC socket: %s", err.Error())
					}

				} else {
					print.InfoStatusEvent(os.Stdout, "Checking if Dapr sidecar is listening on HTTP port %v", output.DaprHTTPPort)
					err = utils.IsDaprListeningOnPort(output.DaprHTTPPort, time.Duration(runtimeWaitTimeoutInSeconds)*time.Second)
					if err != nil {
						sidecarUp = false
						print.WarningStatusEvent(os.Stdout, "Dapr sidecar is not listening on HTTP port: %s", err.Error())
					}

					print.InfoStatusEvent(os.Stdout, "Checking if Dapr sidecar is listening on GRPC port %v", output.DaprGRPCPort)
					err = utils.IsDaprListeningOnPort(output.DaprGRPCPort, time.Duration(runtimeWaitTimeoutInSeconds)*time.Second)
					if err != nil {
						sidecarUp = false
						print.WarningStatusEvent(os.Stdout, "Dapr sidecar is not listening on GRPC port: %s", err.Error())
					}
				}

				if sidecarUp {
					print.InfoStatusEvent(os.Stdout, "Dapr sidecar is up and running.")
				} else {
					print.WarningStatusEvent(os.Stdout, "Dapr sidecar might not be responding.")
				}
			}

			daprRunning <- true
		}()

		<-daprRunning

		go func() {
			if output.AppCMD == nil {
				appRunning <- true
				return
			}

			stdErrPipe, pipeErr := output.AppCMD.StderrPipe()
			if pipeErr != nil {
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error creating stderr for App: %s", err.Error()))
				appRunning <- false
				return
			}

			stdOutPipe, pipeErr := output.AppCMD.StdoutPipe()
			if pipeErr != nil {
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error creating stdout for App: %s", err.Error()))
				appRunning <- false
				return
			}

			errScanner := bufio.NewScanner(stdErrPipe)
			outScanner := bufio.NewScanner(stdOutPipe)
			go func() {
				for errScanner.Scan() {
					fmt.Println(print.Blue(fmt.Sprintf("== APP == %s", errScanner.Text())))
				}
			}()

			go func() {
				for outScanner.Scan() {
					fmt.Println(print.Blue(fmt.Sprintf("== APP == %s", outScanner.Text())))
				}
			}()

			err = output.AppCMD.Start()
			if err != nil {
				print.FailureStatusEvent(os.Stderr, err.Error())
				appRunning <- false
				return
			}

			go func() {
				appErr := output.AppCMD.Wait()

				if appErr != nil {
					output.AppErr = appErr
					print.FailureStatusEvent(os.Stderr, "The App process exited with error code: %s", appErr.Error())
				} else {
					print.SuccessStatusEvent(os.Stdout, "Exited App successfully")
				}
				sigCh <- os.Interrupt
			}()

			appRunning <- true
		}()

		appRunStatus := <-appRunning
		if !appRunStatus {
			// Start App failed, try to stop Dapr and exit.
			err = output.DaprCMD.Process.Kill()
			if err != nil {
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Start App failed, try to stop Dapr Error: %s", err))
			} else {
				print.SuccessStatusEvent(os.Stdout, "Start App failed, try to stop Dapr successfully")
			}
			os.Exit(1)
		}

		// Metadata API is only available if app has started listening to port, so wait for app to start before calling metadata API.
		err = metadata.Put(output.DaprHTTPPort, "cliPID", strconv.Itoa(os.Getpid()), appID, unixDomainSocket)
		if err != nil {
			print.WarningStatusEvent(os.Stdout, "Could not update sidecar metadata for cliPID: %s", err.Error())
		}

		if output.AppCMD != nil {
			appCommand := strings.Join(args, " ")
			print.InfoStatusEvent(os.Stdout, fmt.Sprintf("Updating metadata for app command: %s", appCommand))
			err = metadata.Put(output.DaprHTTPPort, "appCommand", appCommand, appID, unixDomainSocket)
			if err != nil {
				print.WarningStatusEvent(os.Stdout, "Could not update sidecar metadata for appCommand: %s", err.Error())
			} else {
				print.SuccessStatusEvent(os.Stdout, "You're up and running! Both Dapr and your app logs will appear here.\n")
			}
		} else {
			print.SuccessStatusEvent(os.Stdout, "You're up and running! Dapr logs will appear here.\n")
		}

		<-sigCh
		print.InfoStatusEvent(os.Stdout, "\nterminated signal received: shutting down")

		exitWithError := false

		if output.DaprErr != nil {
			exitWithError = true
			print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting Dapr: %s", output.DaprErr))
		} else if output.DaprCMD.ProcessState == nil || !output.DaprCMD.ProcessState.Exited() {
			err = output.DaprCMD.Process.Kill()
			if err != nil {
				exitWithError = true
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting Dapr: %s", err))
			} else {
				print.SuccessStatusEvent(os.Stdout, "Exited Dapr successfully")
			}
		}

		if output.AppErr != nil {
			exitWithError = true
			print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting App: %s", output.AppErr))
		} else if output.AppCMD != nil && (output.AppCMD.ProcessState == nil || !output.AppCMD.ProcessState.Exited()) {
			err = output.AppCMD.Process.Kill()
			if err != nil {
				exitWithError = true
				print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting App: %s", err))
			} else {
				print.SuccessStatusEvent(os.Stdout, "Exited App successfully")
			}
		}

		if unixDomainSocket != "" {
			for _, s := range []string{"http", "grpc"} {
				os.Remove(utils.GetSocket(unixDomainSocket, output.AppID, s))
			}
		}

		if exitWithError {
			os.Exit(1)
		}
	},
}

func getChannelCommand(config *standalone.RunConfig) (*exec.Cmd, error) {
	daprCMD := filepath.Join(daprRuntimePath, "probe")
	c := Config(*config)
	args := (&c).GetArgs()
	cmd := exec.Command(daprCMD, args...)
	fmt.Println(cmd)
	return cmd, nil
}

func getAppCommand(config *standalone.RunConfig) *exec.Cmd {
	argCount := len(config.Arguments)

	if argCount == 0 {
		return nil
	}
	command := config.Arguments[0]

	args := []string{}
	if argCount > 1 {
		args = config.Arguments[1:]
	}

	c := Config(*config)
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, (&c).getEnv()...)

	return cmd
}

func newOutput(config *standalone.RunConfig) (*standalone.RunOutput, error) {
	//nolint
	daprCMD, err := getChannelCommand(config)
	if err != nil {
		return nil, err
	}

	//nolint
	var appCMD *exec.Cmd = getAppCommand(config)
	return &standalone.RunOutput{
		DaprCMD:      daprCMD,
		DaprErr:      nil,
		AppCMD:       appCMD,
		AppErr:       nil,
		AppID:        config.AppID,
		DaprHTTPPort: config.HTTPPort,
		DaprGRPCPort: config.GRPCPort,
	}, nil
}

func (config *Config) GetArgs() []string {
	args := []string{}
	schema := reflect.ValueOf(*config)
	for i := 0; i < schema.NumField(); i++ {
		valueField := schema.Field(i).Interface()
		typeField := schema.Type().Field(i)
		key := typeField.Tag.Get("arg")
		if len(key) == 0 {
			continue
		}
		key = "--" + key

		ifneq, hasIfneq := typeField.Tag.Lookup("ifneq")

		switch valueField.(type) {
		case bool:
			if valueField == true {
				args = append(args, key)
			}
		default:
			value := fmt.Sprintf("%v", reflect.ValueOf(valueField))
			if len(value) != 0 && (!hasIfneq || value != ifneq) {
				args = append(args, key, value)
			}
		}
	}
	if print.IsJSONLogEnabled() {
		args = append(args, "--log-as-json")
	}

	return args
}

func (config *Config) getEnv() []string {
	env := []string{}
	schema := reflect.ValueOf(*config)
	for i := 0; i < schema.NumField(); i++ {
		valueField := schema.Field(i).Interface()
		typeField := schema.Type().Field(i)
		key := typeField.Tag.Get("env")
		if len(key) == 0 {
			continue
		}
		if value, ok := valueField.(int); ok && value <= 0 {
			// ignore unset numeric variables.
			continue
		}

		value := fmt.Sprintf("%v", reflect.ValueOf(valueField))
		env = append(env, fmt.Sprintf("%s=%v", key, value))
	}
	return env
}

func init() {
	RunCmd.Flags().IntVarP(&appPort, "app-port", "p", -1, "The port your application is listening on")
	RunCmd.Flags().StringVarP(&appID, "app-id", "a", "", "The id for your application, used for service discovery")
	RunCmd.Flags().StringVarP(&configFile, "config", "c", standalone.DefaultConfigFilePath(), "Dapr configuration file")
	RunCmd.Flags().IntVarP(&port, "dapr-http-port", "H", -1, "The HTTP port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&grpcPort, "dapr-grpc-port", "G", -1, "The gRPC port for Dapr to listen on")
	RunCmd.Flags().IntVarP(&internalGRPCPort, "dapr-internal-grpc-port", "I", 56471, "The gRPC port for the Dapr internal API to listen on")
	RunCmd.Flags().BoolVar(&enableProfiling, "enable-profiling", false, "Enable pprof profiling via an HTTP endpoint")
	RunCmd.Flags().IntVarP(&profilePort, "profile-port", "", -1, "The port for the profile server to listen on")
	RunCmd.Flags().StringVarP(&logLevel, "log-level", "", "info", "The log verbosity. Valid values are: debug, info, warn, error, fatal, or panic")
	RunCmd.Flags().IntVarP(&maxConcurrency, "app-max-concurrency", "", -1, "The concurrency level of the application, otherwise is unlimited")
	RunCmd.Flags().StringVarP(&protocol, "app-protocol", "P", "http", "The protocol (gRPC or HTTP) Dapr uses to talk to the application")
	RunCmd.Flags().StringVarP(&componentsPath, "components-path", "d", standalone.DefaultComponentsDirPath(), "The path for components directory")
	RunCmd.Flags().String("placement-host-address", "localhost", "The address of the placement service. Format is either <hostname> for default port or <hostname>:<port> for custom port")
	RunCmd.Flags().BoolVar(&appSSL, "app-ssl", false, "Enable https when Dapr invokes the application")
	RunCmd.Flags().IntVarP(&metricsPort, "metrics-port", "M", -1, "The port of metrics on dapr")
	RunCmd.Flags().BoolP("help", "h", false, "Print this help message")
	RunCmd.Flags().IntVarP(&maxRequestBodySize, "dapr-http-max-request-size", "", -1, "Max size of request body in MB")
	RunCmd.Flags().IntVarP(&readBufferSize, "dapr-http-read-buffer-size", "", -1, "HTTP header read buffer in KB")
	RunCmd.Flags().StringVarP(&unixDomainSocket, "unix-domain-socket", "u", "", "Path to a unix domain socket dir. If specified, Dapr API servers will use Unix Domain Sockets")
	RunCmd.Flags().BoolVar(&enableAppHealth, "enable-app-health-check", false, "Enable health checks for the application using the protocol defined with app-protocol")
	RunCmd.Flags().StringVar(&appHealthPath, "app-health-check-path", "", "Path used for health checks; HTTP only")
	RunCmd.Flags().IntVar(&appHealthInterval, "app-health-probe-interval", 0, "Interval to probe for the health of the app in seconds")
	RunCmd.Flags().IntVar(&appHealthTimeout, "app-health-probe-timeout", 0, "Timeout for app health probes in milliseconds")
	RunCmd.Flags().IntVar(&appHealthThreshold, "app-health-threshold", 0, "Number of consecutive failures for the app to be considered unhealthy")
	RunCmd.Flags().BoolVar(&enableAPILogging, "enable-api-logging", false, "Log API calls at INFO verbosity. Valid values are: true or false")

	RootCmd.AddCommand(RunCmd)
}
