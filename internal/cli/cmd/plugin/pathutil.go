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
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type move struct {
	from, to string
}

func findMoveTargets(fromDir, toDir string, fo FileOperation) ([]move, error) {
	if fo.To != filepath.Clean(fo.To) {
		return nil, errors.Errorf("the provided path is not clean, %q should be %q", fo.To, filepath.Clean(fo.To))
	}
	fromDir, err := filepath.Abs(fromDir)
	if err != nil {
		return nil, errors.Wrap(err, "could not get the relative path for the move src")
	}

	klog.V(4).Infof("Trying to move single file directly from=%q to=%q with file operation=%#v", fromDir, toDir, fo)
	if m, ok, err := getDirectMove(fromDir, toDir, fo); err != nil {
		return nil, errors.Wrap(err, "failed to detect single move operation")
	} else if ok {
		klog.V(3).Infof("Detected single move from file operation=%#v", fo)
		return []move{m}, nil
	}

	klog.V(4).Infoln("Wasn't a single file, proceeding with Glob move")
	newDir, err := filepath.Abs(filepath.Join(filepath.FromSlash(toDir), filepath.FromSlash(fo.To)))
	if err != nil {
		return nil, errors.Wrap(err, "could not get the relative path for the move dst")
	}

	gl, err := filepath.Glob(filepath.Join(filepath.FromSlash(fromDir), filepath.FromSlash(fo.From)))
	if err != nil {
		return nil, errors.Wrap(err, "could not get files using a glob string")
	}
	if len(gl) == 0 {
		return nil, errors.Errorf("no files in the plugin archive matched the glob pattern=%s", fo.From)
	}

	moves := make([]move, 0, len(gl))
	for _, v := range gl {
		newPath := filepath.Join(newDir, filepath.Base(filepath.FromSlash(v)))
		// Check secure path
		m := move{from: v, to: newPath}
		if !isMoveAllowed(fromDir, toDir, m) {
			return nil, errors.Errorf("can't move, move target %v is not a subpath from=%q, to=%q", m, fromDir, toDir)
		}
		moves = append(moves, m)
	}
	return moves, nil
}

func getDirectMove(fromDir, toDir string, fo FileOperation) (move, bool, error) {
	var m move
	fromDir, err := filepath.Abs(fromDir)
	if err != nil {
		return m, false, errors.Wrap(err, "could not get the relative path for the move src")
	}

	toDir, err = filepath.Abs(toDir)
	if err != nil {
		return m, false, errors.Wrap(err, "could not get the relative path for the move src")
	}

	// Check is direct file (not a Glob)
	fromFilePath := filepath.Clean(filepath.Join(fromDir, fo.From))
	_, err = os.Stat(fromFilePath)
	if err != nil {
		return m, false, nil
	}

	// If target is empty use old file name.
	if filepath.Clean(fo.To) == "." {
		fo.To = filepath.Base(fromFilePath)
	}

	// Build new file name
	toFilePath, err := filepath.Abs(filepath.Join(filepath.FromSlash(toDir), filepath.FromSlash(fo.To)))
	if err != nil {
		return m, false, errors.Wrap(err, "could not get the relative path for the move dst")
	}

	// Check sane path
	m = move{from: fromFilePath, to: toFilePath}
	if !isMoveAllowed(fromDir, toDir, m) {
		return move{}, false, errors.Errorf("can't move, move target %v is out of bounds from=%q, to=%q", m, fromDir, toDir)
	}

	return m, true, nil
}

func isMoveAllowed(fromBase, toBase string, m move) bool {
	_, okFrom := IsSubPath(fromBase, m.from)
	_, okTo := IsSubPath(toBase, m.to)
	return okFrom && okTo
}

func moveFiles(fromDir, toDir string, fo FileOperation) error {
	klog.V(4).Infof("Finding move targets from %q to %q with file operation=%#v", fromDir, toDir, fo)
	moves, err := findMoveTargets(fromDir, toDir, fo)
	if err != nil {
		return errors.Wrap(err, "could not find move targets")
	}

	for _, m := range moves {
		klog.V(2).Infof("Move file from %q to %q", m.from, m.to)
		if err := os.MkdirAll(filepath.Dir(m.to), 0o755); err != nil {
			return errors.Wrapf(err, "failed to create move path %q", filepath.Dir(m.to))
		}

		if err = renameOrCopy(m.from, m.to); err != nil {
			return errors.Wrapf(err, "could not rename/copy file from %q to %q", m.from, m.to)
		}
	}
	klog.V(4).Infoln("Move operations are complete")
	return nil
}

func moveAllFiles(fromDir, toDir string, fos []FileOperation) error {
	for _, fo := range fos {
		if err := moveFiles(fromDir, toDir, fo); err != nil {
			return errors.Wrap(err, "failed moving files")
		}
	}
	return nil
}

