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

package components

import (
	"github.com/apecloud/kubeblocks/internal/controller/factory"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statelessComponentWorkloadBuilder struct {
	componentWorkloadBuilderBase
}

var _ componentWorkloadBuilder = &statelessComponentWorkloadBuilder{}

func (b *statelessComponentWorkloadBuilder) BuildWorkload() componentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		deploy, err := factory.BuildDeploy(b.ReqCtx, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent(), b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}
		b.Workload = deploy
		return nil, nil // don't return deployment here
	}
	return b.BuildWrapper(buildfn)
}
