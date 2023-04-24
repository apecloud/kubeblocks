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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/plugin/download"
)

var (
	ErrIsAlreadyInstalled = errors.New("can't install, the newest version is already installed")
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

// ListIndexes returns a slice of Index objects. The path argument is used as
// the base path of the index.
func ListIndexes(paths *Paths) ([]Index, error) {
	entries, err := os.ReadDir(paths.IndexBase())
	if err != nil {
		return nil, err
	}

	var indexes []Index
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		indexName := e.Name()
		remote, err := GitGetRemoteURL(paths.IndexPath(indexName))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list the remote URL for index %s", indexName)
		}

		indexes = append(indexes, Index{
			Name: indexName,
			URL:  remote,
		})
	}
	return indexes, nil
}

// AddIndex initializes a new index to install plugins from.
func AddIndex(paths *Paths, name, url string) error {
	dir := paths.IndexPath(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return EnsureCloned(url, dir)
	} else if err != nil {
		return err
	}
	return errors.New("index already exists")
}

// DeleteIndex removes specified index name. If index does not exist, returns an error that can be tested by os.IsNotExist.
func DeleteIndex(paths *Paths, name string) error {
	dir := paths.IndexPath(name)
	if _, err := os.Stat(dir); err != nil {
		return err
	}

	return os.RemoveAll(dir)
}

func GitGetRemoteURL(dir string) (string, error) {
	return Exec(dir, "config", "--get", "remote.origin.url")
}

func EnsureCloned(uri, destinationPath string) error {
	if ok, err := IsGitCloned(destinationPath); err != nil {
		return err
	} else if !ok {
		_, err = Exec("", "clone", "-v", uri, destinationPath)
		return err
	}
	return nil
}

func IsGitCloned(gitPath string) (bool, error) {
	f, err := os.Stat(filepath.Join(gitPath, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil && f.IsDir(), err
}

func Exec(pwd string, args ...string) (string, error) {
	klog.V(4).Infof("Going to run git %s", strings.Join(args, " "))
	cmd := exec.Command("git", args...)
	cmd.Dir = pwd
	buf := bytes.Buffer{}
	var w io.Writer = &buf
	if klog.V(2).Enabled() {
		w = io.MultiWriter(w, os.Stderr)
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		return "", errors.Wrapf(err, "command execution failure, output=%q", buf.String())
	}
	return strings.TrimSpace(buf.String()), nil
}

func CanonicalPluginName(in string) (string, string) {
	if strings.Count(in, "/") == 0 {
		return DefaultIndexName, in
	}
	p := strings.SplitN(in, "/", 2)
	return p[0], p[1]
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

		binDir: p.BinPath(),
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
	if err := downloadAndExtract(downloadStagingDir, op.platform.URI, op.platform.Sha256, opts.ArchiveFileOverride); err != nil {
		return errors.Wrap(err, "failed to unpack into staging dir")
	}

	binName := op.platform.Bin
	if !strings.HasPrefix(binName, "kbcli-") && !strings.HasPrefix(binName, "kubectl") {
		binName = "kbcli-" + binName
	}
	if err := copyData(filepath.Join(downloadStagingDir, op.platform.Bin), filepath.Join(paths.BinPath(), binName)); err != nil {
		return errors.Wrap(err, "failed copy file into bin")
	}

	return nil
}

// downloadAndExtract downloads the specified archive uri (or uses the provided overrideFile, if a non-empty value)
// while validating its checksum with the provided sha256sum, and extracts its contents to extractDir that must be.
// created.
func downloadAndExtract(extractDir, uri, sha256sum, overrideFile string) error {
	var fetcher download.Fetcher = download.HTTPFetcher{}
	if overrideFile != "" {
		fetcher = download.NewFileFetcher(overrideFile)
	}

	verifier := download.NewSha256Verifier(sha256sum)
	err := download.NewDownloader(verifier, fetcher).Get(uri, extractDir)
	return errors.Wrap(err, "failed to unpack the plugin archive")
}

func indent(s string) string {
	out := "\\\n"
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	out += regexp.MustCompile("(?m)^").ReplaceAllString(s, " | ")
	out += "\n/"
	return out
}

func copyData(sourcePath, targetPath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return errors.New("read file err")
	}

	err = os.WriteFile(targetPath, data, 0777)
	if err != nil {
		return errors.New("write file err")
	}
	return nil
}
