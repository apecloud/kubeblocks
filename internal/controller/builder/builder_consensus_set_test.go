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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

var _ = Describe("consensus_set builder", func() {
	It("should work well", func() {
		const (
			name     = "foo"
			ns       = "default"
			replicas = int32(5)
			port     = int32(12345)
		)
		role := workloads.ReplicaRole{
			Name:       "foo",
			AccessMode: workloads.ReadWriteMode,
			IsLeader:   true,
			CanVote:    true,
		}
		pod := NewPodBuilder(ns, "foo").
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          "foo",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: port,
					},
				},
			}).GetObject()
		template := corev1.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}
		actions := []workloads.Action{
			{
				Image:   "foo-1",
				Command: []string{"bar-1"},
			},
		}
		action := workloads.Action{
			Image:   "foo-2",
			Command: []string{"bar-2"},
		}
		service := corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "foo",
					Protocol: corev1.ProtocolTCP,
					Port:     port,
				},
			},
		}
		csSet := NewConsensusSetBuilder(ns, name).
			SetReplicas(replicas).
			SetRoles([]workloads.ReplicaRole{role}).
			SetTemplate(template).
			SetObservationActions(actions).
			AddObservationAction(action).
			SetService(service).
			GetObject()

		Expect(csSet.Name).Should(Equal(name))
		Expect(csSet.Namespace).Should(Equal(ns))
		Expect(csSet.Spec.Replicas).Should(Equal(replicas))
		Expect(len(csSet.Spec.Roles)).Should(Equal(1))
		Expect(csSet.Spec.Roles[0]).Should(Equal(role))
		Expect(csSet.Spec.Template).Should(Equal(template))
		Expect(len(csSet.Spec.RoleObservation.ObservationActions)).Should(Equal(2))
		Expect(csSet.Spec.RoleObservation.ObservationActions[0]).Should(Equal(actions[0]))
		Expect(csSet.Spec.RoleObservation.ObservationActions[1]).Should(Equal(action))
		Expect(csSet.Spec.Service).Should(Equal(service))
	})
})
