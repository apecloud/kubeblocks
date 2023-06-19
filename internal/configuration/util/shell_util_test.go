/*
Copyright ApeCloud, Inc.

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
