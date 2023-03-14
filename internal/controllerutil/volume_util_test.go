/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllerutil

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("lifecycle_utils", func() {

	Context("has the checkAndUpdatePodVolumes function which generates Pod Volumes for mounting ConfigMap objects", func() {
		var sts appsv1.StatefulSet
		var volumes map[string]appsv1alpha1.ComponentTemplateSpec
		BeforeEach(func() {
			sts = appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name:            "mysql",
									Image:           "docker.io/apecloud/apecloud-mysql-server:latest",
									ImagePullPolicy: "IfNotPresent",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/data",
										},
									},
								},
							},
						},
					},
				},
			}
			volumes = make(map[string]appsv1alpha1.ComponentTemplateSpec)

		})

		It("should succeed in corner case where input volumes is nil, which means no volume is added", func() {
			ps := &sts.Spec.Template.Spec
			err := CreateOrUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(1))
		})

		It("should succeed in normal test case, where one volume is added", func() {
			volumes["my_config"] = appsv1alpha1.ComponentTemplateSpec{
				Name:        "myConfig",
				TemplateRef: "myConfig",
				VolumeName:  "myConfigVolume",
			}
			ps := &sts.Spec.Template.Spec
			err := CreateOrUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(2))
		})

		It("should succeed in normal test case, where two volumes are added", func() {
			volumes["my_config"] = appsv1alpha1.ComponentTemplateSpec{
				Name:        "myConfig",
				TemplateRef: "myConfig",
				VolumeName:  "myConfigVolume",
			}
			volumes["my_config1"] = appsv1alpha1.ComponentTemplateSpec{
				Name:        "myConfig",
				TemplateRef: "myConfig",
				VolumeName:  "myConfigVolume2",
			}
			ps := &sts.Spec.Template.Spec
			err := CreateOrUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(ps.Volumes)).To(Equal(3))
		})

		It("should fail if updated volume doesn't contain ConfigMap", func() {
			const (
				cmName            = "my_config_for_test"
				replicaVolumeName = "mytest-cm-volume_for_test"
			)
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
			volumes[cmName] = appsv1alpha1.ComponentTemplateSpec{
				Name:        "configTplName",
				TemplateRef: "configTplName",
				VolumeName:  replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			Expect(CreateOrUpdatePodVolumes(ps, volumes)).ShouldNot(Succeed())
		})

		It("should succeed if updated volume contains ConfigMap", func() {
			const (
				cmName            = "my_config_for_isv"
				replicaVolumeName = "mytest-cm-volume_for_isv"
			)

			// mock clusterdefinition has volume
			sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: replicaVolumeName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "anything"},
						},
					},
				})

			volumes[cmName] = appsv1alpha1.ComponentTemplateSpec{
				Name:        "configTplName",
				TemplateRef: "configTplName",
				VolumeName:  replicaVolumeName,
			}
			ps := &sts.Spec.Template.Spec
			err := CreateOrUpdatePodVolumes(ps, volumes)
			Expect(err).Should(BeNil())
			Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(2))
			volume := GetVolumeMountName(sts.Spec.Template.Spec.Volumes, cmName)
			Expect(volume).ShouldNot(BeNil())
			Expect(volume.ConfigMap).ShouldNot(BeNil())
			Expect(volume.ConfigMap.Name).Should(BeEquivalentTo(cmName))
			Expect(volume.Name).Should(BeEquivalentTo(replicaVolumeName))
		})

	})
})
