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

package controllerutil

import (
	"strings"
	"testing"
)

func TestNewCUETplFromPath(t *testing.T) {
	_, err := NewCUETplFromPath("cue.cue")
	if err == nil {
		t.Error("Expected error to fall through, got err")
	}
}

func TestNewCUETplFromBytes(t *testing.T) {
	_, err := NewCUETplFromBytes([]byte(""), nil)
	if err != nil {
		t.Error("Expected error to fall through, got nil")
	}
}

func TestNewCUETpl(t *testing.T) {
	NewCUETpl([]byte{})
}

type testCUEInput struct {
	Replicas int `json:"replicas"`
}

type testCUEInputIntOmitEmpty struct {
	Replicas int `json:"replicas,omitempty"`
}

type testCUEInputBoolOmitEmpty struct {
	Flag bool `json:"flag,omitempty"`
}

// This test shows that the omitempty tag should be used with much care if the field
// is used in cue template.
func TestCUE(t *testing.T) {
	cueTplIntJSON := `
input: {
	replicas:       int32
}
output: {
	replicas:       input.replicas
}
`
	cueTplBoolJSON := `
input: {
	flag:       bool
}
output: {
	flag:       input.flag
}
`

	testCases := []struct {
		name  string
		tpl   string
		input any
		err   string
	}{{
		name:  "testCUEInput",
		tpl:   cueTplIntJSON,
		input: testCUEInput{Replicas: 0},
	}, {
		name:  "testCUEInputIntOmitEmptyWithNonZeroValue",
		tpl:   cueTplIntJSON,
		input: testCUEInputIntOmitEmpty{Replicas: 1},
	}, {
		name:  "testCUEInputIntOmitEmpty",
		tpl:   cueTplIntJSON,
		input: testCUEInputIntOmitEmpty{Replicas: 0},
		err:   "marshal error",
	}, {
		name:  "testCUEInputBoolOmitEmptyWithNonZeroValue",
		tpl:   cueTplBoolJSON,
		input: testCUEInputBoolOmitEmpty{Flag: true},
	}, {
		name:  "testCUEInputBoolOmitEmpty",
		tpl:   cueTplBoolJSON,
		input: testCUEInputBoolOmitEmpty{Flag: false},
		err:   "marshal error",
	}}

	for _, tc := range testCases {
		cueTpl := NewCUETpl([]byte(tc.tpl))
		cueValue := NewCUEBuilder(*cueTpl)

		if err := cueValue.FillObj("input", tc.input); err != nil {
			t.Error("Expected non-nil input error")
		}
		_, err := cueValue.Lookup("output")
		checkErr(t, err, tc.err, tc.name)
	}
}

func TestCUEFillObj(t *testing.T) {
	cueTplIntJSON := `
	input: {
		replicas:       int32
	}
	output: {
		replicas:       input.replicas
	}
	`

	testCases := []struct {
		name  string
		tpl   string
		input any
		err   string
	}{
		{
			name:  "testCUEInvalidInput",
			tpl:   cueTplIntJSON,
			input: make(chan int),
			err:   "unsupported type",
		},
		{
			name:  "testCUEInput",
			tpl:   cueTplIntJSON,
			input: testCUEInput{Replicas: 0},
		},
	}

	for _, tc := range testCases {
		cueTpl := NewCUETpl([]byte(tc.tpl))
		cueValue := NewCUEBuilder(*cueTpl)

		err := cueValue.FillObj("input", tc.input)
		checkErr(t, err, tc.err, tc.name)
	}
}

func TestCUEFill(t *testing.T) {
	cueTplIntJSON := `
	input: {
		replicas:       int32
	}
	output: {
		replicas:       input.replicas
	}
	`

	testCases := []struct {
		name  string
		tpl   string
		input string
		err   string
	}{
		{
			name:  "testCUEInvalidJSON",
			tpl:   cueTplIntJSON,
			input: "",
			err:   "invalid JSON",
		},
		{
			name:  "testCUEInput",
			input: `{ "replicas": 0}`,
			tpl:   cueTplIntJSON,
		},
	}

	for _, tc := range testCases {
		cueTpl := NewCUETpl([]byte(tc.tpl))
		cueValue := NewCUEBuilder(*cueTpl)

		err := cueValue.Fill("input", []byte(tc.input))
		checkErr(t, err, tc.err, tc.name)
	}
}

func checkErr(t *testing.T, err error, str, name string) bool {
	t.Helper()
	if err == nil {
		if str != "" {
			t.Errorf(`err:%s: got ""; want %q`, name, str)
		}
		return true
	}
	return checkFailed(t, err, str, name)
}

func checkFailed(t *testing.T, err error, str, name string) bool {
	t.Helper()
	if err != nil {
		got := err.Error()
		if str == "" {
			t.Fatalf(`err:%s: got %q; want ""`, name, got)
		}
		if !strings.Contains(got, str) {
			t.Errorf(`err:%s: got %q; want %q`, name, got, str)
		}
		return false
	}
	return true
}
