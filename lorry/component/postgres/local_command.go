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
}

func (cmd *execCommand) BindStdout(stdout *bytes.Buffer) {
	cmd.Stdout = stdout
}

func (cmd *execCommand) BindStdErr(stderr *bytes.Buffer) {
	cmd.Stderr = stderr
}

func newExecCommander(name string, args ...string) LocalCommand {
	execCmd := exec.Command(name, args...)
	return &execCommand{Cmd: execCmd}
}

var localCommander = newExecCommander

func ExecCommand(name string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := localCommander(name, args...)
	cmd.BindStdout(&stdout)
	cmd.BindStdErr(&stderr)

	err := cmd.Run()
	if err != nil || stderr.String() != "" {
		return "", errors.Errorf("exec command %s failed, err:%v, stderr:%s", name, err, stderr.String())
	}

	return stdout.String(), nil
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
