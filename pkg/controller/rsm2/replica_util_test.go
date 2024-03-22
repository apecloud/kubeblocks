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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

var _ = Describe("replica util test", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetService(&corev1.Service{}).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
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
			nameTemplate[name0].PodTemplateSpec.Spec.Volumes = nil
			envConfigName := rsm1.GetEnvConfigMapName(rsm.Name)
			defaultTemplate := rsm1.BuildPodTemplate(rsm, envConfigName)
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(defaultTemplate.Spec))
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

	Context("buildReplicaByTemplate", func() {
		It("should work well", func() {
			nameTemplate, err := buildReplicaName2TemplateMap(rsm)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name := name + "-0"
			Expect(nameTemplate).Should(HaveKey(name))
			template := nameTemplate[name]
			replica, err := buildReplicaByTemplate(name, template, rsm)
			Expect(err).Should(BeNil())
			Expect(replica.pod).ShouldNot(BeNil())
			Expect(replica.pvcs).ShouldNot(BeNil())
			Expect(replica.pvcs).Should(HaveLen(1))
			Expect(replica.pod.Name).Should(Equal(name))
			Expect(replica.pod.Namespace).Should(Equal(rsm.Namespace))
			Expect(replica.pod.Spec.Volumes).Should(HaveLen(1))
			Expect(replica.pod.Spec.Volumes[0].Name).Should(Equal(volumeClaimTemplates[0].Name))
			envConfigName := rsm1.GetEnvConfigMapName(rsm.Name)
			expectedTemplate := rsm1.BuildPodTemplate(rsm, envConfigName)
			Expect(replica.pod.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			// reset pod.volumes, pod.hostname and pod.subdomain
			replica.pod.Spec.Volumes = nil
			replica.pod.Spec.Hostname = ""
			replica.pod.Spec.Subdomain = ""
			Expect(replica.pod.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(replica.pvcs[0].Name).Should(Equal(fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, replica.pod.Name)))
			Expect(replica.pvcs[0].Spec.Resources).Should(Equal(volumeClaimTemplates[0].Spec.Resources))
		})
	})

	Context("validateSpec", func() {
		It("should work well", func() {
			By("a valid spec")
			Expect(validateSpec(rsm)).Should(Succeed())

			By("instance.replicas exceeds 1 when name set")
			rsm1 := rsm.DeepCopy()
			name := "name-override-0"
			replicas := int32(2)
			instance := workloads.InstanceTemplate{
				Name:     &name,
				Replicas: &replicas,
			}
			rsm1.Spec.Instances = append(rsm1.Spec.Instances, instance)
			err := validateSpec(rsm1)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("replicas should be empty or no more than 1 if name set"))

			By("sum of replicas in instance exceeds spec.replicas")
			rsm2 := rsm.DeepCopy()
			replicas = 4
			generateName := "barrrrr"
			instance = workloads.InstanceTemplate{
				Replicas:     &replicas,
				GenerateName: &generateName,
			}
			rsm2.Spec.Instances = append(rsm2.Spec.Instances, instance)
			err = validateSpec(rsm2)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("should not greater than replicas in spec"))
		})
	})

	Context("copyAndMerge", func() {
		It("should work well", func() {
			By("merge sts")
			oldSts := builder.NewStatefulSetBuilder(namespace, name).
				AddAnnotations("foo", "foo").
				AddLabels("foo", "foo").
				GetObject()
			newSts := builder.NewStatefulSetBuilder(namespace, name).
				AddAnnotations("foo", "bar").
				AddLabels("foo", "bar").
				SetReplicas(3).
				SetTemplate(template).
				SetUpdateStrategyType(appsv1.OnDeleteStatefulSetStrategyType).
				GetObject()
			sts := copyAndMerge(oldSts, newSts)
			Expect(sts).Should(Equal(newSts))

			By("merge svc")
			oldSvc := builder.NewServiceBuilder(namespace, name).
				AddAnnotations("foo", "foo").
				SetSpec(&corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"foo": "foo",
					},
					Ports: []corev1.ServicePort{
						{
							Port:     1235,
							Protocol: corev1.ProtocolTCP,
							Name:     "foo",
						},
					},
				}).
				GetObject()
			newSvc := builder.NewServiceBuilder(namespace, name).
				AddAnnotations("foo", "bar").
				SetSpec(&corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Selector: map[string]string{
						"foo": "bar",
					},
					Ports: []corev1.ServicePort{
						{
							Port:     1234,
							Protocol: corev1.ProtocolUDP,
							Name:     "bar",
						},
					},
				}).
				GetObject()
			svc := copyAndMerge(oldSvc, newSvc)
			Expect(svc).Should(Equal(newSvc))

			By("merge cm")
			oldCm := builder.NewConfigMapBuilder(namespace, name).
				SetBinaryData(map[string][]byte{
					"foo": []byte("foo"),
				}).
				SetData(map[string]string{
					"foo": "foo",
				}).
				GetObject()
			newCm := builder.NewConfigMapBuilder(namespace, name).
				SetBinaryData(map[string][]byte{
					"foo": []byte("bar"),
				}).
				SetData(map[string]string{
					"foo": "bar",
				}).
				GetObject()
			cm := copyAndMerge(oldCm, newCm)
			Expect(cm).Should(Equal(newCm))

			By("merge pod")
			oldPod := builder.NewPodBuilder(namespace, name).GetObject()
			newPod := builder.NewPodBuilder(namespace, name).
				SetPodSpec(template.Spec).
				GetObject()
			pod := copyAndMerge(oldPod, newPod)
			Expect(pod).Should(BeNil())

			By("merge pvc")
			oldPvc := builder.NewPVCBuilder(namespace, name).
				SetResources(corev1.ResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1G"),
				}}).
				GetObject()
			newPvc := builder.NewPVCBuilder(namespace, name).
				SetResources(corev1.ResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("2G"),
				}}).
				GetObject()
			pvc := copyAndMerge(oldPvc, newPvc)
			Expect(pvc).Should(Equal(newPvc))

			By("merge other kind(secret)")
			oldSecret := builder.NewSecretBuilder(namespace, name).
				SetData(map[string][]byte{
					"foo": []byte("foo"),
				}).
				SetImmutable(true).
				GetObject()
			newSecret := builder.NewSecretBuilder(namespace, name).
				SetData(map[string][]byte{
					"foo": []byte("bar"),
				}).
				SetImmutable(false).
				GetObject()
			secret := copyAndMerge(oldSecret, newSecret)
			Expect(secret).Should(Equal(secret))
		})
	})
})
