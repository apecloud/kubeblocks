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
	"path/filepath"
	"reflect"
	"time"

	"github.com/dapr/cli/pkg/print"
)

// Options represents the application configuration parameters.
type Options struct {
	HTTPPort           int    `env:"DAPR_HTTP_PORT" arg:"dapr-http-port"`
	GRPCPort           int    `env:"DAPR_GRPC_PORT" arg:"dapr-grpc-port"`
	ConfigFile         string `arg:"config"`
	Protocol           string `arg:"app-protocol"`
	Arguments          []string
	LogLevel           string `arg:"log-level"`
	ComponentsPath     string `arg:"components-path"`
	MetricsPort        int    `env:"DAPR_METRICS_PORT" arg:"metrics-port"`
	MaxRequestBodySize int    `arg:"dapr-http-max-request-size"`
	HTTPReadBufferSize int    `arg:"dapr-http-read-buffer-size"`
	InternalGRPCPort   int    `arg:"dapr-internal-grpc-port"`
	EnableAppHealth    bool   `arg:"enable-app-health-check"`
	AppHealthThreshold int    `arg:"app-health-threshold" ifneq:"0"`
}

func (options *Options) validate() error {
	if options.MaxRequestBodySize < 0 {
		options.MaxRequestBodySize = -1
	}

	if options.HTTPReadBufferSize < 0 {
		options.HTTPReadBufferSize = -1
	}

	return nil
}

func (options *Options) getArgs() []string {
	args := []string{"--app-id", "dbservice"}
	schema := reflect.ValueOf(*options)
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

	return args
}

func (options *Options) getEnv() []string {
	env := []string{}
	schema := reflect.ValueOf(*options)
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

// Commands represents the managed subprocesses.
type Commands struct {
	SQLChannelCMD      *exec.Cmd
	SQLChannelErr      error
	SQLChannelHTTPPort int
	SQLChannelGRPCPort int
	SQLChannelStarted  chan bool
	AppCMD             *exec.Cmd
	AppErr             error
	AppStarted         chan bool
	IsStopped          bool
	Options            *Options
}

func getSQLChannelCommand(options *Options) (*exec.Cmd, error) {
	sqlChannelCMD := filepath.Join(sqlChannelRuntimePath, "probe")
	args := options.getArgs()
	cmd := exec.Command(sqlChannelCMD, args...)
	return cmd, nil
}

func getAppCommand(options *Options) *exec.Cmd {
	argCount := len(options.Arguments)

	if argCount == 0 {
		return nil
	}
	command := options.Arguments[0]

	args := []string{}
	if argCount > 1 {
		args = options.Arguments[1:]
	}

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, options.getEnv()...)

	return cmd
}

func newCommands(options *Options) (*Commands, error) {
	//nolint
	err := options.validate()
	if err != nil {
		return nil, err
	}

	sqlChannelCMD, err := getSQLChannelCommand(options)
	if err != nil {
		return nil, err
	}

	//nolint
	var appCMD *exec.Cmd = getAppCommand(options)
	return &Commands{
		SQLChannelCMD:      sqlChannelCMD,
		SQLChannelErr:      nil,
		SQLChannelHTTPPort: options.HTTPPort,
		SQLChannelGRPCPort: options.GRPCPort,
		SQLChannelStarted:  make(chan bool, 1),
		AppCMD:             appCMD,
		AppErr:             nil,
		AppStarted:         make(chan bool, 1),
		Options:            options,
	}, nil
}

func (commands *Commands) RestartDBServiceIfExited() {
	for !commands.IsStopped {
		if commands.AppCMD == nil || (commands.AppCMD.ProcessState != nil && commands.AppCMD.ProcessState.Exited()) {
			commands.RestartDBService()
		}
		time.Sleep(1 * time.Second)
	}
}

func (commands *Commands) RestartDBService() {
	commands.StopDBService()
	commands.AppCMD = getAppCommand(commands.Options)
	commands.AppErr = nil
	commands.AppStarted = make(chan bool, 1)
	commands.StartDBService()

}

func (commands *Commands) StartDBService() {
	if commands.AppCMD == nil {
		commands.AppStarted <- true
		return
	}

	stdErrPipe, pipeErr := commands.AppCMD.StderrPipe()
	if pipeErr != nil {
		print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error creating stderr for DB Service: %s", pipeErr.Error()))
		commands.AppStarted <- false
		return
	}

	stdOutPipe, pipeErr := commands.AppCMD.StdoutPipe()
	if pipeErr != nil {
		print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error creating stdout for DB Service: %s", pipeErr.Error()))
		commands.AppStarted <- false
		return
	}

	errScanner := bufio.NewScanner(stdErrPipe)
	outScanner := bufio.NewScanner(stdOutPipe)
	go func() {
		for errScanner.Scan() {
			fmt.Println(print.Blue(fmt.Sprintf("== DB Service == %s", errScanner.Text())))
		}
		fmt.Println("== scanner exited == ")

	}()

	go func() {
		for outScanner.Scan() {
			fmt.Println(print.Blue(fmt.Sprintf("== DB Service == %s", outScanner.Text())))
		}
		fmt.Println("== scanner exited == ")
	}()

	err := commands.AppCMD.Start()
	if err != nil {
		print.FailureStatusEvent(os.Stderr, err.Error())
		commands.AppStarted <- false
		return
	}
	commands.AppStarted <- true

	go commands.WaitDBService()
}

func (commands *Commands) StopSQLChannel() bool {
	exitWithError := false
	if commands.SQLChannelErr != nil {
		exitWithError = true
		print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting SQLChannel: %s", commands.SQLChannelErr))
	} else if commands.SQLChannelCMD.ProcessState == nil || !commands.SQLChannelCMD.ProcessState.Exited() {
		err := commands.SQLChannelCMD.Process.Kill()
		if err != nil {
			exitWithError = true
			print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting SQLChannel: %s", err))
		} else {
			print.SuccessStatusEvent(os.Stdout, "Exited SQLChannel successfully")
		}
	}
	return exitWithError
}

func (commands *Commands) StopDBService() bool {
	exitWithError := false
	if commands.AppErr != nil {
		exitWithError = true
		print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting App: %s", commands.AppErr))
	} else if commands.AppCMD != nil && (commands.AppCMD.ProcessState == nil || !commands.AppCMD.ProcessState.Exited()) {
		err := commands.AppCMD.Process.Kill()
		if err != nil {
			exitWithError = true
			print.FailureStatusEvent(os.Stderr, fmt.Sprintf("Error exiting App: %s", err))
		} else {
			print.SuccessStatusEvent(os.Stdout, "Exited App successfully")
		}
	}
	commands.WaitDBService()
	return exitWithError
}

func (commands *Commands) WaitDBService() {
	if commands.AppCMD == nil || (commands.AppCMD.ProcessState != nil && commands.AppCMD.ProcessState.Exited()) {
		return
	}

	appErr := commands.AppCMD.Wait()

	if appErr != nil {
		commands.AppErr = appErr
		print.FailureStatusEvent(os.Stderr, "The DB Service exited with error code: %s", appErr.Error())
	} else {
		print.SuccessStatusEvent(os.Stdout, "The DB Service exited")
	}
}
