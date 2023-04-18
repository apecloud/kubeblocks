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

package plan

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	ctrlcomp "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("tpl env template", func() {

	patroniTemplate := `
bootstrap:
  initdb:
    - auth-host: md5
    - auth-local: trust
`

	var (
		podSpec     *corev1.PodSpec
		cfgTemplate []appsv1alpha1.ComponentConfigSpec
		component   *ctrlcomp.SynthesizedComponent
		cluster     *appsv1alpha1.Cluster

		mockClient *testutil.K8sClientMockHelper
	)

	BeforeEach(func() {
		mockClient = testutil.NewK8sMockClient()

		mockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]coreclient.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "patroni-template-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"postgresql.yaml": patroniTemplate,
				}},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-config-env",
					Namespace: "default",
				},
				Data: map[string]string{
					"KB_MYSQL_0_HOSTNAME": "my-mysql-0.my-mysql-headless",
					"KB_MYSQL_FOLLOWERS":  "",
					"KB_MYSQL_LEADER":     "my-mysql-0",
					"KB_MYSQL_N":          "1",
					"KB_MYSQL_RECREATE":   "false",
					"LOOP_REFERENCE_A":    "$(LOOP_REFERENCE_B)",
					"LOOP_REFERENCE_B":    "$(LOOP_REFERENCE_C)",
					"LOOP_REFERENCE_C":    "$(LOOP_REFERENCE_A)",
				}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-conn-credential",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"password": []byte("NHpycWZsMnI="),
					"username": []byte("cm9vdA=="),
				}},
		}), testutil.WithAnyTimes()))

		// 2 configmap and 2 secret
		// Add any setup steps that needs to be executed before each test
		podSpec = &corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "mytest",
					Env: []corev1.EnvVar{
						{
							Name:  "KB_CLUSTER_NAME",
							Value: "my",
						},
						{
							Name:  "KB_COMP_NAME",
							Value: "mysql",
						},
						{
							Name: "MEMORY_SIZE",
							ValueFrom: &corev1.EnvVarSource{
								ResourceFieldRef: &corev1.ResourceFieldSelector{
									ContainerName: "mytest",
									Resource:      "limits.memory",
								},
							},
						},
						{
							Name: "CPU",
							ValueFrom: &corev1.EnvVarSource{
								ResourceFieldRef: &corev1.ResourceFieldSelector{
									Resource: "limits.cpu",
								},
							},
						},
						{
							Name: "CPU2",
							ValueFrom: &corev1.EnvVarSource{
								ResourceFieldRef: &corev1.ResourceFieldSelector{
									ContainerName: "not_exist_container",
									Resource:      "limits.memory",
								},
							},
						},
						{
							Name: "MYSQL_USER",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "my-conn-credential",
									},
									Key: "username",
								},
							},
						},
						{
							Name: "MYSQL_PASSWORD",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "$(CONN_CREDENTIAL_SECRET_NAME)",
									},
									Key: "password",
								},
							},
						},
						{
							Name: "SPILO_CONFIGURATION",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "patroni-template-config",
									},
									Key: "postgresql.yaml",
								},
							},
						},
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-config-env",
								},
							},
						},
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-secret-env",
								},
							},
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceMemory: resource.MustParse("8Gi"),
							corev1.ResourceCPU:    resource.MustParse("4"),
						},
					},
				},
				{
					Name: "invalid_container",
				},
			},
		}
		component = &ctrlcomp.SynthesizedComponent{
			Name: "mysql",
		}
		cluster = &appsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my",
			},
		}
		cfgTemplate = []appsv1alpha1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "mysql-config-8.0.2",
				TemplateRef: "mysql-config-8.0.2",
				VolumeName:  "config1",
			},
			ConfigConstraintRef: "mysql-config-8.0.2",
		}}
	})

	AfterEach(func() {
		mockClient.Finish()
	})

	// for test GetContainerWithVolumeMount
	Context("ConfigTemplateBuilder built-in env test", func() {
		It("test built-in function", func() {
			cfgBuilder := newTemplateBuilder(
				"my_test",
				"default",
				&appsv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my_test",
						Namespace: "default",
					},
				},
				nil, ctx, mockClient.Client(),
			)

			task := intctrltypes.InitReconcileTask(nil, nil, cluster, component)
			task.AppendResource(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "patroni-template-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"postgresql.yaml": patroniTemplate,
				}})
			Expect(cfgBuilder.injectBuiltInObjectsAndFunctions(podSpec, cfgTemplate, component, task)).Should(BeNil())

			rendered, err := cfgBuilder.render(map[string]string{
				// KB_CLUSTER_NAME, KB_COMP_NAME from env
				// MYSQL_USER,MYSQL_PASSWORD from valueFrom secret key
				// SPILO_CONFIGURATION from valueFrom configmap key
				// KB_MYSQL_LEADER from envFrom configmap
				// MEMORY_SIZE, CPU from resourceFieldRef
				"my":            "{{ getEnvByName ( index $.podSpec.containers 0 ) \"KB_CLUSTER_NAME\" }}",
				"mysql":         "{{ getEnvByName ( index $.podSpec.containers 0 ) \"KB_COMP_NAME\" }}",
				"root":          "{{ getEnvByName ( index $.podSpec.containers 0 ) \"MYSQL_USER\" }}",
				"4zrqfl2r":      "{{ getEnvByName ( index $.podSpec.containers 0 ) \"MYSQL_PASSWORD\" }}",
				patroniTemplate: "{{ getEnvByName ( index $.podSpec.containers 0 ) \"SPILO_CONFIGURATION\" }}",
				"my-mysql-0":    "{{ getEnvByName ( index $.podSpec.containers 0 ) \"KB_MYSQL_LEADER\" }}",

				strconv.Itoa(4):                      "{{ getEnvByName ( index $.podSpec.containers 0 ) \"CPU\" }}",
				strconv.Itoa(8 * 1024 * 1024 * 1024): "{{ getEnvByName ( index $.podSpec.containers 0 ) \"MEMORY_SIZE\" }}",
			})

			Expect(err).Should(Succeed())
			for key, value := range rendered {
				Expect(key).Should(BeEquivalentTo(value))
			}

			_, err = cfgBuilder.render(map[string]string{
				"error": "{{ getEnvByName ( index $.podSpec.containers 0 ) \"CPU2\" }}",
			})
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("not found named[not_exist_container] container"))

			_, err = cfgBuilder.render(map[string]string{
				"error_loop_reference": "{{ getEnvByName ( index $.podSpec.containers 0 ) \"LOOP_REFERENCE_A\" }}",
			})
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("too many reference count, maybe there is a loop reference"))
		})
	})
})
