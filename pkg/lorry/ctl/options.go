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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"time"
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
	InternalGRPCPort   int    `arg:"dapr-internal-grpc-port"`
	EnableAppHealth    bool   `arg:"enable-app-health-check"`
	AppHealthThreshold int    `arg:"app-health-threshold" ifneq:"0"`
}

func (options *Options) validate() error {
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
	ctx           context.Context
	WaitGroup     sync.WaitGroup
	LorryCMD      *exec.Cmd
	LorryErr      error
	LorryHTTPPort int
	LorryGRPCPort int
	LorryStarted  chan bool
	AppCMD        *exec.Cmd
	AppErr        error
	AppStarted    chan bool
	Options       *Options
	SigCh         chan os.Signal
	// if it's false, lorryctl will not restart db service
	restartDB bool
}

func getLorryCommand(options *Options) (*exec.Cmd, error) {
	lorryCMD := filepath.Join(lorryRuntimePath, "lorry")
	args := options.getArgs()
	cmd := exec.Command(lorryCMD, args...)
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

func newCommands(ctx context.Context, options *Options) (*Commands, error) {
	//nolint
	err := options.validate()
	if err != nil {
		return nil, err
	}

	lorryCMD, err := getLorryCommand(options)
	if err != nil {
		return nil, err
	}

	//nolint
	var appCMD *exec.Cmd = getAppCommand(options)
	cmd := &Commands{
		ctx:           ctx,
		LorryCMD:      lorryCMD,
		LorryErr:      nil,
		LorryHTTPPort: options.HTTPPort,
		LorryGRPCPort: options.GRPCPort,
		LorryStarted:  make(chan bool, 1),
		AppCMD:        appCMD,
		AppErr:        nil,
		AppStarted:    make(chan bool, 1),
		Options:       options,
		SigCh:         make(chan os.Signal, 1),
		restartDB:     true,
	}
	return cmd, nil
}

func (commands *Commands) StartLorry() {
	var startInfo string
	fmt.Fprintf(os.Stdout, "Starting Lorry HTTP Port: %v. gRPC Port: %v\n",
		commands.LorryHTTPPort,
		commands.LorryGRPCPort)
	fmt.Fprintln(os.Stdout, startInfo)

	commands.LorryCMD.Stdout = os.Stdout
	commands.LorryCMD.Stderr = os.Stderr

	err := commands.LorryCMD.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	commands.LorryStarted <- true
}

func (commands *Commands) RestartDBServiceIfExited() {
	if commands.AppCMD == nil {
		return
	}
	commands.WaitGroup.Add(1)
	for {
		select {
		case <-commands.ctx.Done():
			commands.WaitGroup.Done()
			return
		default:
		}

		if commands.AppCMD.ProcessState != nil {
			Printf("DB service exits: %v\n", commands.AppCMD.ProcessState)
			if !commands.restartDB {
				Printf("restart DB service: %v\n", commands.restartDB)
				time.Sleep(2 * time.Second)
				continue
			}

			status, ok := commands.AppCMD.ProcessState.Sys().(syscall.WaitStatus)
			if commands.AppCMD.ProcessState.Exited() || (ok && status.Signaled()) {
				commands.RestartDBService()
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func (commands *Commands) RestartDBService() {
	if commands.AppCMD == nil {
		return
	}
	Printf("DB service restart: %v\n", commands.AppCMD)
	// commands.StopDBService()
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
		fmt.Fprintf(os.Stderr, "Error creating stderr for DB Service: %s\n", pipeErr.Error())
		commands.AppStarted <- false
		return
	}

	stdOutPipe, pipeErr := commands.AppCMD.StdoutPipe()
	if pipeErr != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout for DB Service: %s\n", pipeErr.Error())
		commands.AppStarted <- false
		return
	}

	errScanner := bufio.NewScanner(stdErrPipe)
	outScanner := bufio.NewScanner(stdOutPipe)
	go func() {
		for errScanner.Scan() {
			fmt.Printf("== DB Service err == %s\n", errScanner.Text())
		}
	}()

	go func() {
		for outScanner.Scan() {
			fmt.Printf("== DB Service == %s\n", outScanner.Text())
		}
	}()

	err := commands.AppCMD.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		commands.AppStarted <- false
		return
	}
	commands.AppStarted <- true
	go commands.WaitDBService()
}

func (commands *Commands) StopLorry() bool {
	exitWithError := false
	if commands.LorryErr != nil {
		exitWithError = true
		fmt.Fprintf(os.Stderr, "Error exiting Lorry: %s\n", commands.LorryErr)
	} else if commands.LorryCMD.ProcessState == nil || !commands.LorryCMD.ProcessState.Exited() {
		err := commands.LorryCMD.Process.Signal(syscall.SIGTERM)
		if err != nil {
			exitWithError = true
			fmt.Fprintf(os.Stderr, "Error exiting Lorry: %s\n", err)
		} else {
			fmt.Fprintln(os.Stdout, "Send SIGTERM to Lorry")
		}
	}
	// state, err = commands.LorryCMD.Process.Wait()
	// fmt.Printf("state: %v, err: %v\n", state, err)
	commands.WaitLorry()
	return exitWithError
}

func (commands *Commands) StopDBService() bool {
	exitWithError := false
	if commands.AppErr != nil {
		exitWithError = true
		fmt.Fprintf(os.Stderr, "Error exiting App: %s\n", commands.AppErr)
	} else if commands.AppCMD != nil && (commands.AppCMD.ProcessState == nil || !commands.AppCMD.ProcessState.Exited()) {
		err := commands.AppCMD.Process.Signal(syscall.SIGTERM)
		if err != nil {
			exitWithError = true
			fmt.Fprintf(os.Stderr, "Error exiting App: %s\n", err)
		} else {
			fmt.Fprintln(os.Stdout, "Exited App successfully")
		}
	}
	commands.WaitDBService()
	return exitWithError
}

func (commands *Commands) WaitDBService() {
	commands.AppErr = waitCmd(commands.AppCMD)
}

func (commands *Commands) WaitLorry() {
	commands.LorryErr = waitCmd(commands.LorryCMD)
}

func waitCmd(cmd *exec.Cmd) error {
	if cmd == nil || (cmd.ProcessState != nil && cmd.ProcessState.Exited()) {
		return nil
	}

	err := cmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "The command [%s] exited with error code: %s\n", cmd.String(), err.Error())
	} else {
		fmt.Fprintf(os.Stdout, "The command [%s] exited\n", cmd.String())
	}
	return err
}
