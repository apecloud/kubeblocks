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

package util

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// CloneGitRepo clone git repo to local path
func CloneGitRepo(url, branch, path string) error {
	pullFunc := func(repo *git.Repository) error {
		// Get the working directory for the repository
		w, err := repo.Worktree()
		if err != nil {
			return err
		}
		// Pull the latest changes from the origin remote
		err = w.Pull(&git.PullOptions{
			RemoteName:    "origin",
			Progress:      os.Stdout,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			SingleBranch:  true,
		})
		if err != git.NoErrAlreadyUpToDate && err != git.ErrUnstagedChanges {
			return err
		}
		return nil
	}

	// check if local repo path already exists
	repo, err := git.PlainOpen(path)

	// repo exists, pull it
	if err == nil {
		if err = pullFunc(repo); err != nil {
			return err
		}
	}

	if err != git.ErrRepositoryNotExists {
		return err
	}

	// repo does not exists, clone it
	_, err = git.PlainClone(path, false, &git.CloneOptions{
		URL:           url,
		Progress:      os.Stdout,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	})
	return err
}

func GitGetRemoteURL(dir string) (string, error) {
	return ExecGitCommand(dir, "config", "--get", "remote.origin.url")
}

// EnsureCloned will clone into the destination path, otherwise will return no error.
func EnsureCloned(uri, destinationPath string) error {
	if ok, err := IsGitCloned(destinationPath); err != nil {
		return err
	} else if !ok {
		_, err = ExecGitCommand("", "clone", "-v", uri, destinationPath)
		return err
	}
	return nil
}

// IsGitCloned will test if the path is a git dir.
func IsGitCloned(gitPath string) (bool, error) {
	f, err := os.Stat(filepath.Join(gitPath, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil && f.IsDir(), err
}

// EnsureUpdated will ensure the destination path exists and is up to date.
func EnsureUpdated(uri, destinationPath string) error {
	if err := EnsureCloned(uri, destinationPath); err != nil {
		return err
	}
	return UpdateAndCleanUntracked(destinationPath)
}

// UpdateAndCleanUntracked  will fetch origin and set HEAD to origin/HEAD
// and also will create a pristine working directory by removing
// untracked files and directories.
func UpdateAndCleanUntracked(destinationPath string) error {
	if _, err := ExecGitCommand(destinationPath, "fetch", "-v"); err != nil {
		return errors.Wrapf(err, "fetch index at %q failed", destinationPath)
	}

	if _, err := ExecGitCommand(destinationPath, "reset", "--hard", "@{upstream}"); err != nil {
		return errors.Wrapf(err, "reset index at %q failed", destinationPath)
	}

	_, err := ExecGitCommand(destinationPath, "clean", "-xfd")
	return errors.Wrapf(err, "clean index at %q failed", destinationPath)
}

// ExecGitCommand executes a git command in the given directory.
func ExecGitCommand(pwd string, args ...string) (string, error) {
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
