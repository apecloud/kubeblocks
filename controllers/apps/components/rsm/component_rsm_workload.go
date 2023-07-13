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

package rsm

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

type rsmComponentWorkloadBuilder struct {
	internal.ComponentWorkloadBuilderBase
}

var _ internal.ComponentWorkloadBuilder = &rsmComponentWorkloadBuilder{}

func (b *rsmComponentWorkloadBuilder) BuildWorkload() internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		component := b.Comp.GetSynthesizedComponent()
		obj, err := builder.BuildRSM(b.ReqCtx, b.Comp.GetCluster(), component)
		if err != nil {
			return nil, err
		}

		b.Workload = obj

		return nil, nil // don't return sts here
	}
	return b.BuildWrapper(buildfn)
}

// BuildEnv overrides internal.ComponentWorkloadBuilderBase as env has been pushed down to rsm
func (b *rsmComponentWorkloadBuilder) BuildEnv() internal.ComponentWorkloadBuilder {
	return b.ConcreteBuilder
}
