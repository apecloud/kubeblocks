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

package plugin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	pluginListExample = templates.Examples(`
	# List all available plugins file on a user's PATH.
	kbcli plugin list
	`)

	ValidPluginFilenamePrefixes = []string{"kbcli", "kubectl"}
)

func NewPluginCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Provides utilities for interacting with plugins.",
		Run:   cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	cmd.AddCommand(NewPluginListCmd(streams))
	return cmd
}

type PluginListOptions struct {
	Verifier PathVerifier
	NameOnly bool

	PluginPaths []string

	genericclioptions.IOStreams
}

func NewPluginListCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &PluginListOptions{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:                   "list",
		DisableFlagsInUseLine: true,
		Short:                 "List all visible plugin executables on a user's PATH",
		Example:               pluginListExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().BoolVar(&o.NameOnly, "name-only", o.NameOnly, "If true, display only the binary name of each plugin, rather than its full path.")
	return cmd
}

func (o *PluginListOptions) Complete(cmd *cobra.Command) error {
	o.Verifier = &CommandOverrideVerifier{
		root:        cmd.Root(),
		seenPlugins: map[string]string{},
	}

	o.PluginPaths = filepath.SplitList(os.Getenv("PATH"))
	return nil
}

func (o *PluginListOptions) Run() error {
	plugins, pluginErrors := o.ListPlugins()

	if len(plugins) > 0 {
		fmt.Fprintf(o.Out, "The following compatible plugins are available:\n\n")
	} else {
		pluginErrors = append(pluginErrors, fmt.Errorf("error: unable to find any kbcli or kubectl plugins in your PATH"))
	}

	pluginWarnings := 0
	for _, pluginPath := range plugins {
		if o.NameOnly {
			fmt.Fprintf(o.Out, "%s\n", filepath.Base(pluginPath))
		} else {
			fmt.Fprintf(o.Out, "%s\n", pluginPath)
		}
		if errs := o.Verifier.Verify(pluginPath); len(errs) != 0 {
			for _, err := range errs {
				fmt.Fprintf(o.ErrOut, " - %s\n", err)
				pluginWarnings++
			}
		}
	}

	if pluginWarnings > 0 {
		if pluginWarnings == 1 {
			pluginErrors = append(pluginErrors, fmt.Errorf("error: one plugin warining was found"))
		} else {
			pluginErrors = append(pluginErrors, fmt.Errorf("error: %d plugin warnings were found", pluginWarnings))
		}
	}
	if len(pluginErrors) > 0 {
		errs := bytes.NewBuffer(nil)
		for _, e := range pluginErrors {
			fmt.Fprintln(errs, e)
		}
		return fmt.Errorf("%s", errs.String())
	}

	return nil
}

func (o *PluginListOptions) ListPlugins() ([]string, []error) {
	var plugins []string
	var errors []error

	for _, dir := range uniquePathsList(o.PluginPaths) {
		if len(strings.TrimSpace(dir)) == 0 {
			continue
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				fmt.Fprintf(o.ErrOut, "Unable read directory %q from your PATH: %v. Skipping...\n", dir, err)
				continue
			}

			errors = append(errors, fmt.Errorf("error: unable to read directory %q in your PATH: %v", dir, err))
			continue
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !hasValidPrefix(f.Name(), ValidPluginFilenamePrefixes) {
				continue
			}

			plugins = append(plugins, filepath.Join(dir, f.Name()))
		}
	}

	return plugins, errors
}

// PathVerifier receives a path and determines if it is valid or not.
type PathVerifier interface {
	Verify(path string) []error
}

type CommandOverrideVerifier struct {
	root        *cobra.Command
	seenPlugins map[string]string
}

// Verify implements PathVerifier and determines if a given path
// is valid depending on whether it overwrites an existing
// kbcli command path, or a previously seen plugin.
func (v *CommandOverrideVerifier) Verify(path string) []error {
	if v.root == nil {
		return []error{fmt.Errorf("unable to verify path with nil root")}
	}

	// extract the plugin binary name
	binName := filepath.Base(path)

	cmdPath := strings.Split(binName, "-")
	if len(cmdPath) > 1 {
		// the first argument is always "kbcli" or "kubectl" for a plugin binary
		cmdPath = cmdPath[1:]
	}

	var errors []error
	if isExec, err := isExecutable(path); err == nil && !isExec {
		errors = append(errors, fmt.Errorf("warning: %q identified as a kbcli or kubectl plugin, but it is not executable", path))
	} else if err != nil {
		errors = append(errors, fmt.Errorf("error: unable to indentify %s as an executable file: %v", path, err))
	}

	if existingPath, ok := v.seenPlugins[binName]; ok {
		errors = append(errors, fmt.Errorf("warning: %s is overshadowed by a similarly named plugin: %s", path, existingPath))
	} else {
		v.seenPlugins[binName] = path
	}

	if cmd, _, err := v.root.Find(cmdPath); err == nil {
		errors = append(errors, fmt.Errorf("warning: %q overwrites existing kbcli command: %q", path, cmd.CommandPath()))
	}

	return errors
}

func isExecutable(fullPath string) (bool, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}

	if runtime.GOOS == "windows" {
		fileExt := strings.ToLower(filepath.Ext(fullPath))

		switch fileExt {
		case ".bat", ".cmd", ".com", ".exe", ".ps1":
			return true, nil
		}
		return false, nil
	}

	if m := info.Mode(); !m.IsDir() && m&0111 != 0 {
		return true, nil
	}

	return false, nil
}

func uniquePathsList(paths []string) []string {
	var newPaths []string
	seen := map[string]bool{}

	for _, path := range paths {
		if !seen[path] {
			newPaths = append(newPaths, path)
			seen[path] = true
		}
	}
	return newPaths
}

func hasValidPrefix(filepath string, validPrefixes []string) bool {
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(filepath, prefix+"-") {
			return true
		}
	}
	return false
}
