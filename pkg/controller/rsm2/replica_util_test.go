/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package rsm2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

var _ = Describe("replica util test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)
	var (
		priorityMap map[string]int
		rsm         *workloads.ReplicatedStateMachine

		roles = []workloads.ReplicaRole{
			{
				Name:       "leader",
				IsLeader:   true,
				CanVote:    true,
				AccessMode: workloads.ReadWriteMode,
			},
			{
				Name:       "follower",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.ReadonlyMode,
			},
			{
				Name:       "logger",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.NoneMode,
			},
			{
				Name:       "learner",
				IsLeader:   false,
				CanVote:    false,
				AccessMode: workloads.ReadonlyMode,
			},
		}
		pod = builder.NewPodBuilder("", "").
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-svc",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: 12345,
					},
				},
			}).GetObject()
		template = corev1.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}
	)

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetService(&corev1.Service{}).
			SetReplicas(3).
			SetTemplate(template).
			SetRoles(roles).
			GetObject()
		priorityMap = rsm1.ComposeRolePriorityMap(rsm.Spec.Roles)
	})

	Context("sortObjects function", func() {
		It("should work well", func() {
			pods := []client.Object{
				builder.NewPodBuilder(namespace, "pod-0").AddLabels(rsm1.RoleLabelKey, "follower").GetObject(),
				builder.NewPodBuilder(namespace, "pod-1").AddLabels(rsm1.RoleLabelKey, "logger").GetObject(),
				builder.NewPodBuilder(namespace, "pod-2").GetObject(),
				builder.NewPodBuilder(namespace, "pod-3").AddLabels(rsm1.RoleLabelKey, "learner").GetObject(),
				builder.NewPodBuilder(namespace, "pod-4").AddLabels(rsm1.RoleLabelKey, "candidate").GetObject(),
				builder.NewPodBuilder(namespace, "pod-5").AddLabels(rsm1.RoleLabelKey, "leader").GetObject(),
				builder.NewPodBuilder(namespace, "pod-6").AddLabels(rsm1.RoleLabelKey, "learner").GetObject(),
			}
			expectedOrder := []string{"pod-4", "pod-2", "pod-3", "pod-6", "pod-1", "pod-0", "pod-5"}

			sortObjects(pods, priorityMap, false)
			for i, pod := range pods {
				Expect(pod.GetName()).Should(Equal(expectedOrder[i]))
			}
		})
	})

	Context("isRunningAndReady", func() {
		It("should work well", func() {
			By("creating a new pod")
			pod := builder.NewPodBuilder(namespace, name).GetObject()
			Expect(isRunningAndReady(pod)).Should(BeFalse())

			By("set phase to running")
			pod.Status.Phase = corev1.PodRunning
			Expect(isRunningAndReady(pod)).Should(BeFalse())

			By("set ready condition")
			condition := corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue}
			pod.Status.Conditions = append(pod.Status.Conditions, condition)
			Expect(isRunningAndReady(pod)).Should(BeTrue())
		})
	})

	Context("getPodRevision", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).GetObject()
			Expect(getPodRevision(pod)).Should(BeEmpty())

			revision := "revision"
			pod = builder.NewPodBuilder(namespace, name).AddControllerRevisionHashLabel(revision).GetObject()
			Expect(getPodRevision(pod)).Should(Equal(revision))
		})
	})

	Context("validateDupReplicaNames", func() {
		It("should work well", func() {
			By("build name list without duplication")
			replicas := []string{"pod-0", "pod-1"}
			Expect(validateDupReplicaNames(replicas, func(item string) string {
				return item
			})).Should(Succeed())

			By("add a duplicate name")
			replicas = append(replicas, "pod-0")
			Expect(validateDupReplicaNames(replicas, func(item string) string {
				return item
			})).ShouldNot(Succeed())
		})
	})

	Context("buildReplicaName2TemplateMap", func() {
		It("build a rsm with default template only", func() {
			nameTemplate, err := buildReplicaName2TemplateMap(rsm)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := rsm.Name + "-0"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(rsm.Name + "-1"))
			Expect(nameTemplate).Should(HaveKey(rsm.Name + "-2"))
			Expect(nameTemplate[name0].PodTemplateSpec).Should(Equal(template))
		})

		It("build a rsm with one instance template override", func() {
			nameOverride0 := "name-o-0"
			annotationOverride := map[string]string{
				"foo": "bar",
			}
			labelOverride := map[string]string{
				"foo": "bar",
			}
			imageOverride := "foo:latest"
			instance := workloads.InstanceTemplate{
				Name:        &nameOverride0,
				Annotations: annotationOverride,
				Labels:      labelOverride,
				Image:       &imageOverride,
			}
			rsm.Spec.Instances = append(rsm.Spec.Instances, instance)
			nameTemplate, err := buildReplicaName2TemplateMap(rsm)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := rsm.Name + "-0"
			name1 := rsm.Name + "-1"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(name1))
			Expect(nameTemplate).Should(HaveKey(nameOverride0))
			envConfigName := rsm1.GetEnvConfigMapName(rsm.Name)
			expectedTemplate := rsm1.BuildPodTemplate(rsm, envConfigName)
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[name1].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Annotations).Should(Equal(annotationOverride))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Labels).Should(Equal(labelOverride))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec.Containers[0].Image).Should(Equal(imageOverride))
		})
	})
})
