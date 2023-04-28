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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

var (
	ErrIsAlreadyInstalled = errors.New("can't install, the newest version is already installed")
	ErrIsNotInstalled     = errors.New("plugin is not installed")
	ErrIsAlreadyUpgraded  = errors.New("can't upgrade, the newest version is already installed")
)

func GetKbcliPluginPath() *Paths {
	base := filepath.Join(homedir.HomeDir(), ".kbcli", "plugins")
	return NewPaths(base)
}

func EnsureDirs(paths ...string) error {
	for _, p := range paths {
		if err := os.MkdirAll(p, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func NewPaths(base string) *Paths {
	return &Paths{base: base, tmp: os.TempDir()}
}

func LoadPluginByName(pluginsDir, pluginName string) (Plugin, error) {
	klog.V(4).Infof("Reading plugin %q from %s", pluginName, pluginsDir)
	return ReadPluginFromFile(filepath.Join(pluginsDir, pluginName+ManifestExtension))
}

func ReadPluginFromFile(path string) (Plugin, error) {
	var plugin Plugin
	err := readFromFile(path, &plugin)
	if err != nil {
		return plugin, err
	}
	return plugin, nil
}

func ReadReceiptFromFile(path string) (Receipt, error) {
	var receipt Receipt
	err := readFromFile(path, &receipt)
	if err != nil {
		return receipt, err
	}
	return receipt, nil
}

func readFromFile(path string, as interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = decodeFile(f, &as)
	return errors.Wrapf(err, "failed to parse yaml file %q", path)
}

func decodeFile(r io.ReadCloser, as interface{}) error {
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, &as)
}

func indent(s string) string {
	out := "\\\n"
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	out += regexp.MustCompile("(?m)^").ReplaceAllString(s, " | ")
	out += "\n/"
	return out
}

func applyDefaults(platform *Platform) {
	if platform.Files == nil {
		platform.Files = []FileOperation{{From: "*", To: "."}}
		klog.V(4).Infof("file operation not specified, assuming %v", platform.Files)
	}
}

// GetInstalledPluginReceipts returns a list of receipts.
func GetInstalledPluginReceipts(receiptsDir string) ([]Receipt, error) {
	files, err := filepath.Glob(filepath.Join(receiptsDir, "*"+ManifestExtension))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to glob receipts directory (%s) for manifests", receiptsDir)
	}
	out := make([]Receipt, 0, len(files))
	for _, f := range files {
		r, err := ReadReceiptFromFile(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse plugin install receipt %s", f)
		}
		out = append(out, r)
		klog.V(4).Infof("parsed receipt for %s: version=%s", r.GetObjectMeta().GetName(), r.Spec.Version)

	}
	return out, nil
}
