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

package instanceset

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
)

var _ = Describe("instance util test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
		priorityMap = ComposeRolePriorityMap(its.Spec.Roles)
	})

	Context("sortObjects function", func() {
		It("should work well", func() {
			pods := []client.Object{
				builder.NewPodBuilder(namespace, "pod-0").AddLabels(RoleLabelKey, "follower").GetObject(),
				builder.NewPodBuilder(namespace, "pod-1").AddLabels(RoleLabelKey, "logger").GetObject(),
				builder.NewPodBuilder(namespace, "pod-2").GetObject(),
				builder.NewPodBuilder(namespace, "pod-3").AddLabels(RoleLabelKey, "learner").GetObject(),
				builder.NewPodBuilder(namespace, "pod-4").AddLabels(RoleLabelKey, "candidate").GetObject(),
				builder.NewPodBuilder(namespace, "pod-5").AddLabels(RoleLabelKey, "leader").GetObject(),
				builder.NewPodBuilder(namespace, "pod-6").AddLabels(RoleLabelKey, "learner").GetObject(),
				builder.NewPodBuilder(namespace, "pod-10").AddLabels(RoleLabelKey, "learner").GetObject(),
				builder.NewPodBuilder(namespace, "foo-20").AddLabels(RoleLabelKey, "learner").GetObject(),
			}
			expectedOrder := []string{"pod-4", "pod-2", "pod-10", "pod-6", "pod-3", "foo-20", "pod-1", "pod-0", "pod-5"}

			sortObjects(pods, priorityMap, false)
			for i, pod := range pods {
				Expect(pod.GetName()).Should(Equal(expectedOrder[i]))
			}
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

	Context("buildInstancePodByTemplate", func() {
		It("should work well", func() {
			itsExt, err := instancetemplate.BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := nameBuilder.BuildInstanceName2TemplateMap()
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name := name + "-0"
			Expect(nameTemplate).Should(HaveKey(name))
			template := nameTemplate[name]
			pod, err := buildInstancePodByTemplate(name, template, its, "")
			Expect(err).Should(BeNil())
			Expect(pod).ShouldNot(BeNil())
			Expect(pod.Name).Should(Equal(name))
			Expect(pod.Namespace).Should(Equal(its.Namespace))
			Expect(pod.Spec.Volumes).Should(HaveLen(1))
			Expect(pod.Spec.Volumes[0].Name).Should(Equal(volumeClaimTemplates[0].Name))
			expectedTemplate := its.Spec.Template.DeepCopy()
			Expect(pod.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			// reset pod.volumes, pod.hostname and pod.subdomain
			pod.Spec.Volumes = nil
			pod.Spec.Hostname = ""
			pod.Spec.Subdomain = ""
			Expect(pod.Spec).Should(Equal(expectedTemplate.Spec))
		})

		It("adds nodeSelector according to annotation", func() {
			itsExt, err := instancetemplate.BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := nameBuilder.BuildInstanceName2TemplateMap()
			Expect(err).Should(BeNil())
			name := name + "-0"
			Expect(nameTemplate).Should(HaveKey(name))
			template := nameTemplate[name]

			node := "test-node-1"
			Expect(MergeNodeSelectorOnceAnnotation(its, map[string]string{name: node})).To(Succeed())
			pod, err := buildInstancePodByTemplate(name, template, its, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.NodeSelector[corev1.LabelHostname]).To(Equal(node))

			By("test with an already existing annotation")
			delete(its.Annotations, constant.NodeSelectorOnceAnnotationKey)
			Expect(MergeNodeSelectorOnceAnnotation(its, map[string]string{"other-pod": "other-node"})).To(Succeed())
			Expect(MergeNodeSelectorOnceAnnotation(its, map[string]string{name: node})).To(Succeed())
			mapping, err := ParseNodeSelectorOnceAnnotation(its)
			Expect(err).NotTo(HaveOccurred())
			Expect(mapping).To(HaveKeyWithValue("other-pod", "other-node"))
			Expect(mapping).To(HaveKeyWithValue(name, node))
			pod, err = buildInstancePodByTemplate(name, template, its, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.NodeSelector[corev1.LabelHostname]).To(Equal(node))
		})
	})

	Context("buildInstancePVCByTemplate", func() {
		It("should work well", func() {
			itsExt, err := instancetemplate.BuildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := nameBuilder.BuildInstanceName2TemplateMap()
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name := name + "-0"
			Expect(nameTemplate).Should(HaveKey(name))
			template := nameTemplate[name]
			pvcs, err := buildInstancePVCByTemplate(name, template, its)
			Expect(err).Should(BeNil())
			Expect(pvcs).Should(HaveLen(1))
			Expect(pvcs[0].Name).Should(Equal(fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, name)))
			Expect(pvcs[0].Labels[constant.VolumeClaimTemplateNameLabelKey]).Should(Equal(volumeClaimTemplates[0].Name))
			Expect(pvcs[0].Spec.Resources).Should(Equal(volumeClaimTemplates[0].Spec.Resources))
		})
	})

	Context("copyAndMerge", func() {
		It("should work well", func() {
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
							Name:     "foo",
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
			oldPod := builder.NewPodBuilder(namespace, name).
				AddContainer(corev1.Container{Name: "foo", Image: "bar-old"}).
				GetObject()
			newPod := builder.NewPodBuilder(namespace, name).
				SetPodSpec(template.Spec).
				GetObject()
			pod := copyAndMerge(oldPod, newPod)
			Expect(equalBasicInPlaceFields(pod.(*corev1.Pod), newPod)).Should(BeTrue())

			By("merge pvc")
			oldPvc := builder.NewPVCBuilder(namespace, name).
				SetResources(corev1.VolumeResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1G"),
				}}).
				GetObject()
			newPvc := builder.NewPVCBuilder(namespace, name).
				SetResources(corev1.VolumeResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{
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

	Context("GenerateInstanceNamesFromTemplate", func() {
		It("should work well", func() {
			parentName := "foo"
			templateName := "bar"
			templates := []*instanceTemplateExt{
				{
					Name:     "",
					Replicas: 2,
				},
				{
					Replicas: 2,
					Name:     templateName,
				},
			}
			offlineInstances := []string{"foo-bar-1", "foo-0"}

			var instanceNameList []string
			for _, template := range templates {
				instanceNames, err := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, offlineInstances, nil)
				Expect(err).Should(BeNil())
				instanceNameList = append(instanceNameList, instanceNames...)
			}
			getNameNOrdinalFunc := func(i int) (string, int) {
				return parseParentNameAndOrdinal(instanceNameList[i])
			}
			baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
			podNamesExpected := []string{"foo-1", "foo-2", "foo-bar-0", "foo-bar-2"}
			Expect(instanceNameList).Should(Equal(podNamesExpected))
		})

		It("without OfflineInstances, should work well", func() {
			parentName := "foo"
			templateName := "bar"
			templates := []*instanceTemplateExt{
				{
					Name:     "",
					Replicas: 2,
				},
				{
					Replicas: 2,
					Name:     templateName,
				},
			}
			templateName2OrdinalListMap := map[string][]int32{
				"":           {1, 2},
				templateName: {0, 2},
			}

			var instanceNameList []string
			for _, template := range templates {
				instanceNames, err := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, nil, templateName2OrdinalListMap[template.Name])
				Expect(err).Should(BeNil())
				instanceNameList = append(instanceNameList, instanceNames...)
			}
			getNameNOrdinalFunc := func(i int) (string, int) {
				return parseParentNameAndOrdinal(instanceNameList[i])
			}
			baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
			podNamesExpected := []string{"foo-1", "foo-2", "foo-bar-0", "foo-bar-2"}
			Expect(instanceNameList).Should(Equal(podNamesExpected))
		})

		It("with OfflineInstances, should work well", func() {
			parentName := "foo"
			templateName := "bar"
			templates := []*instanceTemplateExt{
				{
					Name:     "",
					Replicas: 2,
				},
				{
					Replicas: 2,
					Name:     templateName,
				},
			}
			templateName2OrdinalListMap := map[string][]int32{
				"":           {0, 1, 2},
				templateName: {0, 1, 2},
			}
			offlineInstances := []string{"foo-bar-1", "foo-0"}

			var instanceNameList []string
			for _, template := range templates {
				instanceNames, err := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, offlineInstances, templateName2OrdinalListMap[template.Name])
				Expect(err).Should(BeNil())
				instanceNameList = append(instanceNameList, instanceNames...)
			}
			getNameNOrdinalFunc := func(i int) (string, int) {
				return parseParentNameAndOrdinal(instanceNameList[i])
			}
			baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
			podNamesExpected := []string{"foo-1", "foo-2", "foo-bar-0", "foo-bar-2"}
			Expect(instanceNameList).Should(Equal(podNamesExpected))
		})

		It("w/ ordinals, unmatched replicas", func() {
			parentName := "foo"
			templateName := "bar"
			template := &instanceTemplateExt{
				Replicas: 5,
				Name:     templateName,
			}
			template2OrdinalList := []int32{0, 1, 2}

			_, err := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, nil, template2OrdinalList)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("expected 5 instance names but generated 3"))
		})

		It("w/ ordinals, zero replicas", func() {
			parentName := "foo"
			templateName := "bar"
			template := &instanceTemplateExt{
				Replicas: 0,
				Name:     templateName,
			}
			template2OrdinalList := []int32{0, 1, 2}

			instanceNames, err := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, nil, template2OrdinalList)
			Expect(err).Should(BeNil())
			Expect(instanceNames).Should(BeEmpty())
		})
	})

	Context("ConvertOrdinalsToSortedList", func() {
		It("should work well", func() {
			ordinals := kbappsv1.Ordinals{
				Ranges: []kbappsv1.Range{
					{
						Start: 2,
						End:   4,
					},
				},
				Discrete: []int32{0, 6},
			}
			ordinalList, err := ConvertOrdinalsToSortedList(ordinals)
			Expect(err).Should(BeNil())
			sets.New(ordinalList...).Equal(sets.New[int32](0, 2, 3, 4, 6))
		})

		It("rightNumber must >= leftNumber", func() {
			ordinals := kbappsv1.Ordinals{
				Ranges: []kbappsv1.Range{
					{
						Start: 4,
						End:   2,
					},
				},
				Discrete: []int32{0},
			}
			ordinalList, err := ConvertOrdinalsToSortedList(ordinals)
			errExpected := fmt.Errorf("range's end(%v) must >= start(%v)", 2, 4)
			Expect(err).Should(Equal(errExpected))
			Expect(ordinalList).Should(BeNil())
		})
	})

	Context("parseParentNameAndOrdinal", func() {
		It("Benchmark", Serial, Label("measurement"), func() {
			experiment := gmeasure.NewExperiment("parseParentNameAndOrdinal Benchmark")
			AddReportEntry(experiment.Name, experiment)

			experiment.Sample(func(idx int) {
				experiment.MeasureDuration("parseParentNameAndOrdinal", func() {
					_, _ = parseParentNameAndOrdinal("foo-bar-666")
				})
			}, gmeasure.SamplingConfig{N: 100, Duration: time.Second})

			parsingStats := experiment.GetStats("parseParentNameAndOrdinal")
			medianDuration := parsingStats.DurationFor(gmeasure.StatMedian)
			Expect(medianDuration).To(BeNumerically("<", time.Millisecond))
		})
	})

	Context("isImageMatched", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).GetObject()

			By("spec: image name, status: hostname, image name, tag, digest")
			pod.Spec.Containers = []corev1.Container{{
				Name:  name,
				Image: "nginx",
			}}
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Name:  name,
				Image: "docker.io/nginx:latest@0f37a86c04f8",
			}}
			Expect(isImageMatched(pod)).Should(BeTrue())

			By("exactly match w/o registry and repository")
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Name:  name,
				Image: "nginx",
			}}
			Expect(isImageMatched(pod)).Should(BeTrue())

			By("digest not matches")
			pod.Spec.Containers = []corev1.Container{{
				Name:  name,
				Image: "nginx:latest@xxxxxxxxx",
			}}
			Expect(isImageMatched(pod)).Should(BeFalse())

			By("tag not matches")
			pod.Spec.Containers = []corev1.Container{{
				Name:  name,
				Image: "nginx:xxxx@0f37a86c04f8",
			}}
			Expect(isImageMatched(pod)).Should(BeFalse())

			By("hostname not matches")
			pod.Spec.Containers = []corev1.Container{{
				Name:  name,
				Image: "apecloud.com/nginx",
			}}
			Expect(isImageMatched(pod)).Should(BeTrue())
		})
	})

	Context("isRoleReady", func() {
		It("should work well", func() {
			pod := builder.NewPodBuilder(namespace, name).GetObject()
			Expect(isRoleReady(pod, nil)).Should(BeTrue())
			Expect(isRoleReady(pod, roles)).Should(BeFalse())
			pod.Labels = map[string]string{constant.RoleLabelKey: "leader"}
			Expect(isRoleReady(pod, roles)).Should(BeTrue())
		})
	})
})
