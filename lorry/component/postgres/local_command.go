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

package postgres

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

type execCommand struct {
	*exec.Cmd
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

func NewExecCommander(name string, args ...string) LocalCommand {
	execCmd := exec.Command(name, args...)

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	return &execCommand{
		Cmd:    execCmd,
		Stdout: &stdout,
		Stderr: &stderr,
	}
}

func (cmd *execCommand) GetStdout() *bytes.Buffer {
	return cmd.Stdout
}

func (cmd *execCommand) GetStderr() *bytes.Buffer {
	return cmd.Stderr
}

var LocalCommander = NewExecCommander

func ExecCommand(name string, args ...string) (string, error) {
	cmd := LocalCommander(name, args...)

	err := cmd.Run()
	if err != nil || cmd.GetStderr().String() != "<nil>" {
		return "", errors.Errorf("exec command %s failed, err:%v, stderr:%s", name, err, cmd.GetStderr().String())
	}

	return cmd.GetStdout().String(), nil
}

func Psql(args ...string) (string, error) {
	return ExecCommand("psql", args...)
}

func PgCtl(arg string) (string, error) {
	args := []string{"-c"}
	args = append(args, "pg_ctl "+arg)
	args = append(args, "postgres")

	return ExecCommand("su", args...)
}

func PgWalDump(args ...string) (string, error) {
	return ExecCommand("pg_waldump", args...)
}

func PgRewind(args ...string) (string, error) {
	return ExecCommand("pg_rewind", args...)
}

type FakeCommand struct {
	Stdout     *bytes.Buffer
	Stderr     *bytes.Buffer
	RunnerFunc func() error
}

func NewFakeCommander(f func() error, stdout, stderr *bytes.Buffer) func(name string, args ...string) LocalCommand {
	return func(name string, args ...string) LocalCommand {
		return &FakeCommand{
			RunnerFunc: f,
			Stdout:     stdout,
			Stderr:     stderr,
		}
	}
}

func (cmd *FakeCommand) Run() error {
	return cmd.RunnerFunc()
}

func (cmd *FakeCommand) GetStdout() *bytes.Buffer {
	return cmd.Stdout
}

func (cmd *FakeCommand) GetStderr() *bytes.Buffer {
	return cmd.Stderr
}
