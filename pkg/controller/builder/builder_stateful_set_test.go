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
	"k8s.io/apimachinery/pkg/util/intstr"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("stateful_set builder", func() {
	It("should work well", func() {
		const (
			name                         = "foo"
			ns                           = "default"
			selectorKey1, selectorValue1 = "foo-1", "bar-1"
			selectorKey2, selectorValue2 = "foo-2", "bar-2"
			selectorKey3, selectorValue3 = "foo-3", "bar-3"
			selectorKey4, selectorValue4 = "foo-4", "bar-4"
			port                         = int32(12345)
			serviceName                  = "foo"
			replicas                     = int32(5)
			minReadySeconds              = int32(37)
			policy                       = apps.OrderedReadyPodManagement
		)
		selectors := map[string]string{selectorKey4: selectorValue4}
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
		sts := NewStatefulSetBuilder(ns, name).
			AddMatchLabel(selectorKey1, selectorValue1).
			AddMatchLabels(selectorKey2, selectorValue2, selectorKey3, selectorValue3).
			AddMatchLabelsInMap(selectors).
			SetServiceName(serviceName).
			SetReplicas(replicas).
			SetMinReadySeconds(minReadySeconds).
			SetPodManagementPolicy(policy).
			SetTemplate(template).
			SetVolumeClaimTemplates(vcs...).
			AddVolumeClaimTemplates(vc).
			SetUpdateStrategy(strategy).
			SetUpdateStrategyType(strategyType).
			GetObject()

		Expect(sts.Name).Should(Equal(name))
		Expect(sts.Namespace).Should(Equal(ns))
		Expect(sts.Spec.Selector).ShouldNot(BeNil())
		Expect(sts.Spec.Selector.MatchLabels).Should(HaveLen(4))
		Expect(sts.Spec.Selector.MatchLabels[selectorKey1]).Should(Equal(selectorValue1))
		Expect(sts.Spec.Selector.MatchLabels[selectorKey2]).Should(Equal(selectorValue2))
		Expect(sts.Spec.Selector.MatchLabels[selectorKey3]).Should(Equal(selectorValue3))
		Expect(sts.Spec.Selector.MatchLabels[selectorKey4]).Should(Equal(selectorValue4))
		Expect(sts.Spec.ServiceName).Should(Equal(serviceName))
		Expect(sts.Spec.Replicas).ShouldNot(BeNil())
		Expect(*sts.Spec.Replicas).Should(Equal(replicas))
		Expect(sts.Spec.PodManagementPolicy).Should(Equal(policy))
		Expect(sts.Spec.Template).Should(Equal(template))
		Expect(sts.Spec.VolumeClaimTemplates).Should(HaveLen(2))
		Expect(sts.Spec.VolumeClaimTemplates[0]).Should(Equal(vcs[0]))
		Expect(sts.Spec.VolumeClaimTemplates[1]).Should(Equal(vc))
		Expect(sts.Spec.UpdateStrategy.Type).Should(Equal(strategyType))
		Expect(sts.Spec.UpdateStrategy.RollingUpdate).ShouldNot(BeNil())
		Expect(sts.Spec.UpdateStrategy.RollingUpdate.Partition).ShouldNot(BeNil())
		Expect(*sts.Spec.UpdateStrategy.RollingUpdate.Partition).Should(Equal(partition))
		Expect(sts.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable).ShouldNot(BeNil())
		Expect(sts.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable).ShouldNot(Equal(maxUnavailable))

		labelSelector := &metav1.LabelSelector{
			MatchLabels: selectors,
		}
		sts = NewStatefulSetBuilder(ns, name).SetSelector(labelSelector).GetObject()
		Expect(sts.Spec.Selector).Should(Equal(labelSelector))
	})
})
