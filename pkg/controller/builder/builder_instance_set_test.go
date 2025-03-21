/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("instance_set builder", func() {
	It("should work well", func() {
		const (
			name                         = "foo"
			ns                           = "default"
			selectorKey1, selectorValue1 = "foo-1", "bar-1"
			selectorKey2, selectorValue2 = "foo-2", "bar-2"
			selectorKey3, selectorValue3 = "foo-3", "bar-3"
			selectorKey4, selectorValue4 = "foo-4", "bar-4"
			replicas                     = int32(5)
			minReadySeconds              = int32(11)
			port                         = int32(12345)
			policy                       = appsv1.OrderedReadyPodManagement
			podUpdatePolicy              = kbappsv1.PreferInPlacePodUpdatePolicyType
		)
		parallelPodManagementConcurrency := &intstr.IntOrString{Type: intstr.String, StrVal: "100%"}
		selectors := map[string]string{selectorKey1: selectorValue1, selectorKey2: selectorValue2, selectorKey3: selectorValue3, selectorKey4: selectorValue4}
		role := workloads.ReplicaRole{
			Name:                 "foo",
			ParticipatesInQuorum: true,
			UpdatePriority:       1,
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
					Resources: corev1.VolumeResourceRequirements{
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
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("600m"),
					},
				},
			},
		}
		updateReplicas, maxUnavailable := intstr.FromInt32(3), intstr.FromInt32(2)
		strategy := workloads.InstanceUpdateStrategy{
			RollingUpdate: &workloads.RollingUpdate{
				Replicas:       &updateReplicas,
				MaxUnavailable: &maxUnavailable,
			},
		}
		memberUpdateStrategy := workloads.BestEffortParallelUpdateStrategy
		paused := true
		instances := []workloads.InstanceTemplate{
			{
				Name:     "hello",
				Replicas: func() *int32 { r := int32(2); return &r }(),
			},
			{
				Name:     "world",
				Replicas: func() *int32 { r := int32(1); return &r }(),
			},
		}
		its := NewInstanceSetBuilder(ns, name).
			SetReplicas(replicas).
			SetMinReadySeconds(minReadySeconds).
			SetSelectorMatchLabel(selectors).
			SetRoles([]workloads.ReplicaRole{role}).
			SetTemplate(template).
			SetVolumeClaimTemplates(vcs...).
			AddVolumeClaimTemplates(vc).
			SetPodManagementPolicy(policy).
			SetParallelPodManagementConcurrency(parallelPodManagementConcurrency).
			SetPodUpdatePolicy(podUpdatePolicy).
			SetInstanceUpdateStrategy(&strategy).
			SetMemberUpdateStrategy(&memberUpdateStrategy).
			SetPaused(paused).
			SetInstances(instances).
			GetObject()

		Expect(its.Name).Should(Equal(name))
		Expect(its.Namespace).Should(Equal(ns))
		Expect(its.Spec.Replicas).ShouldNot(BeNil())
		Expect(*its.Spec.Replicas).Should(Equal(replicas))
		Expect(its.Spec.Selector).ShouldNot(BeNil())
		Expect(its.Spec.Selector.MatchLabels).Should(HaveLen(4))
		Expect(its.Spec.Selector.MatchLabels[selectorKey1]).Should(Equal(selectorValue1))
		Expect(its.Spec.Selector.MatchLabels[selectorKey2]).Should(Equal(selectorValue2))
		Expect(its.Spec.Selector.MatchLabels[selectorKey3]).Should(Equal(selectorValue3))
		Expect(its.Spec.Selector.MatchLabels[selectorKey4]).Should(Equal(selectorValue4))
		Expect(its.Spec.Roles).Should(HaveLen(1))
		Expect(its.Spec.Roles[0]).Should(Equal(role))
		Expect(its.Spec.MembershipReconfiguration).Should(BeNil())
		Expect(its.Spec.Template).Should(Equal(template))
		Expect(its.Spec.VolumeClaimTemplates).Should(HaveLen(2))
		Expect(its.Spec.VolumeClaimTemplates[0]).Should(Equal(vcs[0]))
		Expect(its.Spec.VolumeClaimTemplates[1]).Should(Equal(vc))
		Expect(its.Spec.PodManagementPolicy).Should(Equal(policy))
		Expect(its.Spec.ParallelPodManagementConcurrency).Should(Equal(parallelPodManagementConcurrency))
		Expect(its.Spec.PodUpdatePolicy).Should(Equal(podUpdatePolicy))
		Expect(its.Spec.InstanceUpdateStrategy).ShouldNot(BeNil())
		Expect(its.Spec.InstanceUpdateStrategy.RollingUpdate).ShouldNot(BeNil())
		Expect(its.Spec.InstanceUpdateStrategy.RollingUpdate.Replicas).ShouldNot(BeNil())
		Expect(*its.Spec.InstanceUpdateStrategy.RollingUpdate.Replicas).Should(Equal(updateReplicas))
		Expect(its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable).ShouldNot(BeNil())
		Expect(*its.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable).Should(Equal(maxUnavailable))
		Expect(its.Spec.MemberUpdateStrategy).ShouldNot(BeNil())
		Expect(*its.Spec.MemberUpdateStrategy).Should(Equal(memberUpdateStrategy))
		Expect(its.Spec.Paused).Should(Equal(paused))
		Expect(its.Spec.Instances).Should(Equal(instances))
	})
})
