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

package hypevisor

import (
	"bufio"
	"fmt"
	"os"
)

// Daemon represents the managed subprocesses.
type Daemon struct {
	Cmd  string
	Args []string

	Stopped bool
	Pid     int
	Stdout  *os.File
	Stderr  *os.File
	Status  *os.ProcessState
	process *os.Process
}

func NewDeamon(args []string) (*Daemon, error) {
	argCount := len(args)

	if argCount == 0 {
		return nil, nil
	}
	command := args[0]

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
			fmt.Printf("== DB Service == %s\n", outScanner.Text())
		}
	}()

	stderrPipe, stderr, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	errScanner := bufio.NewScanner(stderrPipe)

	go func() {
		for errScanner.Scan() {
			fmt.Printf("== DB Service ERR == %s\n", errScanner.Text())
		}
	}()

	daemon := &Daemon{
		Cmd:    command,
		Args:   args,
		Stdout: stdout,
		Stderr: stderr,
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
	process, err := os.StartProcess(daemon.Cmd, args, procAtr)
	if err != nil {
		return err
	}
	daemon.process = process
	return nil
}
