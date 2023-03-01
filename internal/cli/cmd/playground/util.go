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

package playground

import (
	"path/filepath"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

func playgroundDir() (string, error) {
	cliPath, err := util.GetCliHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cliPath, "playground"), nil
}

// cloudProviderRepoDir cloud provider repo directory
func cloudProviderRepoDir() (string, error) {
	dir, err := playgroundDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cp.GitRepoName), err
}
