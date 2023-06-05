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
				Roles: []workloads.ConsensusRole{
					{
						Name:       "leader",
						AccessMode: workloads.ReadWriteMode,
						IsLeader:   true,
						CanVote:    true,
					},
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

func (builder *ConsensusSetBuilder) SetRoles(roles []workloads.ConsensusRole) *ConsensusSetBuilder {
	builder.get().Spec.Roles = roles
	return builder
}
