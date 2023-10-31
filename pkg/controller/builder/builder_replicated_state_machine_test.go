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

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

var _ = Describe("replicated_state_machine builder", func() {
	It("should work well", func() {
		const (
			name                         = "foo"
			ns                           = "default"
			selectorKey1, selectorValue1 = "foo-1", "bar-1"
			selectorKey2, selectorValue2 = "foo-2", "bar-2"
			selectorKey3, selectorValue3 = "foo-3", "bar-3"
			selectorKey4, selectorValue4 = "foo-4", "bar-4"
			serviceName                  = "foo"
			replicas                     = int32(5)
			port                         = int32(12345)
			policy                       = apps.OrderedReadyPodManagement
		)
		selectors := map[string]string{selectorKey4: selectorValue4}
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
		vcs := []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-1",
					Namespace: ns,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: "foo-1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("500m"),
						},
					},
				},
			},
		}
		vc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-2",
				Namespace: ns,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeName: "foo-2",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("600m"),
					},
				},
			},
		}
		partition, maxUnavailable := int32(3), intstr.FromInt(2)
		strategy := apps.StatefulSetUpdateStrategy{
			Type: apps.RollingUpdateStatefulSetStrategyType,
			RollingUpdate: &apps.RollingUpdateStatefulSetStrategy{
				Partition:      &partition,
				MaxUnavailable: &maxUnavailable,
			},
		}
		strategyType := apps.OnDeleteStatefulSetStrategyType
		delay := int32(10)
		roleProbe := workloads.RoleProbe{InitialDelaySeconds: delay}
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
		memberUpdateStrategy := workloads.BestEffortParallelUpdateStrategy
		service := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "foo",
						Protocol: corev1.ProtocolTCP,
						Port:     port,
					},
				},
			},
		}
		alternativeServices := []corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "bar",
							Protocol: corev1.ProtocolTCP,
							Port:     port,
						},
					},
				},
			},
		}
		paused := true
		credential := workloads.Credential{
			Username: workloads.CredentialVar{Value: "foo"},
			Password: workloads.CredentialVar{Value: "bar"},
		}
		rsm := NewReplicatedStateMachineBuilder(ns, name).
			SetReplicas(replicas).
			AddMatchLabel(selectorKey1, selectorValue1).
			AddMatchLabels(selectorKey2, selectorValue2, selectorKey3, selectorValue3).
			AddMatchLabelsInMap(selectors).
			SetServiceName(serviceName).
			SetRoles([]workloads.ReplicaRole{role}).
			SetMembershipReconfiguration(&reconfiguration).
			SetTemplate(template).
			SetVolumeClaimTemplates(vcs...).
			AddVolumeClaimTemplates(vc).
			SetPodManagementPolicy(policy).
			SetUpdateStrategy(strategy).
			SetUpdateStrategyType(strategyType).
			SetRoleProbe(&roleProbe).
			SetCustomHandler(actions).
			AddCustomHandler(action).
			SetMemberUpdateStrategy(&memberUpdateStrategy).
			SetService(service).
			SetAlternativeServices(alternativeServices).
			SetPaused(paused).
			SetCredential(credential).
			GetObject()

		Expect(rsm.Name).Should(Equal(name))
		Expect(rsm.Namespace).Should(Equal(ns))
		Expect(rsm.Spec.Replicas).ShouldNot(BeNil())
		Expect(*rsm.Spec.Replicas).Should(Equal(replicas))
		Expect(rsm.Spec.Selector).ShouldNot(BeNil())
		Expect(rsm.Spec.Selector.MatchLabels).Should(HaveLen(4))
		Expect(rsm.Spec.Selector.MatchLabels[selectorKey1]).Should(Equal(selectorValue1))
		Expect(rsm.Spec.Selector.MatchLabels[selectorKey2]).Should(Equal(selectorValue2))
		Expect(rsm.Spec.Selector.MatchLabels[selectorKey3]).Should(Equal(selectorValue3))
		Expect(rsm.Spec.Selector.MatchLabels[selectorKey4]).Should(Equal(selectorValue4))
		Expect(rsm.Spec.ServiceName).Should(Equal(serviceName))
		Expect(rsm.Spec.Roles).Should(HaveLen(1))
		Expect(rsm.Spec.Roles[0]).Should(Equal(role))
		Expect(rsm.Spec.MembershipReconfiguration).ShouldNot(BeNil())
		Expect(*rsm.Spec.MembershipReconfiguration).Should(Equal(reconfiguration))
		Expect(rsm.Spec.Template).Should(Equal(template))
		Expect(rsm.Spec.VolumeClaimTemplates).Should(HaveLen(2))
		Expect(rsm.Spec.VolumeClaimTemplates[0]).Should(Equal(vcs[0]))
		Expect(rsm.Spec.VolumeClaimTemplates[1]).Should(Equal(vc))
		Expect(rsm.Spec.PodManagementPolicy).Should(Equal(policy))
		Expect(rsm.Spec.UpdateStrategy.Type).Should(Equal(strategyType))
		Expect(rsm.Spec.UpdateStrategy.RollingUpdate).ShouldNot(BeNil())
		Expect(rsm.Spec.UpdateStrategy.RollingUpdate.Partition).ShouldNot(BeNil())
		Expect(*rsm.Spec.UpdateStrategy.RollingUpdate.Partition).Should(Equal(partition))
		Expect(rsm.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable).ShouldNot(BeNil())
		Expect(rsm.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable).ShouldNot(Equal(maxUnavailable))
		Expect(rsm.Spec.RoleProbe).ShouldNot(BeNil())
		Expect(rsm.Spec.RoleProbe.InitialDelaySeconds).Should(Equal(delay))
		Expect(rsm.Spec.RoleProbe.CustomHandler).Should(HaveLen(2))
		Expect(rsm.Spec.RoleProbe.CustomHandler[0]).Should(Equal(actions[0]))
		Expect(rsm.Spec.RoleProbe.CustomHandler[1]).Should(Equal(action))
		Expect(rsm.Spec.MemberUpdateStrategy).ShouldNot(BeNil())
		Expect(*rsm.Spec.MemberUpdateStrategy).Should(Equal(memberUpdateStrategy))
		Expect(rsm.Spec.Service).ShouldNot(BeNil())
		Expect(rsm.Spec.Service).Should(BeEquivalentTo(service))
		Expect(rsm.Spec.AlternativeServices).ShouldNot(BeNil())
		Expect(rsm.Spec.AlternativeServices).Should(Equal(alternativeServices))
		Expect(rsm.Spec.Paused).Should(Equal(paused))
		Expect(rsm.Spec.Credential).ShouldNot(BeNil())
		Expect(*rsm.Spec.Credential).Should(Equal(credential))
	})
})
