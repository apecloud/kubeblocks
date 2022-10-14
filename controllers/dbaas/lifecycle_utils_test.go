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

	"github.com/leaanthony/debme"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
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

var _ = Describe("create", func() {
	Context("addLogSidecarContainers", func() {
		var params createParams
		var sts appsv1.StatefulSet

		BeforeEach(func() {
			cacheCtx := make(map[string]interface{})
			params = createParams{
				clusterDefinition: nil,
				appVersion:        nil,
				cluster: &dbaasv1alpha1.Cluster{
					Spec: dbaasv1alpha1.ClusterSpec{
						LogsEnable: true,
					},
				},
				component: &Component{
					LogsConfig: []dbaasv1alpha1.LogConfig{
						{
							Name:      "mysql-errorlog",
							FilePath:  "/data/mysql/log/mysql-error.log",
							Variables: []string{"log_error=/data/mysql/log/mysqld.err"},
						},
					},
				},
				roleGroup: nil,
				applyObjs: nil,
				cacheCtx:  &cacheCtx,
			}
			sts = appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{},
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
		})
		It("Log enable is false", func() {
			params.cluster.Spec.LogsEnable = false
			err := addLogSidecarContainers(params, &sts)
			Expect(err).To(BeNil())
			Expect(len(sts.Spec.Template.Spec.Containers)).To(Equal(1))
		})
		It("Log enable is true and current case", func() {
			err := addLogSidecarContainers(params, &sts)
			Expect(err).To(BeNil())
			Expect(len(sts.Spec.Template.Spec.Containers)).To(Equal(2))
			Expect(sts.Spec.Template.Spec.Volumes).ShouldNot(BeEmpty())
		})
	})
})
