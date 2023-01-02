/*
Copyright ApeCloud Inc.

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
	NewCUETpl("")
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

// This test shows that the omitempty tag should be used with care if the field
// is used in cue template.
func TestCUE(t *testing.T) {
	cueTplIntJson := `
input: {
	replicas:       int32
}
output: {
	replicas:       input.replicas
}
`
	cueTplBoolJson := `
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
		tpl:   cueTplIntJson,
		input: testCUEInput{Replicas: 0},
	}, {
		name:  "testCUEInputIntOmitEmptyWithNonZeroValue",
		tpl:   cueTplIntJson,
		input: testCUEInputIntOmitEmpty{Replicas: 1},
	}, {
		name:  "testCUEInputIntOmitEmpty",
		tpl:   cueTplIntJson,
		input: testCUEInputIntOmitEmpty{Replicas: 0},
		err:   "marshal error",
	}, {
		name:  "testCUEInputBoolOmitEmptyWithNonZeroValue",
		tpl:   cueTplBoolJson,
		input: testCUEInputBoolOmitEmpty{Flag: true},
	}, {
		name:  "testCUEInputBoolOmitEmpty",
		tpl:   cueTplBoolJson,
		input: testCUEInputBoolOmitEmpty{Flag: false},
		err:   "marshal error",
	}}

	for _, tc := range testCases {
		cueTpl := NewCUETpl(tc.tpl)
		cueValue := NewCUEBuilder(*cueTpl)

		if err := cueValue.FillObj("input", tc.input); err != nil {
			t.Error("Expected non-nil input error")
		}
		_, err := cueValue.Lookup("output")
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
