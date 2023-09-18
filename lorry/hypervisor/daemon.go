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

package hypervisor

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
)

// Daemon represents the managed DB process.
type Daemon struct {
	Cmd  string
	Args []string

	Stopped bool
	Pid     int
	Stdout  *os.File
	Stderr  *os.File
	Status  *os.ProcessState
	Process *os.Process
	Logger  logr.Logger
}

func NewDaemon(args []string, logger logr.Logger) (*Daemon, error) {
	argCount := len(args)

	if argCount == 0 {
		return nil, nil
	}
	command := args[0]
	command, err := exec.LookPath(command)
	if err != nil {
		return nil, err
	}

	if argCount > 1 {
		args = args[1:]
	}

	stdoutPipe, stdout, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outScanner := bufio.NewScanner(stdoutPipe)

	go func() {
		for outScanner.Scan() {
			fmt.Printf("== DB == %s\n", outScanner.Text())
		}
	}()

	stderrPipe, stderr, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	errScanner := bufio.NewScanner(stderrPipe)

	go func() {
		for errScanner.Scan() {
			fmt.Printf("== DB ERR == %s\n", errScanner.Text())
		}
	}()

	daemon := &Daemon{
		Cmd:    command,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
		Logger: logger,
	}

	return daemon, nil
}

func (daemon *Daemon) Start() error {
	wd, _ := os.Getwd()
	procAtr := &os.ProcAttr{
		Dir: wd,
		Env: os.Environ(),
		Files: []*os.File{
			os.Stdin,
			daemon.Stdout,
			daemon.Stderr,
		},
	}
	args := append([]string{daemon.Cmd}, daemon.Args...)
	daemon.Logger.Info("Start DB Service", "command", daemon)
	process, err := os.StartProcess(daemon.Cmd, args, procAtr)
	if err != nil {
		daemon.Logger.Error(err, "Start DB Service failed")
		return err
	}
	daemon.Process = process
	return nil
}

func (daemon *Daemon) IsAlive() bool {
	if daemon.Process == nil {
		return false
	}

	p, err := os.FindProcess(daemon.Process.Pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func (daemon *Daemon) ForceStop() error {
	daemon.Stopped = true
	if daemon.Process != nil {
		err := daemon.Process.Signal(syscall.SIGKILL)
		return err
	}
	return errors.New("process not exist")
}

func (daemon *Daemon) Stop() error {
	daemon.Stopped = true
	if daemon.Process != nil {
		err := daemon.Process.Signal(syscall.SIGTERM)
		return err
	}
	return errors.New("process not exist")
}

func (daemon *Daemon) Wait() (*os.ProcessState, error) {
	return daemon.Process.Wait()
}

func (daemon *Daemon) String() string {
	return daemon.Cmd + " " + strings.Join(daemon.Args, " ")
}
