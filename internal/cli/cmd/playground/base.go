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

package playground

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
)

type baseOptions struct {
	startTime time.Time
	Timeout   time.Duration
	// prevCluster is the previous cluster info
	prevCluster *cp.K8sClusterInfo
	// kubeConfigPath is the tmp kubeconfig path that will be used when int and destroy
	kubeConfigPath string
	// stateFilePath is the state file path
	stateFilePath string
}

func (o *baseOptions) validate() error {
	playgroundDir, err := initPlaygroundDir()
	if err != nil {
		return err
	}

	o.kubeConfigPath = filepath.Join(playgroundDir, "kubeconfig")
	if _, err = os.Stat(o.kubeConfigPath); err == nil {
		if err = os.Remove(o.kubeConfigPath); err != nil {
			return err
		}
	}

	o.stateFilePath = filepath.Join(playgroundDir, stateFileName)
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
