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

package builder

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type ConsensusSetBuilder struct {
	BaseBuilder[workloads.ConsensusSet, *workloads.ConsensusSet, ConsensusSetBuilder]
}

func NewConsensusSetBuilder(namespace, name string) *ConsensusSetBuilder {
	builder := &ConsensusSetBuilder{}
	builder.init(namespace, name,
		&workloads.ConsensusSet{
			Spec: workloads.ConsensusSetSpec{
				Replicas: 1,
				Leader: workloads.ConsensusMember{
					Name: "leader",
				},
				UpdateStrategy: workloads.SerialUpdateStrategy,
			},
		}, builder)
	return builder
}

func (builder *ConsensusSetBuilder) SetReplicas(replicas int32) *ConsensusSetBuilder {
	builder.get().Spec.Replicas = replicas
	return builder
}
