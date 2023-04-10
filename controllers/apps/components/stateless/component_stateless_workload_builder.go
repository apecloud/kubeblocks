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

package stateless

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

type statelessComponentWorkloadBuilder struct {
	internal.ComponentWorkloadBuilderBase
	workload *appsv1.Deployment
}

func (b *statelessComponentWorkloadBuilder) MutableWorkload(_ int32) client.Object {
	return b.workload
}

func (b *statelessComponentWorkloadBuilder) BuildWorkload(_ int32) internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		deploy, err := builder.BuildDeployLow(b.ReqCtx, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent())
		if err != nil {
			return nil, err
		}

		b.workload = deploy

		return nil, nil // don't return deployment here
	}
	return b.BuildWrapper(buildfn)
}
