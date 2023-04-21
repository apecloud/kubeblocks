/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
