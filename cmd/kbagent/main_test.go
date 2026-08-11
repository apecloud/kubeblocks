/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestReexecUnderInitSkipsNonPID1(t *testing.T) {
	called := false
	err := reexecUnderInit(2, []string{"/bin/kbagent"}, nil,
		func() (string, error) {
			called = true
			return "", nil
		},
		func(string, []string, []string) error {
			called = true
			return nil
		})
	if err != nil || called {
		t.Fatalf("reexecUnderInit() = %v, called = %v", err, called)
	}
}

func TestReexecUnderInitPreservesArgumentsAndEnvironment(t *testing.T) {
	tests := []struct {
		name string
		exe  string
	}{
		{name: "tools image", exe: "/bin/kbagent"},
		{name: "custom action image", exe: "/kubeblocks/kbagent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{tt.exe, "--port", "3501", "--server=false"}
			env := []string{"A=B", "C=D"}
			var gotPath string
			var gotArgs, gotEnv []string
			err := reexecUnderInit(1, args, env,
				func() (string, error) { return tt.exe, nil },
				func(path string, argv, envv []string) error {
					gotPath = path
					gotArgs = append([]string(nil), argv...)
					gotEnv = append([]string(nil), envv...)
					return nil
				})
			if err != nil {
				t.Fatalf("reexecUnderInit() error = %v", err)
			}
			wantPath := tt.exe[:len(tt.exe)-len("kbagent")] + tiniBinaryName
			wantArgs := []string{wantPath, "--", tt.exe, "--port", "3501", "--server=false"}
			if gotPath != wantPath || !reflect.DeepEqual(gotArgs, wantArgs) || !reflect.DeepEqual(gotEnv, env) {
				t.Fatalf("exec = (%q, %#v, %#v), want (%q, %#v, %#v)", gotPath, gotArgs, gotEnv, wantPath, wantArgs, env)
			}
		})
	}
}

func TestReexecUnderInitReturnsStartupErrors(t *testing.T) {
	resolveErr := errors.New("resolve")
	err := reexecUnderInit(1, []string{"kbagent"}, nil,
		func() (string, error) { return "", resolveErr },
		func(string, []string, []string) error { return nil })
	if !errors.Is(err, resolveErr) {
		t.Fatalf("expected executable error, got %v", err)
	}

	execErr := errors.New("exec")
	err = reexecUnderInit(1, []string{"/bin/kbagent"}, nil,
		func() (string, error) { return "/bin/kbagent", nil },
		func(string, []string, []string) error { return execErr })
	if !errors.Is(err, execErr) {
		t.Fatalf("expected exec error, got %v", err)
	}
}
