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

package consensus

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

type consensusComponentWorkloadBuilder struct {
	internal.ComponentWorkloadBuilderBase
}

var _ internal.ComponentWorkloadBuilder = &consensusComponentWorkloadBuilder{}

func (b *consensusComponentWorkloadBuilder) BuildWorkload() internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		component := b.Comp.GetSynthesizedComponent()
		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), component, b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}
		sts.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

		b.Workload = sts

		// build PDB object
		if component.MaxUnavailable != nil {
			pdb, err := builder.BuildPDBLow(b.Comp.GetCluster(), component)
			if err != nil {
				return nil, err
			}
			return []client.Object{pdb}, err // don't return sts here
		}
		return nil, nil
	}
	return b.BuildWrapper(buildfn)
}

func (b *consensusComponentWorkloadBuilder) BuildService() internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		svcList, err := builder.BuildSvcListLow(b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}
		objs := make([]client.Object, 0, len(svcList))
		leader := b.Comp.GetConsensusSpec().Leader
		for _, svc := range svcList {
			if len(leader.Name) > 0 {
				svc.Spec.Selector[constant.RoleLabelKey] = leader.Name
			}
			objs = append(objs, svc)
		}
		return objs, err
	}
	return b.BuildWrapper(buildfn)
}
