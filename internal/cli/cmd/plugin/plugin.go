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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	pluginLong = templates.LongDesc(`
	Provides utilities for interacting with plugins.
		
	Plugins provide extended functionality that is not part of the major command-line distribution.
	`)

	pluginListExample = templates.Examples(`
	# List all available plugins file on a user's PATH.
	kbcli plugin list
	`)

	ValidPluginFilenamePrefixes = []string{"kbcli", "kubectl"}
	paths                       = GetKbcliPluginPath()
)

func NewPluginCmd(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Provides utilities for interacting with plugins.",
		Long:  pluginLong,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			InitPlugin()
		},
	}

	cmd.AddCommand(
		NewPluginListCmd(streams),
		NewPluginIndexCmd(streams),
		NewPluginInstallCmd(streams),
		NewPluginUninstallCmd(streams),
		NewPluginSearchCmd(streams),
		NewPluginDescribeCmd(streams),
		NewPluginUpgradeCmd(streams),
	)
	return cmd
}

type PluginListOptions struct {
	Verifier PathVerifier

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

	if len(plugins) == 0 {
		pluginErrors = append(pluginErrors, fmt.Errorf("error: unable to find any kbcli or kubectl plugins in your PATH"))
	}

	pluginWarnings := 0
	p := NewPluginPrinter(o.IOStreams.Out)
	errMsg := ""
	for _, pluginPath := range plugins {
		name := filepath.Base(pluginPath)
		path := filepath.Dir(pluginPath)
		if errs := o.Verifier.Verify(pluginPath); len(errs) != 0 {
			for _, err := range errs {
				errMsg += fmt.Sprintf("%s\n", err)
				pluginWarnings++
			}
		}
		addPluginRow(name, path, p)
	}
	p.Print()
	klog.V(1).Info(errMsg)

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
				klog.V(1).Info("Unable to read directory %q from your PATH: %v. Skipping...\n", dir, err)
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

// PathVerifier receives a path and validates it.
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

	if util.IsWindows() {
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

func NewPluginPrinter(out io.Writer) *printer.TablePrinter {
	t := printer.NewTablePrinter(out)
	t.SetHeader("NAME", "PATH")
	return t
}

func addPluginRow(name, path string, p *printer.TablePrinter) {
	p.AddRow(name, path)
}

func InitPlugin() {
	// Ensure that the base directories exist
	if err := EnsureDirs(paths.BasePath(),
		paths.BinPath(),
		paths.InstallPath(),
		paths.IndexBase(),
		paths.InstallReceiptsPath()); err != nil {
		klog.Fatal(err)
	}

	// check if index exists, if indexes don't exist, download default index
	indexes, err := ListIndexes(paths)
	if err != nil {
		klog.Fatal(err)
	}
	if len(indexes) == 0 {
		klog.V(1).Info("no index found, downloading default index")
		if err := AddIndex(paths, DefaultIndexName, DefaultIndexURI); err != nil {
			klog.Fatal("failed to download default index", err)
		}
	}
}
