/*
Copyright 2022 The KubeBlocks Authors

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

package dbaas

import (
	"testing"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/leaanthony/debme"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var tlog = ctrl.Log.WithName("lifecycle_util_testing")

func TestReadCUETplFromEmbeddedFS(t *testing.T) {
	cueFS, err := debme.FS(cueTemplates, "cue")
	if err != nil {
		t.Error("Expected no error", err)
	}
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("secret_template.cue"))

	if err != nil {
		t.Error("Expected no error", err)
	}

	tlog.Info("", "cueValue", cueTpl)
}

func TestCheckAndUpdatePodVolumes(t *testing.T) {
	var _ = Describe("lifecycle_utils", func() {
		var sts appsv1.StatefulSet
		var volumes map[string]dbaasv1alpha1.ConfigTemplate

		Context("TestCheckAndUpdatePodVolumes", func() {
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
										Image:           "docker.io/infracreate/wesql-server-8.0:0.1-SNAPSHOT",
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
				volumes = make(map[string]dbaasv1alpha1.ConfigTemplate)

			})

			It("Corner case volume is nil, and add no volume", func() {
				err := checkAndUpdatePodVolumes(&sts, volumes)
				Expect(err).Should(BeNil())
				Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(1))
			})

			It("Normal test case, and add one volume", func() {
				volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
					Name:       "myConfig",
					VolumeName: "myConfigVolume",
				}
				err := checkAndUpdatePodVolumes(&sts, volumes)
				Expect(err).Should(BeNil())
				Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(2))
			})

			It("Normal test case, and add two volume", func() {
				volumes["my_config"] = dbaasv1alpha1.ConfigTemplate{
					Name:       "myConfig",
					VolumeName: "myConfigVolume",
				}
				volumes["my_config1"] = dbaasv1alpha1.ConfigTemplate{
					Name:       "myConfig",
					VolumeName: "myConfigVolume",
				}
				err := checkAndUpdatePodVolumes(&sts, volumes)
				Expect(err).Should(BeNil())
				Expect(len(sts.Spec.Template.Spec.Volumes)).To(Equal(3))
			})
		})
	})
}
