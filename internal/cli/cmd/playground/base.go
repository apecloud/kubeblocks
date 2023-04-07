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
	"fmt"
	"path/filepath"
	"time"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
)

type baseOptions struct {
	startTime     time.Time
	prevCluster   *cp.K8sClusterInfo
	playgroundDir string
	stateFilePath string
}

func (o *baseOptions) validate() error {
	var err error

	o.playgroundDir, err = initPlaygroundDir()
	if err != nil {
		return err
	}

	o.stateFilePath = filepath.Join(o.playgroundDir, stateFileName)
	o.prevCluster, err = readClusterInfoFromFile(o.stateFilePath)
	if err != nil {
		return err
	}

	// check existed cluster info
	if o.prevCluster != nil && !o.prevCluster.IsValid() {
		return fmt.Errorf("invalid playground kubernetes cluster info from state file %s, %v", o.stateFilePath, o.prevCluster)
	}
	return nil
}
