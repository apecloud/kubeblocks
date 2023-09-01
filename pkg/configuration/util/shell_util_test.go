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

package util

import (
	"testing"
)

func TestExecShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     []string
		want    string
		wantErr bool
	}{{
		name:    "go_test",
		cmd:     []string{"go", "env"},
		want:    "",
		wantErr: false,
	}, {
		name:    "failed_test",
		cmd:     []string{"not_command"},
		want:    "",
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunShellCommand(tt.cmd[0], tt.cmd[1:]...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecShellCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("ExecShellCommand() got = %v, want %v", got, tt.want)
			}
		})
	}
}
