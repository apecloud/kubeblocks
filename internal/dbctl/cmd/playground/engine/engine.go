/*
Copyright 2022 The KubeBlocks Authors

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

package engine

import (
	"fmt"
	"strings"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

type Interface interface {
	HelmInstallOpts() *helm.InstallOpts
}

func New(engine string, version string, replicas int, name string, ns string) (Interface, error) {
	if strings.EqualFold(engine, "wesql") {
		return &WeSQL{
			version:   version,
			replicas:  replicas,
			name:      name,
			namespace: ns,
		}, nil
	}
	return nil, fmt.Errorf("unsupported engine type: %s", engine)
}
