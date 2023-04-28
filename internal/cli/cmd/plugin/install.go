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
	"runtime"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/plugin/download"
)

var (
	pluginInstallExample = templates.Examples(`
	# install a kbcli or kubectl plugin by name
	kbcli plugin install [PLUGIN]

	# install a kbcli or kubectl plugin by name and index
	kbcli plugin install [INDEX/PLUGIN]
	`)
)

type pluginInstallOption struct {
	plugins []pluginEntry

	genericclioptions.IOStreams
}

type pluginEntry struct {
	index  string
	plugin Plugin
}

func NewPluginInstallCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &pluginInstallOption{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install kbcli or kubectl plugins",
		Example: pluginInstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.install())
		},
	}
	return cmd
}

func (o *pluginInstallOption) complete(names []string) error {
	for _, name := range names {
		indexName, pluginName := CanonicalPluginName(name)
		plugin, err := LoadPluginByName(paths.IndexPluginsPath(indexName), pluginName)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.Errorf("plugin %q does not exist in the plugin index", name)
			}
			return errors.Wrapf(err, "failed to load plugin %q from the index", name)
		}
		o.plugins = append(o.plugins, pluginEntry{
			index:  indexName,
			plugin: plugin,
		})
	}
	return nil
}

func (o *pluginInstallOption) install() error {
	var failed []string
	var returnErr error
	for _, entry := range o.plugins {
		plugin := entry.plugin
		fmt.Fprintf(os.Stderr, "Installing plugin: %s\n", plugin.Name)
		err := Install(paths, plugin, entry.index, InstallOpts{})
		if err == ErrIsAlreadyInstalled {
			klog.Warningf("Skipping plugin %q, it is already installed", plugin.Name)
			continue
		}
		if err != nil {
			klog.Warningf("failed to install plugin %q: %v", plugin.Name, err)
			if returnErr == nil {
				returnErr = err
			}
			failed = append(failed, plugin.Name)
			continue
		}
		fmt.Fprintf(os.Stderr, "Installed plugin: %s\n", plugin.Name)
		output := fmt.Sprintf("Use this plugin:\n\tkubectl %s\n", plugin.Name)
		if plugin.Spec.Homepage != "" {
			output += fmt.Sprintf("Documentation:\n\t%s\n", plugin.Spec.Homepage)
		}
		if plugin.Spec.Caveats != "" {
			output += fmt.Sprintf("Caveats:\n%s\n", indent(plugin.Spec.Caveats))
		}
		fmt.Fprintln(os.Stderr, indent(output))
	}
	if len(failed) > 0 {
		return errors.Wrapf(returnErr, "failed to install some plugins: %+v", failed)
	}
	return nil
}

func IsWindows() bool {
	goos := runtime.GOOS
	return goos == "windows"
}

// Install will download and install a plugin. The operation tries
// to not get the plugin dir in a bad state if it fails during the process.
func Install(p *Paths, plugin Plugin, indexName string, opts InstallOpts) error {
	klog.V(2).Infof("Looking for installed versions")
	_, err := ReadReceiptFromFile(p.PluginInstallReceiptPath(plugin.Name))
	if err == nil {
		return ErrIsAlreadyInstalled
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to look up plugin receipt")
	}

	// Find available installation candidate
	candidate, ok, err := GetMatchingPlatform(plugin.Spec.Platforms)
	if err != nil {
		return errors.Wrap(err, "failed trying to find a matching platform in plugin spec")
	}
	if !ok {
		return errors.Errorf("plugin %q does not offer installation for this platform", plugin.Name)
	}

	// The actual install should be the last action so that a failure during receipt
	// saving does not result in an installed plugin without receipt.
	klog.V(3).Infof("Install plugin %s at version=%s", plugin.Name, plugin.Spec.Version)
	if err := install(installOperation{
		pluginName: plugin.Name,
		platform:   candidate,

		binDir:     p.BinPath(),
		installDir: p.PluginVersionInstallPath(plugin.Name, plugin.Spec.Version),
	}, opts); err != nil {
		return errors.Wrap(err, "install failed")
	}

	klog.V(3).Infof("Storing install receipt for plugin %s", plugin.Name)
	err = StoreReceipt(NewReceipt(plugin, indexName, metav1.Now()), p.PluginInstallReceiptPath(plugin.Name))
	return errors.Wrap(err, "installation receipt could not be stored, uninstall may fail")
}

func install(op installOperation, opts InstallOpts) error {
	// Download and extract
	klog.V(3).Infof("Creating download staging directory")
	downloadStagingDir, err := os.MkdirTemp("", "kbcli-downloads")
	if err != nil {
		return errors.Wrapf(err, "could not create staging dir %q", downloadStagingDir)
	}
	klog.V(3).Infof("Successfully created download staging directory %q", downloadStagingDir)
	defer func() {
		klog.V(3).Infof("Deleting the download staging directory %s", downloadStagingDir)
		if err := os.RemoveAll(downloadStagingDir); err != nil {
			klog.Warningf("failed to clean up download staging directory: %s", err)
		}
	}()
	if err := download.DownloadAndExtract(downloadStagingDir, op.platform.URI, op.platform.Sha256, opts.ArchiveFileOverride); err != nil {
		return errors.Wrap(err, "failed to unpack into staging dir")
	}

	applyDefaults(&op.platform)
	if err := moveToInstallDir(downloadStagingDir, op.installDir, op.platform.Files); err != nil {
		return errors.Wrap(err, "failed while moving files to the installation directory")
	}

	subPathAbs, err := filepath.Abs(op.installDir)
	if err != nil {
		return errors.Wrapf(err, "failed to get the absolute fullPath of %q", op.installDir)
	}
	fullPath := filepath.Join(op.installDir, filepath.FromSlash(op.platform.Bin))
	pathAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return errors.Wrapf(err, "failed to get the absolute fullPath of %q", fullPath)
	}
	if _, ok := IsSubPath(subPathAbs, pathAbs); !ok {
		return errors.Wrapf(err, "the fullPath %q does not extend the sub-fullPath %q", fullPath, op.installDir)
	}
	err = createOrUpdateLink(op.binDir, fullPath, op.pluginName)
	return errors.Wrap(err, "failed to link installed plugin")
}
