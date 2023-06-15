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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return plugin, errors.Wrap(ValidatePlugin(plugin.Name, plugin), "plugin manifest validation error")
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

func isSupportAPIVersion(apiVersion string) bool {
	for _, v := range SupportAPIVersion {
		if apiVersion == v {
			return true
		}
	}
	return false
}

func ValidatePlugin(name string, p Plugin) error {
	if !isSupportAPIVersion(p.APIVersion) {
		return errors.Errorf("plugin manifest has apiVersion=%q, not supported in this version of krew (try updating plugin index or install a newer version of krew)", p.APIVersion)
	}
	if p.Kind != PluginKind {
		return errors.Errorf("plugin manifest has kind=%q, but only %q is supported", p.Kind, PluginKind)
	}
	if p.Name != name {
		return errors.Errorf("plugin manifest has name=%q, but expected %q", p.Name, name)
	}
	if p.Spec.ShortDescription == "" {
		return errors.New("should have a short description")
	}
	if len(p.Spec.Platforms) == 0 {
		return errors.New("should have a platform")
	}
	if p.Spec.Version == "" {
		return errors.New("should have a version")
	}
	if _, err := parseVersion(p.Spec.Version); err != nil {
		return errors.Wrap(err, "failed to parse version")
	}
	for _, pl := range p.Spec.Platforms {
		if err := validatePlatform(pl); err != nil {
			return errors.Wrapf(err, "platform (%+v) is badly constructed", pl)
		}
	}
	return nil
}

func validatePlatform(p Platform) error {
	if p.URI == "" {
		return errors.New("`uri` is unset")
	}
	if p.Sha256 == "" {
		return errors.New("`sha256` sum is unset")
	}
	if p.Bin == "" {
		return errors.New("`bin` is unset")
	}
	if err := validateFiles(p.Files); err != nil {
		return errors.Wrap(err, "`files` is invalid")
	}
	if err := validateSelector(p.Selector); err != nil {
		return errors.Wrap(err, "invalid platform selector")
	}
	return nil
}

func validateFiles(fops []FileOperation) error {
	if fops == nil {
		return nil
	}
	if len(fops) == 0 {
		return errors.New("`files` is empty, set it")
	}
	for _, op := range fops {
		if op.From == "" {
			return errors.New("`from` field is unset")
		} else if op.To == "" {
			return errors.New("`to` field is unset")
		}
	}
	return nil
}

// validateSelector checks if the platform selector uses supported keys and is not empty or nil.
func validateSelector(sel *metav1.LabelSelector) error {
	if sel == nil {
		return errors.New("nil selector is not supported")
	}
	if sel.MatchLabels == nil && len(sel.MatchExpressions) == 0 {
		return errors.New("empty selector is not supported")
	}

	// check for unsupported keys
	keys := []string{}
	for k := range sel.MatchLabels {
		keys = append(keys, k)
	}
	for _, expr := range sel.MatchExpressions {
		keys = append(keys, expr.Key)
	}
	for _, key := range keys {
		if key != "os" && key != "arch" {
			return errors.Errorf("key %q not supported", key)
		}
	}

	if sel.MatchLabels != nil && len(sel.MatchLabels) == 0 {
		return errors.New("`matchLabels` specified but empty")
	}
	if sel.MatchExpressions != nil && len(sel.MatchExpressions) == 0 {
		return errors.New("`matchExpressions` specified but empty")
	}

	return nil
}

func findPluginManifestFiles(indexDir string) ([]string, error) {
	var out []string
	files, err := os.ReadDir(indexDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open index dir")
	}
	for _, file := range files {
		if file.Type().IsRegular() && filepath.Ext(file.Name()) == ManifestExtension {
			out = append(out, file.Name())
		}
	}
	return out, nil
}

// LoadPluginListFromFS will parse and retrieve all plugin files.
func LoadPluginListFromFS(indexDir string) ([]Plugin, error) {
	indexDir, err := filepath.EvalSymlinks(indexDir)
	if err != nil {
		return nil, err
	}

	files, err := findPluginManifestFiles(indexDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan plugins in index directory")
	}
	klog.V(4).Infof("found %d plugins in dir %s", len(files), indexDir)

	list := make([]Plugin, 0, len(files))
	for _, file := range files {
		pluginName := strings.TrimSuffix(file, filepath.Ext(file))
		p, err := LoadPluginByName(indexDir, pluginName)
		if err != nil {
			// loading the index repository shouldn't fail because of one plugin
			// if loading the plugin fails, log the error and continue
			klog.Errorf("failed to read or parse plugin manifest %q: %v", pluginName, err)
			continue
		}
		list = append(list, p)
	}
	return list, nil
}