// moveToInstallDir moves plugins from srcDir to dstDir (created in this method) with given FileOperation.
func moveToInstallDir(srcDir, installDir string, fos []FileOperation) error {
	installationDir := filepath.Dir(installDir)
	klog.V(4).Infof("Creating directory %q", installationDir)
	if err := os.MkdirAll(installationDir, 0o755); err != nil {
		return errors.Wrapf(err, "error creating directory at %q", installationDir)
	}

	tmp, err := os.MkdirTemp("", "kbcli-temp-move")
	klog.V(4).Infof("Creating temp plugin move operations dir %q", tmp)
	if err != nil {
		return errors.Wrap(err, "failed to find a temporary director")
	}
	defer os.RemoveAll(tmp)

	if err = moveAllFiles(srcDir, tmp, fos); err != nil {
		return errors.Wrap(err, "failed to move files")
	}

	klog.V(2).Infof("Move directory %q to %q", tmp, installDir)
	if err = renameOrCopy(tmp, installDir); err != nil {
		defer func() {
			klog.V(3).Info("Cleaning up installation directory due to error during copying files")
			os.Remove(installDir)
		}()
		return errors.Wrapf(err, "could not rename/copy directory %q to %q", tmp, installDir)
	}
	return nil
}

// renameOrCopy will try to rename a dir or file. If rename is not supported, a manual copy will be performed.
// Existing files at "to" will be deleted.
func renameOrCopy(from, to string) error {
	// Try atomic rename (does not work cross partition).
	fi, err := os.Stat(to)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "error checking move target dir %q", to)
	}
	if fi != nil && fi.IsDir() {
		klog.V(4).Infof("There's already a directory at move target %q. deleting.", to)
		if err := os.RemoveAll(to); err != nil {
			return errors.Wrapf(err, "error cleaning up dir %q", to)
		}
		klog.V(4).Infof("Move target directory %q cleaned up", to)
	}

	err = os.Rename(from, to)
	// Fallback for invalid cross-device link (errno:18).
	if isCrossDeviceRenameErr(err) {
		klog.V(2).Infof("Cross-device link error while copying, fallback to manual copy")
		return errors.Wrap(copyTree(from, to), "failed to copy directory tree as a fallback")
	}
	return err
}

// copyTree copies files or directories, recursively.
func copyTree(from, to string) (err error) {
	return filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		newPath, _ := ReplaceBase(path, from, to)
		if info.IsDir() {
			klog.V(4).Infof("Creating new dir %q", newPath)
			err = os.MkdirAll(newPath, info.Mode())
		} else {
			klog.V(4).Infof("Copying file %q", newPath)
			err = copyFile(path, newPath, info.Mode())
		}
		return err
	})
}

func copyFile(source, dst string, mode os.FileMode) (err error) {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	if err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

// isCrossDeviceRenameErr determines if a os.Rename error is due to cross-fs/drive/volume copying.
func isCrossDeviceRenameErr(err error) bool {
	le, ok := err.(*os.LinkError)
	if !ok {
		return false
	}
	errno, ok := le.Err.(syscall.Errno)
	if !ok {
		return false
	}
	return (util.IsWindows() && errno == 17) || // syscall.ERROR_NOT_SAME_DEVICE
		(!util.IsWindows() && errno == 18) // syscall.EXDEV
}

// IsSubPath checks if the extending path is an extension of the basePath, it will return the extending path
// elements. Both paths have to be absolute or have the same root directory. The remaining path elements
func IsSubPath(basePath, subPath string) (string, bool) {
	extendingPath, err := filepath.Rel(basePath, subPath)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(extendingPath, "..") {
		return "", false
	}
	return extendingPath, true
}

// ReplaceBase will return a replacement path with replacement as a base of the path instead of the old base. a/b/c, a, d -> d/b/c
func ReplaceBase(path, old, replacement string) (string, error) {
	extendingPath, ok := IsSubPath(old, path)
	if !ok {
		return "", errors.Errorf("can't replace %q in %q, it is not a subpath", old, path)
	}
	return filepath.Join(replacement, extendingPath), nil
}

// CanonicalPluginName resolves a plugin's index and name from input string.
// If an index is not specified, the default index name is assumed.
func CanonicalPluginName(in string) (string, string) {
	if strings.Count(in, "/") == 0 {
		return DefaultIndexName, in
	}
	p := strings.SplitN(in, "/", 2)
	return p[0], p[1]
}

func createOrUpdateLink(binDir, binary, plugin string) error {
	dst := filepath.Join(binDir, pluginNameToBin(plugin, util.IsWindows()))

	if err := removeLink(dst); err != nil {
		return errors.Wrap(err, "failed to remove old symlink")
	}
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		return errors.Wrapf(err, "can't create symbolic link, source binary (%q) cannot be found in extracted archive", binary)
	}

	// Create new
	klog.V(2).Infof("Creating symlink to %q at %q", binary, dst)
	if err := os.Symlink(binary, dst); err != nil {
		return errors.Wrapf(err, "failed to create a symlink from %q to %q", binary, dst)
	}
	klog.V(2).Infof("Created symlink at %q", dst)

	return nil
}

// removeLink removes a symlink reference if exists.
func removeLink(path string) error {
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		klog.V(3).Infof("No file found at %q", path)
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "failed to read the symlink in %q", path)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		return errors.Errorf("file %q is not a symlink (mode=%s)", path, fi.Mode())
	}
	if err := os.Remove(path); err != nil {
		return errors.Wrapf(err, "failed to remove the symlink in %q", path)
	}
	klog.V(3).Infof("Removed symlink from %q", path)
	return nil
}

// pluginNameToBin creates the name of the symlink file for the plugin name.
// It converts dashes to underscores.
func pluginNameToBin(name string, isWindows bool) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = "kbcli-" + name
	if isWindows {
		name += ".exe"
	}
	return name
}
