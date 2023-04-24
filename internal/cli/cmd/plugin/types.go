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
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultIndexURI   = "https://github.com/kubernetes-sigs/krew-index.git"
	DefaultIndexName  = "default"
	ManifestExtension = ".yaml"
)

type Paths struct {
	base string
	tmp  string
}

func (p *Paths) BasePath() string {
	return p.base
}

func (p *Paths) IndexBase() string {
	return filepath.Join(p.base, "index")
}

func (p *Paths) IndexPath(name string) string {
	return filepath.Join(p.IndexBase(), name)
}

func (p *Paths) IndexPluginsPath(name string) string {
	return filepath.Join(p.IndexPath(name), "plugins")
}

func (p *Paths) InstallReceiptsPath() string {
	return filepath.Join(p.base, "receipts")
}

func (p *Paths) BinPath() string {
	return filepath.Join(p.base, "bin")
}

func (p *Paths) PluginInstallReceiptPath(plugin string) string {
	return filepath.Join(p.InstallReceiptsPath(), plugin+".yaml")
}

type Index struct {
	Name string
	URL  string
}

type Plugin struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata"`

	Spec PluginSpec `json:"spec"`
}

// PluginSpec is the plugin specification.
type PluginSpec struct {
	Version          string `json:"version,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	Description      string `json:"description,omitempty"`
	Caveats          string `json:"caveats,omitempty"`
	Homepage         string `json:"homepage,omitempty"`

	Platforms []Platform `json:"platforms,omitempty"`
}

// Platform describes how to perform an installation on a specific platform
// and how to match the target platform (os, arch).
type Platform struct {
	URI    string `json:"uri,omitempty"`
	Sha256 string `json:"sha256,omitempty"`

	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	Files    []FileOperation       `json:"files"`

	// Bin specifies the path to the plugin executable.
	// The path is relative to the root of the installation folder.
	// The binary will be linked after all FileOperations are executed.
	Bin string `json:"bin"`
}

// FileOperation specifies a file copying operation from plugin archive to the
// installation directory.
type FileOperation struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// Receipt describes a plugin receipt file.
type Receipt struct {
	Plugin `json:",inline" yaml:",inline"`

	Status ReceiptStatus `json:"status"`
}

// ReceiptStatus contains information about the installed plugin.
type ReceiptStatus struct {
	Source SourceIndex `json:"source"`
}

// SourceIndex contains information about the index a plugin was installed from.
type SourceIndex struct {
	// Name is the configured name of an index a plugin was installed from.
	Name string `json:"name"`
}

type InstallOpts struct {
	ArchiveFileOverride string
}

type installOperation struct {
	pluginName string
	platform   Platform

	binDir string
}

// NewReceipt returns a new receipt with the given plugin and index name.
func NewReceipt(plugin Plugin, indexName string, timestamp metav1.Time) Receipt {
	plugin.CreationTimestamp = timestamp
	return Receipt{
		Plugin: plugin,
		Status: ReceiptStatus{
			Source: SourceIndex{
				Name: indexName,
			},
		},
	}
}
func StoreReceipt(receipt Receipt, dest string) error {
	yamlBytes, err := yaml.Marshal(receipt)
	if err != nil {
		return errors.Wrapf(err, "convert to yaml")
	}

	err = os.WriteFile(dest, yamlBytes, 0o644)
	return errors.Wrapf(err, "write plugin receipt %q", dest)
}
