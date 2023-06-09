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
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type StatefulReplicaSetBuilder struct {
	BaseBuilder[workloads.StatefulReplicaSet, *workloads.StatefulReplicaSet, StatefulReplicaSetBuilder]
}

func NewStatefulReplicaSetBuilder(namespace, name string) *StatefulReplicaSetBuilder {
	builder := &StatefulReplicaSetBuilder{}
	builder.init(namespace, name,
		&workloads.StatefulReplicaSet{
			Spec: workloads.StatefulReplicaSetSpec{
				Replicas: 1,
				Roles: []workloads.ReplicaRole{
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

func (builder *StatefulReplicaSetBuilder) SetReplicas(replicas int32) *StatefulReplicaSetBuilder {
	builder.get().Spec.Replicas = replicas
	return builder
}

func (builder *StatefulReplicaSetBuilder) SetRoles(roles []workloads.ReplicaRole) *StatefulReplicaSetBuilder {
	builder.get().Spec.Roles = roles
	return builder
}

func (builder *StatefulReplicaSetBuilder) SetTemplate(template corev1.PodTemplateSpec) *StatefulReplicaSetBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *StatefulReplicaSetBuilder) SetObservationActions(actions []workloads.Action) *StatefulReplicaSetBuilder {
	builder.get().Spec.RoleObservation.ObservationActions = actions
	return builder
}

func (builder *StatefulReplicaSetBuilder) AddObservationAction(action workloads.Action) *StatefulReplicaSetBuilder {
	actions := builder.get().Spec.RoleObservation.ObservationActions
	actions = append(actions, action)
	builder.get().Spec.RoleObservation.ObservationActions = actions
	return builder
}

func (builder *StatefulReplicaSetBuilder) SetService(service corev1.ServiceSpec) *StatefulReplicaSetBuilder {
	builder.get().Spec.Service = service
	return builder
}
