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

var _ = Describe("replicated_state_machine builder", func() {
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
		reconfiguration := workloads.MembershipReconfiguration{
			SwitchoverAction: &workloads.Action{
				Image:   name,
				Command: []string{"bar"},
			},
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
		credential := workloads.Credential{
			Username: workloads.CredentialVar{Value: "foo"},
			Password: workloads.CredentialVar{Value: "bar"},
		}
		rsm := NewReplicatedStateMachineBuilder(ns, name).
			SetReplicas(replicas).
			SetRoles([]workloads.ReplicaRole{role}).
			SetMembershipReconfiguration(reconfiguration).
			SetTemplate(template).
			SetObservationActions(actions).
			AddObservationAction(action).
			SetService(service).
			SetCredential(credential).
			GetObject()

		Expect(rsm.Name).Should(Equal(name))
		Expect(rsm.Namespace).Should(Equal(ns))
		Expect(rsm.Spec.Replicas).Should(Equal(replicas))
		Expect(len(rsm.Spec.Roles)).Should(Equal(1))
		Expect(rsm.Spec.Roles[0]).Should(Equal(role))
		Expect(rsm.Spec.MembershipReconfiguration).ShouldNot(BeNil())
		Expect(*rsm.Spec.MembershipReconfiguration).Should(Equal(reconfiguration))
		Expect(rsm.Spec.Template).Should(Equal(template))
		Expect(len(rsm.Spec.RoleObservation.ObservationActions)).Should(Equal(2))
		Expect(rsm.Spec.RoleObservation.ObservationActions[0]).Should(Equal(actions[0]))
		Expect(rsm.Spec.RoleObservation.ObservationActions[1]).Should(Equal(action))
		Expect(rsm.Spec.Service).Should(Equal(service))
		Expect(rsm.Spec.Credential).ShouldNot(BeNil())
		Expect(*rsm.Spec.Credential).Should(Equal(credential))
	})
})
