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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	k8sver "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	pluginUpgradeExample = templates.Examples(`
	# upgrade installed plugins with specified name
	kbcli plugin upgrade myplugin

	# upgrade installed plugin to a newer version
	kbcli plugin upgrade --all
	`)
)

type upgradeOptions struct {
	//	common user flags
	all bool

	pluginNames []string
	genericclioptions.IOStreams
}

func NewPluginUpgradeCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := &upgradeOptions{
		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrade kbcli or kubectl plugins",
		Example: pluginUpgradeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().BoolVar(&o.all, "all", o.all, "Upgrade all installed plugins")

	return cmd
}

func (o *upgradeOptions) complete(args []string) error {
	if o.all {
		installed, err := GetInstalledPluginReceipts(paths.InstallReceiptsPath())
		if err != nil {
			return err
		}
		for _, receipt := range installed {
			o.pluginNames = append(o.pluginNames, receipt.Status.Source.Name+"/"+receipt.Name)
		}
	} else {
		if len(args) == 0 {
			return errors.New("no plugin name specified")
		}
		for _, arg := range args {
			receipt, err := ReadReceiptFromFile(paths.PluginInstallReceiptPath(arg))
			if err != nil {
				return err
			}
			o.pluginNames = append(o.pluginNames, receipt.Status.Source.Name+"/"+receipt.Name)
		}
	}

	return nil
}

func (o *upgradeOptions) run() error {
	for _, name := range o.pluginNames {
		indexName, pluginName := CanonicalPluginName(name)

		plugin, err := LoadPluginByName(paths.IndexPluginsPath(indexName), pluginName)
		if err != nil {
			return err
		}

		fmt.Fprintf(o.Out, "Upgrading plugin: %s\n", name)
		if err := Upgrade(paths, plugin, indexName); err != nil {
			if err == ErrIsAlreadyUpgraded {
				fmt.Fprintf(o.Out, "Plugin %q is already upgraded\n", name)
				continue
			}
			return err
		}
	}
	return nil
}

// Upgrade will reinstall and delete the old plugin. The operation tries
// to not get the plugin dir in a bad state if it fails during the process.
func Upgrade(p *Paths, plugin Plugin, indexName string) error {
	installReceipt, err := ReadReceiptFromFile(p.PluginInstallReceiptPath(plugin.Name))
	if err != nil {
		return errors.Wrapf(err, "failed to load install receipt for plugin %q", plugin.Name)
	}

	curVersion := installReceipt.Spec.Version
	curv, err := parseVersion(curVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to parse installed plugin version (%q) as a semver value", curVersion)
	}

	// Find available installation candidate
	candidate, ok, err := GetMatchingPlatform(plugin.Spec.Platforms)
	if err != nil {
		return errors.Wrap(err, "failed trying to find a matching platform in plugin spec")
	}
	if !ok {
		return errors.Errorf("plugin %q does not offer installation for this platform (%s)",
			plugin.Name, OSArch())
	}

	newVersion := plugin.Spec.Version
	newv, err := parseVersion(newVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to parse candidate version spec (%q)", newVersion)
	}
	klog.V(2).Infof("Comparing versions: current=%s target=%s", curv, newv)

	// See if it's a newer version
	if !curv.LessThan(newv) {
		klog.V(3).Infof("Plugin does not need upgrade (%s ≥ %s)", curv, newv)
		return ErrIsAlreadyUpgraded
	}
	klog.V(1).Infof("Plugin needs upgrade (%s < %s)", curv, newv)

	// Re-Install
	klog.V(1).Infof("Installing new version %s", newVersion)
	if err := install(installOperation{
		pluginName: plugin.Name,
		platform:   candidate,

		installDir: p.PluginVersionInstallPath(plugin.Name, newVersion),
		binDir:     p.BinPath(),
	}, InstallOpts{}); err != nil {
		return errors.Wrap(err, "failed to install new version")
	}

	klog.V(2).Infof("Upgrading install receipt for plugin %s", plugin.Name)
	if err = StoreReceipt(NewReceipt(plugin, indexName, installReceipt.CreationTimestamp), p.PluginInstallReceiptPath(plugin.Name)); err != nil {
		return errors.Wrap(err, "installation receipt could not be stored, uninstall may fail")
	}

	// Clean old installations
	klog.V(2).Infof("Starting old version cleanup")
	return cleanupInstallation(p, plugin, curVersion)
}

// cleanupInstallation will remove a plugin directly
func cleanupInstallation(p *Paths, plugin Plugin, oldVersion string) error {
	klog.V(1).Infof("Remove old plugin installation under %q", p.PluginVersionInstallPath(plugin.Name, oldVersion))
	return os.RemoveAll(p.PluginVersionInstallPath(plugin.Name, oldVersion))
}

func parseVersion(s string) (*k8sver.Version, error) {
	var vv *k8sver.Version
	if !strings.HasPrefix(s, "v") {
		return vv, errors.Errorf("version string %q does not start with 'v'", s)
	}
	vv, err := k8sver.ParseSemantic(s)
	if err != nil {
		return vv, err
	}
	return vv, nil
}
