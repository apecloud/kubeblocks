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

package configuration

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	ctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("tpl env template", func() {

	patroniTemplate := `
bootstrap:
  initdb:
    - auth-host: md5
    - auth-local: trust
`
	const (
		cmTemplateName   = "patroni-template-config"
		cmConfigFileName = "postgresql.yaml"
	)

	var (
		podSpec   *corev1.PodSpec
		component *ctrlcomp.SynthesizedComponent
		cluster   *appsv1.Cluster

		mockClient *testutil.K8sClientMockHelper
	)

	BeforeEach(func() {
		mockClient = testutil.NewK8sMockClient()

		mockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]coreclient.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmTemplateName,
					Namespace: "default",
				},
				Data: map[string]string{
					cmConfigFileName: patroniTemplate,
				}},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-config-env",
					Namespace: "default",
				},
				Data: map[string]string{
					"KB_0_HOSTNAME":    "my-mysql-0.my-mysql-headless",
					"KB_FOLLOWERS":     "",
					"KB_LEADER":        "my-mysql-0",
					"KB_REPLICA_COUNT": "1",
					"LOOP_REFERENCE_A": "$(LOOP_REFERENCE_B)",
					"LOOP_REFERENCE_B": "$(LOOP_REFERENCE_C)",
					"LOOP_REFERENCE_C": "$(LOOP_REFERENCE_A)",
				}},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-conn-credential",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"password": []byte("4zrqfl2r"),
					"username": []byte("root"),
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
							Name:  constant.KBEnvClusterName,
							Value: "my",
						},
						{
							Name:  constant.KBEnvCompName,
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
										Name: "my-conn-credential",
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
										Name: cmTemplateName,
									},
									Key: cmConfigFileName,
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
		cluster = &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my",
				UID:  "b006a20c-fb03-441c-bffa-2605cad7e297",
			},
		}
		component = &ctrlcomp.SynthesizedComponent{
			Name:        "mysql",
			ClusterName: cluster.Name,
		}
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
				ctx, mockClient.Client(),
			)

			localObjs := []coreclient.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmTemplateName,
						Namespace: "default",
					},
					Data: map[string]string{
						cmConfigFileName: patroniTemplate,
					}},
			}
			cfgBuilder.injectBuiltInObjectsAndFunctions(podSpec, component, localObjs,
				&appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my_test",
						Namespace: "default",
					},
				})

			rendered, err := cfgBuilder.render(map[string]string{
				// KB_CLUSTER_NAME, KB_COMP_NAME from env
				// MYSQL_USER,MYSQL_PASSWORD from valueFrom secret key
				// SPILO_CONFIGURATION from valueFrom configmap key
				// KB_LEADER from envFrom configmap
				// MEMORY_SIZE, CPU from resourceFieldRef
				"my":            fmt.Sprintf("{{ getEnvByName ( index $.podSpec.containers 0 ) \"%s\" }}", constant.KBEnvClusterName),
				"mysql":         fmt.Sprintf("{{ getEnvByName ( index $.podSpec.containers 0 ) \"%s\" }}", constant.KBEnvCompName),
				"root":          "{{ getEnvByName ( index $.podSpec.containers 0 ) \"MYSQL_USER\" }}",
				"4zrqfl2r":      "{{ getEnvByName ( index $.podSpec.containers 0 ) \"MYSQL_PASSWORD\" }}",
				patroniTemplate: "{{ getEnvByName ( index $.podSpec.containers 0 ) \"SPILO_CONFIGURATION\" }}",
				"my-mysql-0":    "{{ getEnvByName ( index $.podSpec.containers 0 ) \"KB_LEADER\" }}",

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
			Expect(err.Error()).Should(ContainSubstring("too many reference count, maybe there is a cycled reference"))
		})
	})
})
