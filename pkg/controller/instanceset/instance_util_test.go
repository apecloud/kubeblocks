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

package instanceset

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("instance util test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetService(&corev1.Service{}).
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

	Context("ValidateDupInstanceNames", func() {
		It("should work well", func() {
			By("build name list without duplication")
			replicas := []string{"pod-0", "pod-1"}
			Expect(ValidateDupInstanceNames(replicas, func(item string) string {
				return item
			})).Should(Succeed())

			By("add a duplicate name")
			replicas = append(replicas, "pod-0")
			Expect(ValidateDupInstanceNames(replicas, func(item string) string {
				return item
			})).ShouldNot(Succeed())
		})
	})

	Context("buildInstanceName2TemplateMap", func() {
		It("build an its with default template only", func() {
			itsExt, err := buildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := buildInstanceName2TemplateMap(itsExt)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := its.Name + "-0"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(its.Name + "-1"))
			Expect(nameTemplate).Should(HaveKey(its.Name + "-2"))
			nameTemplate[name0].PodTemplateSpec.Spec.Volumes = nil
			envConfigName := GetEnvConfigMapName(its.Name)
			defaultTemplate := BuildPodTemplate(its, envConfigName)
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(defaultTemplate.Spec))
		})

		It("build an its with one instance template override", func() {
			nameOverride := "name-override"
			nameOverride0 := its.Name + "-" + nameOverride + "-0"
			annotationOverride := map[string]string{
				"foo": "bar",
			}
			labelOverride := map[string]string{
				"foo": "bar",
			}
			imageOverride := "foo:latest"
			resources := corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("600m"),
				},
			}
			instance := workloads.InstanceTemplate{
				Name:        nameOverride,
				Annotations: annotationOverride,
				Labels:      labelOverride,
				Image:       &imageOverride,
				Resources:   &resources,
			}
			its.Spec.Instances = append(its.Spec.Instances, instance)
			itsExt, err := buildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := buildInstanceName2TemplateMap(itsExt)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name0 := its.Name + "-0"
			name1 := its.Name + "-1"
			Expect(nameTemplate).Should(HaveKey(name0))
			Expect(nameTemplate).Should(HaveKey(name1))
			Expect(nameTemplate).Should(HaveKey(nameOverride0))
			envConfigName := GetEnvConfigMapName(its.Name)
			expectedTemplate := BuildPodTemplate(its, envConfigName)
			Expect(nameTemplate[name0].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[name1].PodTemplateSpec.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Annotations).Should(Equal(annotationOverride))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Labels).Should(Equal(labelOverride))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec.Containers[0].Image).Should(Equal(imageOverride))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]).Should(Equal(resources.Limits[corev1.ResourceCPU]))
			Expect(nameTemplate[nameOverride0].PodTemplateSpec.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]).Should(Equal(its.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]))
		})
	})

	Context("buildInstanceByTemplate", func() {
		It("should work well", func() {
			itsExt, err := buildInstanceSetExt(its, nil)
			Expect(err).Should(BeNil())
			nameTemplate, err := buildInstanceName2TemplateMap(itsExt)
			Expect(err).Should(BeNil())
			Expect(nameTemplate).Should(HaveLen(3))
			name := name + "-0"
			Expect(nameTemplate).Should(HaveKey(name))
			template := nameTemplate[name]
			replica, err := buildInstanceByTemplate(name, template, its, "")
			Expect(err).Should(BeNil())
			Expect(replica.pod).ShouldNot(BeNil())
			Expect(replica.pvcs).ShouldNot(BeNil())
			Expect(replica.pvcs).Should(HaveLen(1))
			Expect(replica.pod.Name).Should(Equal(name))
			Expect(replica.pod.Namespace).Should(Equal(its.Namespace))
			Expect(replica.pod.Spec.Volumes).Should(HaveLen(1))
			Expect(replica.pod.Spec.Volumes[0].Name).Should(Equal(volumeClaimTemplates[0].Name))
			envConfigName := GetEnvConfigMapName(its.Name)
			expectedTemplate := BuildPodTemplate(its, envConfigName)
			Expect(replica.pod.Spec).ShouldNot(Equal(expectedTemplate.Spec))
			// reset pod.volumes, pod.hostname and pod.subdomain
			replica.pod.Spec.Volumes = nil
			replica.pod.Spec.Hostname = ""
			replica.pod.Spec.Subdomain = ""
			Expect(replica.pod.Spec).Should(Equal(expectedTemplate.Spec))
			Expect(replica.pvcs[0].Name).Should(Equal(fmt.Sprintf("%s-%s", volumeClaimTemplates[0].Name, replica.pod.Name)))
			Expect(replica.pvcs[0].Labels[constant.VolumeClaimTemplateNameLabelKey]).Should(Equal(volumeClaimTemplates[0].Name))
			Expect(replica.pvcs[0].Spec.Resources).Should(Equal(volumeClaimTemplates[0].Spec.Resources))
		})
	})

	Context("validateSpec", func() {
		It("should work well", func() {
			By("a valid spec")
			Expect(validateSpec(its, nil)).Should(Succeed())

			By("sum of replicas in instance exceeds spec.replicas")
			its2 := its.DeepCopy()
			replicas := int32(4)
			name := "barrrrr"
			instance := workloads.InstanceTemplate{
				Name:     name,
				Replicas: &replicas,
			}
			its2.Spec.Instances = append(its2.Spec.Instances, instance)
			err := validateSpec(its2, nil)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("should not greater than replicas in spec"))
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

	Context("getInstanceTemplates", func() {
		It("should work well", func() {
			By("prepare objects")
			templateObj, annotation, err := mockCompressedInstanceTemplates(namespace, name)
			Expect(err).Should(BeNil())
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
			its := builder.NewInstanceSetBuilder(namespace, name).
				AddAnnotations(templateRefAnnotationKey, annotation).
				SetInstances(instances).
				GetObject()
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			Expect(tree.Add(templateObj)).Should(Succeed())

			By("parse instance templates")
			template, err := findTemplateObject(its, tree)
			Expect(err).Should(BeNil())
			instanceTemplates := getInstanceTemplates(its.Spec.Instances, template)
			// append templates from mock function
			instances = append(instances, []workloads.InstanceTemplate{
				{
					Name:     "foo",
					Replicas: func() *int32 { r := int32(2); return &r }(),
				},
				{
					Name:     "bar0",
					Replicas: func() *int32 { r := int32(1); return &r }(),
					Image:    func() *string { i := "busybox"; return &i }(),
				},
			}...)
			Expect(instanceTemplates).Should(Equal(instances))
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
				instanceNames := GenerateInstanceNamesFromTemplate(parentName, template.Name, template.Replicas, offlineInstances)
				instanceNameList = append(instanceNameList, instanceNames...)
			}
			getNameNOrdinalFunc := func(i int) (string, int) {
				return ParseParentNameAndOrdinal(instanceNameList[i])
			}
			baseSort(instanceNameList, getNameNOrdinalFunc, nil, true)
			podNamesExpected := []string{"foo-1", "foo-2", "foo-bar-0", "foo-bar-2"}
			Expect(instanceNameList).Should(Equal(podNamesExpected))
		})
	})

	Context("GenerateAllInstanceNames", func() {
		It("should work well", func() {
			parentName := "foo"
			templatesFoo := &workloads.InstanceTemplate{
				Name:     "foo",
				Replicas: pointer.Int32(1),
			}
			templateBar := &workloads.InstanceTemplate{
				Name:     "bar",
				Replicas: pointer.Int32(2),
			}
			var templates []InstanceTemplate
			templates = append(templates, templatesFoo, templateBar)
			offlineInstances := []string{"foo-bar-1", "foo-0"}
			instanceNameList := GenerateAllInstanceNames(parentName, 5, templates, offlineInstances)

			podNamesExpected := []string{"foo-1", "foo-2", "foo-bar-0", "foo-bar-2", "foo-foo-0"}
			Expect(instanceNameList).Should(Equal(podNamesExpected))
		})
	})
})
