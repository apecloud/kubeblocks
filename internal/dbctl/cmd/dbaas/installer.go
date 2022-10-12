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
package dbaas

import (
	"helm.sh/helm/v3/pkg/action"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

// Installer will handle the playground cluster creation and management
type Installer struct {
	cfg *action.Configuration

	Namespace string
	Version   string
}

func (i *Installer) Install() error {
	chart := helm.InstallOpts{
		Name:      types.DbaasHelmName,
		Chart:     types.DbaasHelmChart,
		Wait:      true,
		Version:   i.Version,
		Namespace: i.Namespace,
		Sets: []string{
			"image.tag=latest",
			"image.pullPolicy=Always",
		},
		Login:    true,
		TryTimes: 2,
	}

	err := chart.Install(i.cfg)
	if err != nil {
		return err
	}

	return nil
}

// Uninstall remove dbaas
func (i *Installer) Uninstall() error {
	chart := helm.InstallOpts{
		Name:      types.DbaasHelmName,
		Namespace: i.Namespace,
	}

	err := chart.UnInstall(i.cfg)
	if err != nil {
		return err
	}

	return nil
}
