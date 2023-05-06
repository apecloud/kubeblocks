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

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestPluginPathsAreUnaltered(t *testing.T) {
	tempDir1, err := os.MkdirTemp(os.TempDir(), "test-cmd-plugins1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tempDir2, err := os.MkdirTemp(os.TempDir(), "test-cmd-plugins2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cleanup
	defer func() {
		if err := os.RemoveAll(tempDir1); err != nil {
			panic(fmt.Errorf("unexpected cleanup error: %v", err))
		}
		if err := os.RemoveAll(tempDir2); err != nil {
			panic(fmt.Errorf("unexpected cleanup error: %v", err))
		}
	}()

	ioStreams, _, _, errOut := genericclioptions.NewTestIOStreams()
	verifier := newFakePluginPathVerifier()
	pluginPaths := []string{tempDir1, tempDir2}
	o := &PluginListOptions{
		Verifier:  verifier,
		IOStreams: ioStreams,

		PluginPaths: pluginPaths,
	}

	// write at least one valid plugin file
	if _, err := os.CreateTemp(tempDir1, "kbcli-"); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if _, err := os.CreateTemp(tempDir2, "kubectl-"); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if err := o.Run(); err != nil {
		t.Fatalf("unexpected error %v - %v", err, errOut.String())
	}

	// ensure original paths remain unaltered
	if len(verifier.seenUnsorted) != len(pluginPaths) {
		t.Fatalf("saw unexpected plugin paths. Expecting %v, got %v", pluginPaths, verifier.seenUnsorted)
	}
	for actual := range verifier.seenUnsorted {
		if !strings.HasPrefix(verifier.seenUnsorted[actual], pluginPaths[actual]) {
			t.Fatalf("expected PATH slice to be unaltered. Expecting %v, but got %v", pluginPaths[actual], verifier.seenUnsorted[actual])
		}
	}
}

func TestPluginPathsAreValid(t *testing.T) {
	tempDir, err := os.MkdirTemp(os.TempDir(), "test-cmd-plugins")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cleanup
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			panic(fmt.Errorf("unexpected cleanup error: %v", err))
		}
	}()

	tc := []struct {
		name               string
		pluginPaths        []string
		pluginFile         func() (*os.File, error)
		verifier           *fakePluginPathVerifier
		expectVerifyErrors []error
		expectErr          string
		expectErrOut       string
		expectOut          string
	}{
		{
			name:        "ensure no plugins found if no files begin with kubectl- prefix",
			pluginPaths: []string{tempDir},
			verifier:    newFakePluginPathVerifier(),
			pluginFile: func() (*os.File, error) {
				return os.CreateTemp(tempDir, "notkbcli-")
			},
			expectErr: "error: unable to find any kbcli or kubectl plugins in your PATH\n",
			expectOut: "NAME",
		},
		{
			name:        "ensure de-duplicated plugin-paths slice",
			pluginPaths: []string{tempDir, tempDir},
			verifier:    newFakePluginPathVerifier(),
			pluginFile: func() (*os.File, error) {
				return os.CreateTemp(tempDir, "kbcli-")
			},
			expectOut: "NAME",
		},
		{
			name:        "ensure no errors when empty string or blank path are specified",
			pluginPaths: []string{tempDir, "", " "},
			verifier:    newFakePluginPathVerifier(),
			pluginFile: func() (*os.File, error) {
				return os.CreateTemp(tempDir, "kbcli-")
			},
			expectOut: "NAME",
		},
	}

	for _, test := range tc {
		t.Run(test.name, func(t *testing.T) {
			ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams()
			o := &PluginListOptions{
				Verifier:  test.verifier,
				IOStreams: ioStreams,

				PluginPaths: test.pluginPaths,
			}

			// create files
			if test.pluginFile != nil {
				if _, err := test.pluginFile(); err != nil {
					t.Fatalf("unexpected error creating plugin file: %v", err)
				}
			}

			for _, expected := range test.expectVerifyErrors {
				for _, actual := range test.verifier.errors {
					if expected != actual {
						t.Fatalf("unexpected error: expected %v, but got %v", expected, actual)
					}
				}
			}

			err := o.Run()
			switch {
			case err == nil && len(test.expectErr) > 0:
				t.Fatalf("unexpected non-error: expected %v, but got nothing", test.expectErr)
			case err != nil && len(test.expectErr) == 0:
				t.Fatalf("unexpected error: expected nothing, but got %v", err.Error())
			case err != nil && err.Error() != test.expectErr:
				t.Fatalf("unexpected error: expected %v, but got %v", test.expectErr, err.Error())
			}

			if len(test.expectErrOut) == 0 && errOut.Len() > 0 {
				t.Fatalf("unexpected error output: expected nothing, but got %v", errOut.String())
			} else if len(test.expectErrOut) > 0 && !strings.Contains(errOut.String(), test.expectErrOut) {
				t.Fatalf("unexpected error output: expected to contain %v, but got %v", test.expectErrOut, errOut.String())
			}

			if len(test.expectOut) > 0 && !strings.Contains(out.String(), test.expectOut) {
				t.Fatalf("unexpected output: expected to contain %v, but got %v", test.expectOut, out.String())
			}
		})
	}
}

func TestListPlugins(t *testing.T) {
	pluginPath, _ := filepath.Abs("./testdata")
	expectPlugins := []string{
		filepath.Join(pluginPath, "kbcli-foo"),
		filepath.Join(pluginPath, "kbcli-version"),
		filepath.Join(pluginPath, "kubectl-foo"),
		filepath.Join(pluginPath, "kubectl-version"),
	}

	verifier := newFakePluginPathVerifier()
	ioStreams, _, _, _ := genericclioptions.NewTestIOStreams()
	pluginPaths := []string{pluginPath}

	o := &PluginListOptions{
		Verifier:  verifier,
		IOStreams: ioStreams,

		PluginPaths: pluginPaths,
	}

	plugins, errs := o.ListPlugins()
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if !reflect.DeepEqual(expectPlugins, plugins) {
		t.Fatalf("saw unexpected plugins. Expecting %v, got %v", expectPlugins, plugins)
	}
}

type duplicatePathError struct {
	path string
}

func (d *duplicatePathError) Error() string {
	return fmt.Sprintf("path %q already visited", d.path)
}

type fakePluginPathVerifier struct {
	errors       []error
	seen         map[string]bool
	seenUnsorted []string
}

func (f *fakePluginPathVerifier) Verify(path string) []error {
	if f.seen[path] {
		err := &duplicatePathError{path}
		f.errors = append(f.errors, err)
		return []error{err}
	}
	f.seen[path] = true
	f.seenUnsorted = append(f.seenUnsorted, path)
	return nil
}

func newFakePluginPathVerifier() *fakePluginPathVerifier {
	return &fakePluginPathVerifier{seen: make(map[string]bool)}
}
