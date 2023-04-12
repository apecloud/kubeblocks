/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
