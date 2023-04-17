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

package stateful

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/internal"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
)

type statefulComponentWorkloadBuilder struct {
	internal.ComponentWorkloadBuilderBase
}

var _ internal.ComponentWorkloadBuilder = &statefulComponentWorkloadBuilder{}

func (b *statefulComponentWorkloadBuilder) BuildWorkload() internal.ComponentWorkloadBuilder {
	buildfn := func() ([]client.Object, error) {
		if b.EnvConfig == nil {
			return nil, fmt.Errorf("build consensus workload but env config is nil, cluster: %s, component: %s",
				b.Comp.GetClusterName(), b.Comp.GetName())
		}

		sts, err := builder.BuildStsLow(b.ReqCtx, b.Comp.GetCluster(), b.Comp.GetSynthesizedComponent(), b.EnvConfig.Name)
		if err != nil {
			return nil, err
		}

		b.Workload = sts

		return nil, nil // don't return deploy here
	}
	return b.BuildWrapper(buildfn)
}
