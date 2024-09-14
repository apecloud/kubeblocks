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

package component

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("vars", func() {
	optional := func() *bool {
		o := true
		return &o
	}

	required := func() *bool {
		o := false
		return &o
	}

	expp := func(exp string) *string {
		return &exp
	}

	checkTemplateVars := func(templateVars map[string]any, targetVars []corev1.EnvVar) {
		templateVarsMapping := make(map[string]corev1.EnvVar)
		for k, v := range templateVars {
			val := ""
			if v != nil {
				val = v.(string)
			}
			templateVarsMapping[k] = corev1.EnvVar{Name: k, Value: val}
		}

		vars := make([]corev1.EnvVar, 0)
		for _, v := range targetVars {
			if templateVar, ok := templateVarsMapping[v.Name]; ok {
				vars = append(vars, templateVar)
			}
		}
		Expect(vars).Should(BeEquivalentTo(targetVars))
	}

	// without the order check
	checkEnvVars := func(envVars []corev1.EnvVar, targetEnvVars []corev1.EnvVar) {
		targetEnvVarMapping := map[string]corev1.EnvVar{}
		for i, env := range targetEnvVars {
			targetEnvVarMapping[env.Name] = targetEnvVars[i]
		}

		envVarMapping := map[string]corev1.EnvVar{}
		for i, env := range envVars {
			if _, ok := targetEnvVarMapping[env.Name]; ok {
				envVarMapping[env.Name] = envVars[i]
			}
		}
		Expect(envVarMapping).Should(BeEquivalentTo(targetEnvVarMapping))
	}

	checkEnvVarNotExist := func(envVars []corev1.EnvVar, envName string) {
		envVarMapping := map[string]any{}
		for _, env := range envVars {
			envVarMapping[env.Name] = true
		}
		Expect(envVarMapping).ShouldNot(HaveKey(envName))
	}

	checkEnvVarWithValue := func(envVars []corev1.EnvVar, envName, envValue string) {
		envVarMapping := map[string]string{}
		for _, env := range envVars {
			if env.ValueFrom == nil {
				envVarMapping[env.Name] = env.Value
			}
		}
		Expect(envVarMapping).Should(HaveKeyWithValue(envName, envValue))
	}

	checkEnvVarWithValueFrom := func(envVars []corev1.EnvVar, envName string, envValue *corev1.EnvVarSource) {
		envVarMapping := map[string]corev1.EnvVarSource{}
		nilEnvVarMapping := map[string]bool{}
		for _, env := range envVars {
			if env.ValueFrom != nil {
				envVarMapping[env.Name] = *env.ValueFrom
			} else {
				nilEnvVarMapping[env.Name] = true
			}
		}
		if envValue != nil {
			Expect(envVarMapping).Should(HaveKeyWithValue(envName, *envValue))
		} else {
			Expect(nilEnvVarMapping).Should(HaveKey(envName))
		}
	}

	Context("vars test", func() {
		var (
			synthesizedComp *SynthesizedComponent
		)

		BeforeEach(func() {
			synthesizedComp = &SynthesizedComponent{
				Namespace:    testCtx.DefaultNamespace,
				ClusterName:  "test-cluster",
				ClusterUID:   string(uuid.NewUUID()),
				Name:         "comp",
				FullCompName: "test-cluster-comp",
				CompDefName:  "compDef",
				Replicas:     1,
				PodSpec: &corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "init",
							Env: []corev1.EnvVar{
								{
									Name:  "placeholder",
									Value: "placeholder",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name: "01",
							Env: []corev1.EnvVar{
								{
									Name:  "placeholder",
									Value: "placeholder",
								},
							},
						},
						{
							Name: "02",
							Env: []corev1.EnvVar{
								{
									Name:  "placeholder",
									Value: "placeholder",
								},
							},
						},
					},
				},
			}
		})

		It("default vars", func() {
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil)
			Expect(err).Should(Succeed())

			By("check default template vars")
			checkTemplateVars(templateVars, builtinTemplateVars(synthesizedComp, nil))

			By("check default env vars")
			targetEnvVars := builtinTemplateVars(synthesizedComp, nil)
			targetEnvVars = append(targetEnvVars, buildDefaultEnvVars(synthesizedComp, false)...)
			checkEnvVars(envVars, targetEnvVars)
		})

		It("TLS env vars", func() {
			synthesizedComp.TLSConfig = &appsv1.TLSConfig{
				Enable: true,
			}
			_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil)
			Expect(err).Should(Succeed())
			checkEnvVars(envVars, buildEnv4TLS(synthesizedComp))
		})

		It("user-defined env vars", func() {
			By("invalid")
			annotations := map[string]string{
				constant.ExtraEnvAnnotationKey: "invalid-json-format",
			}
			synthesizedComp.Annotations = annotations
			_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			data, _ := json.Marshal(map[string]string{
				"user-defined-var": "user-defined-value",
			})
			annotations = map[string]string{
				constant.ExtraEnvAnnotationKey: string(data),
			}
			synthesizedComp.Annotations = annotations
			_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil)
			Expect(err).Should(Succeed())
			checkEnvVars(envVars, []corev1.EnvVar{{Name: "user-defined-var", Value: "user-defined-value"}})
		})

		It("component-ref env vars", func() {})

		It("configmap vars", func() {
			By("non-exist configmap with optional")
			vars := []appsv1.EnvVar{
				{
					Name: "non-exist-cm-var",
					ValueFrom: &appsv1.VarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "non-exist",
							},
							Key:      "non-exist",
							Optional: optional(),
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-cm-var"))
			checkEnvVarNotExist(envVars, "non-exist-cm-var")

			By("non-exist configmap with required")
			vars = []appsv1.EnvVar{
				{
					Name: "non-exist-cm-var",
					ValueFrom: &appsv1.VarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "non-exist",
							},
							Key:      "non-exist",
							Optional: required(),
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1.EnvVar{
				{
					Name: "cm-var",
					ValueFrom: &appsv1.VarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "cm",
							},
							Key: "cm-key",
						},
					},
				},
			}
			reader := &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "cm",
						},
						Data: map[string]string{
							"cm-key": "cm-var-value",
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("cm-var", "cm-var-value"))
			checkEnvVarWithValue(envVars, "cm-var", "cm-var-value")
		})

		It("secret vars", func() {
			By("non-exist secret with optional")
			vars := []appsv1.EnvVar{
				{
					Name: "non-exist-secret-var",
					ValueFrom: &appsv1.VarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "non-exist",
							},
							Key:      "non-exist",
							Optional: optional(),
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-secret-var"))
			checkEnvVarNotExist(envVars, "non-exist-secret-var")

			By("non-exist secret with required")
			vars = []appsv1.EnvVar{
				{
					Name: "non-exist-secret-var",
					ValueFrom: &appsv1.VarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "non-exist",
							},
							Key:      "non-exist",
							Optional: required(),
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1.EnvVar{
				{
					Name: "secret-var",
					ValueFrom: &appsv1.VarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "secret",
							},
							Key: "secret-key",
						},
					},
				},
			}
			reader := &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "secret",
						},
						Data: map[string][]byte{
							"secret-key": []byte("secret-var-value"),
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKeyWithValue("secret-var", "secret-var-value"))
			checkEnvVarWithValueFrom(envVars, "secret-var", &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "secret",
					},
					Key: "secret-key",
				},
			})
		})

		It("host-network vars", func() {
			vars := []appsv1.EnvVar{
				{
					Name:  "host-network-port",
					Value: "3306", // default value
					ValueFrom: &appsv1.VarSource{
						HostNetworkVarRef: &appsv1.HostNetworkVarSelector{
							ClusterObjectReference: appsv1.ClusterObjectReference{
								Optional: required(),
							},
							HostNetworkVars: appsv1.HostNetworkVars{
								Container: &appsv1.ContainerVars{
									Name: "default",
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				},
			}

			By("has no host-network capability")
			_, _, err := ResolveTemplateNEnvVars(ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(And(ContainSubstring("has no HostNetwork"), ContainSubstring("found when resolving vars")))

			By("has no host-network enabled")
			synthesizedComp.HostNetwork = &appsv1.HostNetwork{}
			_, _, err = ResolveTemplateNEnvVars(ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(And(ContainSubstring("has no HostNetwork"), ContainSubstring("found when resolving vars")))

			By("has no host-network port")
			synthesizedComp.Annotations = map[string]string{
				constant.HostNetworkAnnotationKey: synthesizedComp.Name,
			}
			_, _, err = ResolveTemplateNEnvVars(ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("the required var is not found"))

			By("ok")
			ctx := mockHostNetworkPort(testCtx.Ctx, testCtx.Cli,
				synthesizedComp.ClusterName, synthesizedComp.Name, "default", "default", 30001)
			templateVars, envVars, err := ResolveTemplateNEnvVars(ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("host-network-port", "30001"))
			checkEnvVarWithValue(envVars, "host-network-port", "30001")

			By("w/ default value - has host-network port")
			vars = []appsv1.EnvVar{
				{
					Name:  "host-network-port",
					Value: "3306", // default value
					ValueFrom: &appsv1.VarSource{
						HostNetworkVarRef: &appsv1.HostNetworkVarSelector{
							ClusterObjectReference: appsv1.ClusterObjectReference{
								Optional: optional(), // optional
							},
							HostNetworkVars: appsv1.HostNetworkVars{
								Container: &appsv1.ContainerVars{
									Name: "default",
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("host-network-port", "30001"))
			checkEnvVarWithValue(envVars, "host-network-port", "30001")

			By("w/ default value - back-off to default value")
			synthesizedComp.Annotations = nil // disable the host-network
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("host-network-port", "3306"))
			checkEnvVarWithValue(envVars, "host-network-port", "3306")
		})

		Context("service vars", func() {
			var (
				reader *mockReader
			)

			BeforeEach(func() {
				comp := &appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
						Name:      FullName(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: appsv1.ComponentSpec{
						CompDef: synthesizedComp.CompDefName,
					},
				}
				compDef := &appsv1.ComponentDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: synthesizedComp.CompDefName,
					},
					Spec: appsv1.ComponentDefinitionSpec{
						Services: []appsv1.ComponentService{},
					},
				}
				for _, name := range []string{"non-exist", "service", "service-wo-port-name", "pod-service", "lb", "advertised"} {
					compDef.Spec.Services = append(compDef.Spec.Services, appsv1.ComponentService{
						Service: appsv1.Service{
							Name:        name,
							ServiceName: name,
						},
					})
				}
				reader = &mockReader{
					cli:  testCtx.Cli,
					objs: []client.Object{comp, compDef},
				}
			})

			It("non-exist service - optional", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-service-var",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: optional(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("non-exist-service-var"))
				checkEnvVarNotExist(envVars, "non-exist-service-var")
			})

			It("non-exist service - required", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-service-var",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
			})

			It("has no service defined", func() {
				By("optional")
				vars := []appsv1.EnvVar{
					{
						Name: "not-defined-service",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "not-defined", // the service has not been defined in the componentDefinition
									Optional: optional(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(ContainSubstring("not defined in the component definition"))

				By("required")
				vars = []appsv1.EnvVar{
					{
						Name: "not-defined-service",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "not-defined", // the service has not been defined in the componentDefinition
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(ContainSubstring("not defined in the component definition"))
			})

			It("ok", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "service-type",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									ServiceType: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "service-port",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
					{
						Name: "service-port-wo-name",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service-wo-port-name",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, []client.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      svcName,
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeClusterIP,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service-wo-port-name"),
						},
						Spec: corev1.ServiceSpec{
							Ports: []corev1.ServicePort{
								{
									Port: int32(svcPort + 1),
								},
							},
						},
					},
				}...)
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-type", string(corev1.ServiceTypeClusterIP)))
				Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
				Expect(templateVars).Should(HaveKeyWithValue("service-port", strconv.Itoa(svcPort)))
				Expect(templateVars).Should(HaveKeyWithValue("service-port-wo-name", strconv.Itoa(svcPort+1)))
				checkEnvVarWithValue(envVars, "service-type", string(corev1.ServiceTypeClusterIP))
				checkEnvVarWithValue(envVars, "service-host", svcName)
				checkEnvVarWithValue(envVars, "service-port", strconv.Itoa(svcPort))
				checkEnvVarWithValue(envVars, "service-port-wo-name", strconv.Itoa(svcPort+1))
			})

			It("ok - different name and service name", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "quorum")
				svcPort := 3306

				compDef := reader.objs[1].(*appsv1.ComponentDefinition)
				compDef.Spec.Services = append(compDef.Spec.Services, appsv1.ComponentService{
					Service: appsv1.Service{
						Name:        "client", // name and service name are different
						ServiceName: "quorum",
					},
				})

				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "client", // should use the service.name
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
				})
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
				checkEnvVarWithValue(envVars, "service-host", svcName)
			})

			It("node port", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")
				svcPort := 3306
				nodePort := 30001

				vars := []appsv1.EnvVar{
					{
						Name: "service-node-port",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								Name:     "default",
								Port:     int32(svcPort),
								NodePort: int32(nodePort),
							},
						},
					},
				})
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-node-port", strconv.Itoa(nodePort)))
				checkEnvVarWithValue(envVars, "service-node-port", strconv.Itoa(nodePort))
			})

			It("node port in provisioning", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")
				svcPort := 3306
				nodePort := 30001

				vars := []appsv1.EnvVar{
					{
						Name: "service-node-port",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
								// the node port is not assigned
							},
						},
					},
				})
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("the required var is not found"))

				// assign a node port to the service
				reader.objs[len(reader.objs)-1].(*corev1.Service).Spec.Ports[0].NodePort = int32(nodePort + 1)

				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-node-port", strconv.Itoa(nodePort+1)))
				checkEnvVarWithValue(envVars, "service-node-port", strconv.Itoa(nodePort+1))
			})

			It("pod service", func() {
				svcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "pod-service-0")
				svcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "pod-service-1")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "pod-service-type",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "pod-service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									ServiceType: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "pod-service-endpoint",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "pod-service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "pod-service-port",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "pod-service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Port: &appsv1.NamedVar{
										Name:   "default",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, []client.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      svcName0,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
							Ports: []corev1.ServicePort{
								{
									Name:     "default",
									Port:     int32(svcPort),
									NodePort: 300001,
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      svcName1,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
							Ports: []corev1.ServicePort{
								{
									// Name:     "default",  // don't set the port name
									Port:     int32(svcPort + 1),
									NodePort: 300002,
								},
							},
						},
					},
				}...)
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				checkEnvVarWithValue(envVars, "pod-service-type", string(corev1.ServiceTypeNodePort))
				checkEnvVarWithValue(envVars, "pod-service-endpoint", strings.Join([]string{svcName0, svcName1}, ","))
				checkEnvVarWithValue(envVars, "pod-service-port", strings.Join([]string{fmt.Sprintf("%s:300001", svcName0), fmt.Sprintf("%s:300002", svcName1)}, ","))
			})

			It("load balancer", func() {
				lbSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "lb",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "lb",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      lbSvcName,
						Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									IP: "127.0.0.1",
								},
							},
						},
					},
				})
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				checkEnvVarWithValue(envVars, "lb", "127.0.0.1")
			})

			It("load balancer - pod service", func() {
				lbSvcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-0")
				lbSvcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-1")
				lbSvcName2 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-2")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "lb",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "lb",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, []client.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      lbSvcName0,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										IP: "127.0.0.1", // IP
									},
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      lbSvcName1,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										Hostname: "127.0.0.2", // hostname
									},
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      lbSvcName2,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{ // more than one ingress points
									{
										IP: "127.0.0.4", // IP
									},
									{
										Hostname: "127.0.0.3", // hostname
									},
								},
							},
						},
					},
				}...)
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				endpoints := []string{
					fmt.Sprintf("%s:127.0.0.1", lbSvcName0),
					fmt.Sprintf("%s:127.0.0.2", lbSvcName1),
					fmt.Sprintf("%s:127.0.0.4", lbSvcName2),
				}
				checkEnvVarWithValue(envVars, "lb", strings.Join(endpoints, ","))
			})

			It("load balancer - pod service in provisioning", func() {
				lbSvcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-0")
				lbSvcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-1")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "lb",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "lb",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, []client.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      lbSvcName0,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
						Status: corev1.ServiceStatus{
							LoadBalancer: corev1.LoadBalancerStatus{
								Ingress: []corev1.LoadBalancerIngress{
									{
										IP: "127.0.0.1", // IP
									},
								},
							},
						},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      lbSvcName1,
							Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
							Ports: []corev1.ServicePort{
								{
									Name: "default",
									Port: int32(svcPort),
								},
							},
						},
						Status: corev1.ServiceStatus{}, // has no load balancer status, may be in provisioning
					},
				}...)
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("the required var is not found"))
			})

			It("adaptive - has load balancer pod service", func() {
				advertisedSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "advertised-0")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "advertised",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "advertised",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host:         &appsv1.VarRequired, // both host and loadBalancer
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      advertisedSvcName,
						Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									IP: "127.0.0.1", // IP
								},
							},
						},
					},
				})
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				endpoints := []string{
					fmt.Sprintf("%s:127.0.0.1", advertisedSvcName),
				}
				checkEnvVarWithValue(envVars, "advertised", strings.Join(endpoints, ","))
			})

			It("adaptive - has no load balancer service", func() {
				advertisedSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "advertised-0")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "advertised",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "advertised",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host:         &appsv1.VarRequired, // both host and loadBalancer
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      advertisedSvcName,
						Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeClusterIP,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
				})
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				checkEnvVarWithValue(envVars, "advertised", advertisedSvcName)
			})

			It("adaptive - has load balancer service in provisioning", func() {
				// non pod-service
				advertisedSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "advertised")
				svcPort := 3306

				vars := []appsv1.EnvVar{
					{
						Name: "advertised",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "advertised",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host:         &appsv1.VarRequired, // both host and loadBalancer
									LoadBalancer: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      advertisedSvcName,
						Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
					Status: corev1.ServiceStatus{}, // has no load balancer status, may be in provisioning
				})
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(ContainSubstring("the required var is not found"))

				reader.objs[len(reader.objs)-1] = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      advertisedSvcName,
						Labels:    constant.GetComponentWellKnownLabels(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{
								Name: "default",
								Port: int32(svcPort),
							},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									IP: "127.0.0.1", // IP
								},
							},
						},
					},
				}
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				checkEnvVarWithValue(envVars, "advertised", "127.0.0.1")
			})
		})

		Context("credential vars", func() {
			It("non-exist credential with optional", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-credential-var",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: optional(),
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("non-exist-credential-var"))
				checkEnvVarNotExist(envVars, "non-exist-credential-var")
			})

			It("non-exist credential with required", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-credential-var",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: required(),
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
			})

			It("ok", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "credential-username",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: required(),
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "credential-password",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: required(),
								},
								CredentialVars: appsv1.CredentialVars{
									Password: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, "credential"),
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret:   []byte("username"),
								constant.AccountPasswdForSecret: []byte("password"),
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("credential-username"))
				Expect(templateVars).ShouldNot(HaveKey("credential-password"))
				checkEnvVarWithValueFrom(envVars, "credential-username", &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: reader.objs[0].GetName(),
						},
						Key: constant.AccountNameForSecret,
					},
				})
				checkEnvVarWithValueFrom(envVars, "credential-password", &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: reader.objs[0].GetName(),
						},
						Key: constant.AccountPasswdForSecret,
					},
				})
			})
		})

		Context("service-ref vars", func() {
			It("non-exist service-ref with optional", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-serviceref-var",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: optional(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									Endpoint: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("non-exist-serviceref-var"))
				checkEnvVarNotExist(envVars, "non-exist-serviceref-var")
			})

			It("non-exist service-ref with required", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-serviceref-var",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									Endpoint: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
			})

			It("ok", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "serviceref-endpoint",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "serviceref",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									Endpoint: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "serviceref-host",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "serviceref",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "serviceref-port",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "serviceref",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									Port: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "serviceref-username",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "serviceref",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									CredentialVars: appsv1.CredentialVars{
										Username: &appsv1.VarRequired,
									},
								},
							},
						},
					},
					{
						Name: "serviceref-password",
						ValueFrom: &appsv1.VarSource{
							ServiceRefVarRef: &appsv1.ServiceRefVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "serviceref",
									Optional: required(),
								},
								ServiceRefVars: appsv1.ServiceRefVars{
									CredentialVars: appsv1.CredentialVars{
										Password: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				synthesizedComp.ServiceReferences = map[string]*appsv1.ServiceDescriptor{
					"serviceref": {
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      "serviceref",
						},
						Spec: appsv1.ServiceDescriptorSpec{
							ServiceKind:    "",
							ServiceVersion: "",
							Endpoint: &appsv1.CredentialVar{
								Value: "endpoint",
							},
							Host: &appsv1.CredentialVar{
								Value: "host",
							},
							Port: &appsv1.CredentialVar{
								Value: "port",
							},
							Auth: &appsv1.ConnectionCredentialAuth{
								Username: &appsv1.CredentialVar{
									Value: "username",
								},
								Password: &appsv1.CredentialVar{
									Value: "password",
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("serviceref-endpoint", "endpoint"))
				Expect(templateVars).Should(HaveKeyWithValue("serviceref-host", "host"))
				Expect(templateVars).Should(HaveKeyWithValue("serviceref-port", "port"))
				Expect(templateVars).ShouldNot(HaveKey("serviceref-username"))
				Expect(templateVars).ShouldNot(HaveKey("serviceref-password"))
				checkEnvVarWithValue(envVars, "serviceref-endpoint", "endpoint")
				checkEnvVarWithValue(envVars, "serviceref-host", "host")
				checkEnvVarWithValue(envVars, "serviceref-port", "port")
				checkEnvVarWithValue(envVars, "serviceref-username", "username")
				checkEnvVarWithValue(envVars, "serviceref-password", "password")
			})
		})

		Context("component vars", func() {
			It("non-exist component with optional", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-component-var",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: optional(),
								},
								ComponentVars: appsv1.ComponentVars{
									Replicas: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("non-exist-component-var"))
				checkEnvVarNotExist(envVars, "non-exist-component-var")
			})

			It("non-exist component with required", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "non-exist-component-var",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "non-exist",
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									Replicas: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
			})

			It("ok", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "name",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									ComponentName: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "shortName",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									ShortName: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "replicas",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									Replicas: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "podNames",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									PodNames: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "podFQDNs",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									PodFQDNs: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "podNames4EmptyRole",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									PodNamesForRole: &appsv1.RoledVar{
										// empty role
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
					{
						Name: "podFQDNs4Leader",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Optional: required(),
								},
								ComponentVars: appsv1.ComponentVars{
									PodFQDNsForRole: &appsv1.RoledVar{
										Role:   "leader",
										Option: &appsv1.VarRequired,
									},
								},
							},
						},
					},
				}
				podName := func(suffix string) string {
					return fmt.Sprintf("%s-%s", constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name), suffix)
				}
				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&appsv1.Component{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name),
							},
							Spec: appsv1.ComponentSpec{
								CompDef:  synthesizedComp.CompDefName,
								Replicas: 3,
							},
						},
						&corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      podName("leader"),
								Labels: map[string]string{
									constant.AppManagedByLabelKey:   constant.AppName,
									constant.AppInstanceLabelKey:    synthesizedComp.ClusterName,
									constant.KBAppComponentLabelKey: synthesizedComp.Name,
									constant.RoleLabelKey:           "leader",
								},
							},
							Spec: corev1.PodSpec{},
						},
						&corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      podName("follower"),
								Labels: map[string]string{
									constant.AppManagedByLabelKey:   constant.AppName,
									constant.AppInstanceLabelKey:    synthesizedComp.ClusterName,
									constant.KBAppComponentLabelKey: synthesizedComp.Name,
									constant.RoleLabelKey:           "follower",
								},
							},
							Spec: corev1.PodSpec{},
						},
						&corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      podName("empty"),
								Labels: map[string]string{
									constant.AppManagedByLabelKey:   constant.AppName,
									constant.AppInstanceLabelKey:    synthesizedComp.ClusterName,
									constant.KBAppComponentLabelKey: synthesizedComp.Name,
								},
							},
							Spec: corev1.PodSpec{},
						},
					},
				}
				// pod names and FQDNs are calculated from the spec, and names and FQDNs for specific roles are obtained from runtime resources.
				mockInstanceList := []string{
					constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 0),
					constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 1),
					constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 2),
				}
				_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				compName := constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name)
				checkEnvVarWithValue(envVars, "name", compName)
				checkEnvVarWithValue(envVars, "shortName", synthesizedComp.Name)
				checkEnvVarWithValue(envVars, "replicas", fmt.Sprintf("%d", 3))
				checkEnvVarWithValue(envVars, "podNames", strings.Join(mockInstanceList, ","))
				checkEnvVarWithValue(envVars, "podNames4EmptyRole", podName("empty"))
				fqdnList := func(fn ...func(string) string) []string {
					l := make([]string, 0)
					for _, i := range mockInstanceList {
						l = append(l, PodFQDN(synthesizedComp.Namespace, synthesizedComp.FullCompName, i))
					}
					return l
				}
				checkEnvVarWithValue(envVars, "podFQDNs", strings.Join(fqdnList(), ","))
				checkEnvVarWithValue(envVars, "podFQDNs4Leader", PodFQDN(synthesizedComp.Namespace, synthesizedComp.FullCompName, podName("leader")))
			})
		})

		Context("cluster vars", func() {
			It("ok", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "namespace",
						ValueFrom: &appsv1.VarSource{
							ClusterVarRef: &appsv1.ClusterVarSelector{
								ClusterVars: appsv1.ClusterVars{
									Namespace: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "name",
						ValueFrom: &appsv1.VarSource{
							ClusterVarRef: &appsv1.ClusterVarSelector{
								ClusterVars: appsv1.ClusterVars{
									ClusterName: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "uid",
						ValueFrom: &appsv1.VarSource{
							ClusterVarRef: &appsv1.ClusterVarSelector{
								ClusterVars: appsv1.ClusterVars{
									ClusterUID: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("namespace", synthesizedComp.Namespace))
				Expect(templateVars).Should(HaveKeyWithValue("name", synthesizedComp.ClusterName))
				Expect(templateVars).Should(HaveKeyWithValue("uid", synthesizedComp.ClusterUID))
				checkEnvVarWithValue(envVars, "namespace", synthesizedComp.Namespace)
				checkEnvVarWithValue(envVars, "name", synthesizedComp.ClusterName)
				checkEnvVarWithValue(envVars, "uid", synthesizedComp.ClusterUID)
			})
		})

		Context("resolve component", func() {
			var (
				reader *mockReader
			)

			BeforeEach(func() {
				comp := &appsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: synthesizedComp.Namespace,
						Name:      FullName(synthesizedComp.ClusterName, synthesizedComp.Name),
					},
					Spec: appsv1.ComponentSpec{
						CompDef: synthesizedComp.CompDefName,
					},
				}
				compDef := &appsv1.ComponentDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: synthesizedComp.CompDefName,
					},
					Spec: appsv1.ComponentDefinitionSpec{
						Services: []appsv1.ComponentService{},
					},
				}
				for _, name := range []string{"service"} {
					compDef.Spec.Services = append(compDef.Spec.Services, appsv1.ComponentService{
						Service: appsv1.Service{
							Name:        name,
							ServiceName: name,
						},
					})
				}
				reader = &mockReader{
					cli:  testCtx.Cli,
					objs: []client.Object{comp, compDef},
				}
			})

			It("component not found w/ optional", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  "non-exist",
									Name:     "service",
									Optional: optional(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("service-hst"))
				checkEnvVarNotExist(envVars, "service-host")
			})

			It("component not found w/ required", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  "non-exist",
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
			})

			It("default component", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")

				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									// don't specify the comp def, it will match self by default
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Port: int32(3306),
							},
						},
					},
				})
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
				checkEnvVarWithValue(envVars, "service-host", svcName)
			})
		})

		Context("multiple components", func() {
			var (
				compName1, compName2, compName3 string
				svcName1, svcName2, svcName3    string
				secretName1, secretName2        string

				svcVarName1, svcVarName2, svcVarName3  string
				credentialVarName1, credentialVarName2 string

				combinedSvcVarValue                 string
				combinedSvcVarValueWithComp3        string
				combinedSvcVarValueWithComp3KeyOnly string

				reader *mockReader
			)

			BeforeEach(func() {
				compName1 = synthesizedComp.Name
				compName2 = synthesizedComp.Name + "-other"
				compName3 = synthesizedComp.Name + "-other-not-exist"

				svcName1 = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName1, "service")
				svcName2 = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName2, "service")
				svcName3 = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName3, "service")
				secretName1 = constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName1, "credential")
				secretName2 = constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName2, "credential")

				compVarName := func(compName, envName string) string {
					return fmt.Sprintf("%s_%s", envName, strings.ToUpper(strings.ReplaceAll(compName, "-", "_")))
				}
				svcVarName1 = compVarName(compName1, "service-host")
				svcVarName2 = compVarName(compName2, "service-host")
				svcVarName3 = compVarName(compName3, "service-host")
				credentialVarName1 = compVarName(compName1, "credential-username")
				credentialVarName2 = compVarName(compName2, "credential-username")

				combinedSvcVarValue = fmt.Sprintf("%s:%s,%s:%s", compName1, svcName1, compName2, svcName2)
				combinedSvcVarValueWithComp3 = fmt.Sprintf("%s:%s,%s:%s,%s:%s", compName1, svcName1, compName2, svcName2, compName3, svcName3)
				combinedSvcVarValueWithComp3KeyOnly = fmt.Sprintf("%s:%s,%s:%s,%s:", compName1, svcName1, compName2, svcName2, compName3)

				reader = &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      svcName1,
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Port: int32(3306),
									},
								},
							},
						},
						&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      svcName2,
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Port: int32(3306),
									},
								},
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      secretName1,
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret: []byte("username"),
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      secretName2,
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret: []byte("username"),
							},
						},
					},
				}
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1:       synthesizedComp.CompDefName,
					compName2:       synthesizedComp.CompDefName,
					"comp-other-01": "abc" + synthesizedComp.CompDefName,
					"comp-other-02": "abc" + synthesizedComp.CompDefName,
				}

				for _, name := range []string{compName1, compName2, compName3} {
					comp := &appsv1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: synthesizedComp.Namespace,
							Name:      FullName(synthesizedComp.ClusterName, name),
						},
						Spec: appsv1.ComponentSpec{
							CompDef: synthesizedComp.CompDefName,
						},
					}
					compDef := &appsv1.ComponentDefinition{
						ObjectMeta: metav1.ObjectMeta{
							Name: synthesizedComp.CompDefName,
						},
						Spec: appsv1.ComponentDefinitionSpec{
							Services: []appsv1.ComponentService{},
						},
					}
					for _, name := range []string{"service", svcName1, svcName2} {
						compDef.Spec.Services = append(compDef.Spec.Services, appsv1.ComponentService{
							Service: appsv1.Service{
								Name:        name,
								ServiceName: name,
							},
						})
					}
					reader.objs = append(reader.objs, comp, compDef)
				}
			})

			It("w/o option - ref self", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName, // same as synthesizedComp
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName1))
				checkEnvVarWithValue(envVars, "service-host", svcName1)
			})

			It("w/o option - comp def name with regexp", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  "^" + synthesizedComp.CompDefName + "$", // compDef name with regexp
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName1))
				checkEnvVarWithValue(envVars, "service-host", svcName1)
			})

			It("w/ option - ref others", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  "abc" + synthesizedComp.CompDefName, // different with synthesizedComp
									Name:     "service",
									Optional: required(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(ContainSubstring("more than one referent component found"))
			})

			It("individual", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name: "credential-username",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "credential",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
									},
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				// the defined var will have empty values.
				Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName1, svcName1))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName2, svcName2))
				// the defined var will have empty values.
				checkEnvVarWithValue(envVars, "service-host", "")
				checkEnvVarWithValue(envVars, svcVarName1, svcName1)
				checkEnvVarWithValue(envVars, svcVarName2, svcName2)
				// the defined var will have empty values.
				checkEnvVarWithValueFrom(envVars, "credential-username", nil)
				checkEnvVarWithValueFrom(envVars, credentialVarName1, &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName1,
						},
						Key: constant.AccountNameForSecret,
					},
				})
				checkEnvVarWithValueFrom(envVars, credentialVarName2, &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName2,
						},
						Key: constant.AccountNameForSecret,
					},
				})
			})

			It("combined - reuse", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyCombined,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValue))
				// check that per-component vars not been created.
				Expect(templateVars).ShouldNot(HaveKey(svcVarName1))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName2))
				checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValue)
				// check that per-component vars not been created.
				checkEnvVarNotExist(envVars, svcVarName1)
				checkEnvVarNotExist(envVars, svcVarName2)
			})

			It("combined - new", func() {
				suffix := "suffix"
				combinedSvcVarName := fmt.Sprintf("%s_%s", "service-host", suffix)

				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyCombined,
										CombinedOption: &appsv1.MultipleClusterObjectCombinedOption{
											NewVarSuffix: &suffix,
										},
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				// the defined var will have empty values.
				Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
				Expect(templateVars).Should(HaveKeyWithValue(combinedSvcVarName, combinedSvcVarValue))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName1))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName2))
				// the defined var will have empty values.
				checkEnvVarWithValue(envVars, "service-host", "")
				checkEnvVarWithValue(envVars, combinedSvcVarName, combinedSvcVarValue)
				checkEnvVarNotExist(envVars, svcVarName1)
				checkEnvVarNotExist(envVars, svcVarName2)
			})

			It("combined - value from error", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "credential-username",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "credential",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyCombined,
									},
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(ContainSubstring("combined strategy doesn't support vars with valueFrom values"))
			})

			It("individual - optional partial objects", func() {
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1: synthesizedComp.CompDefName,
					compName2: synthesizedComp.CompDefName,
					compName3: synthesizedComp.CompDefName, // there is no service object for comp3.
				}
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: optional(), // optional
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName1, svcName1))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName2, svcName2))
				// the new var for comp3 will still be created, but its values will be empty.
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName3, ""))
				checkEnvVarWithValue(envVars, "service-host", "")
				checkEnvVarWithValue(envVars, svcVarName1, svcName1)
				checkEnvVarWithValue(envVars, svcVarName2, svcName2)
				// the new var for comp3 will still be created, but its values will be empty.
				checkEnvVarWithValue(envVars, svcVarName3, "")
			})

			It("individual - required partial objects", func() {
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1: synthesizedComp.CompDefName,
					compName2: synthesizedComp.CompDefName,
					compName3: synthesizedComp.CompDefName, // there is no service object for comp3.
				}
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: required(), // required
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(And(ContainSubstring("has no"), ContainSubstring("found when resolving vars")))

				// create service for comp3
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName3,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Port: int32(3306),
							},
						},
					},
				})
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(BeNil())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName1, svcName1))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName2, svcName2))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName3, svcName3))
				checkEnvVarWithValue(envVars, "service-host", "")
				checkEnvVarWithValue(envVars, svcVarName1, svcName1)
				checkEnvVarWithValue(envVars, svcVarName2, svcName2)
				checkEnvVarWithValue(envVars, svcVarName3, svcName3)
			})

			It("combined - optional partial objects", func() {
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1: synthesizedComp.CompDefName,
					compName2: synthesizedComp.CompDefName,
					compName3: synthesizedComp.CompDefName, // there is no service object for comp3.
				}
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: optional(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyCombined,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				// the combined value will have comp3 in it, but its value will be empty: "comp1:val1,comp2:val2,comp3:"
				Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValueWithComp3KeyOnly))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName1))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName2))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName3))
				// the combined value will have comp3 in it, but its value will be empty: "comp1:val1,comp2:val2,comp3:"
				checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValueWithComp3KeyOnly)
				checkEnvVarNotExist(envVars, svcVarName1)
				checkEnvVarNotExist(envVars, svcVarName2)
				checkEnvVarNotExist(envVars, svcVarName3)
			})

			It("combined - required partial objects", func() {
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1: synthesizedComp.CompDefName,
					compName2: synthesizedComp.CompDefName,
					compName3: synthesizedComp.CompDefName, // there is no service object for comp3.
				}
				vars := []appsv1.EnvVar{
					{
						Name: "service-host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "service",
									Optional: required(), // required
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyCombined,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(BeNil())
				Expect(err.Error()).Should(And(ContainSubstring("has no"), ContainSubstring("found when resolving vars")))

				// create service for comp3
				reader.objs = append(reader.objs, &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      svcName3,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Port: int32(3306),
							},
						},
					},
				})
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValueWithComp3))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName1))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName2))
				Expect(templateVars).ShouldNot(HaveKey(svcVarName3))
				checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValueWithComp3)
				checkEnvVarNotExist(envVars, svcVarName1)
				checkEnvVarNotExist(envVars, svcVarName2)
				checkEnvVarNotExist(envVars, svcVarName3)
			})
		})

		Context("vars reference and escaping", func() {
			var (
				reader *mockReader
			)

			BeforeEach(func() {
				reader = &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, "credential"),
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret:   []byte("username"),
								constant.AccountPasswdForSecret: []byte("password"),
							},
						},
					},
				}
			})

			It("reference", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "aa",
						Value: "~",
					},
					{
						Name:  "ab",
						Value: "$(aa)",
					},
					{
						Name:  "ac",
						Value: "abc$(aa)xyz",
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("ab", "~"))
				Expect(templateVars).Should(HaveKeyWithValue("ac", "abc~xyz"))
				checkEnvVarWithValue(envVars, "ab", "~")
				checkEnvVarWithValue(envVars, "ac", "abc~xyz")
			})

			It("reference not defined", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "ba",
						Value: "~",
					},
					{
						Name:  "bb",
						Value: "$(x)",
					},
					{
						Name:  "bc",
						Value: "abc$(x)xyz",
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("bb", "$(x)"))
				Expect(templateVars).Should(HaveKeyWithValue("bc", "abc$(x)xyz"))
				checkEnvVarWithValue(envVars, "bb", "$(x)")
				checkEnvVarWithValue(envVars, "bc", "abc$(x)xyz")
			})

			It("reference credential var", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "ca",
						Value: "~",
					},
					{
						Name:  "cb",
						Value: "$(credential-username)",
					},
					{
						Name: "credential-username",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: optional(),
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("cb", "$(credential-username)"))
				checkEnvVarWithValueFrom(envVars, "cb", &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: reader.objs[0].GetName(),
						},
						Key: "username",
					},
				})
			})

			It("escaping", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "da",
						Value: "~",
					},
					{
						Name:  "db",
						Value: "$$(da)",
					},
					{
						Name:  "dc",
						Value: "abc$$(da)xyz",
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("db", "$(da)"))
				Expect(templateVars).Should(HaveKeyWithValue("dc", "abc$(da)xyz"))
				checkEnvVarWithValue(envVars, "db", "$(da)")
				checkEnvVarWithValue(envVars, "dc", "abc$(da)xyz")
			})

			It("reference and escaping", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "ea",
						Value: "~",
					},
					{
						Name:  "eb",
						Value: "$(ea)$$(ea)$$(ea)$(ea)$(ea)$$(ea)",
					},
					{
						Name:  "ec",
						Value: "abc$(ea)xyz$$(ea)",
					},
					{
						Name:  "ed",
						Value: "$$(x)$(x)",
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("eb", "~$(ea)$(ea)~~$(ea)"))
				Expect(templateVars).Should(HaveKeyWithValue("ec", "abc~xyz$(ea)"))
				Expect(templateVars).Should(HaveKeyWithValue("ed", "$(x)$(x)"))
				checkEnvVarWithValue(envVars, "eb", "~$(ea)$(ea)~~$(ea)")
				checkEnvVarWithValue(envVars, "ec", "abc~xyz$(ea)")
				checkEnvVarWithValue(envVars, "ed", "$(x)$(x)")
			})

			It("all in one", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "fa",
						Value: "~",
					},
					{
						Name:  "fb",
						Value: "abc$(fa)$$(fa)$$(fa)$(credential-username)$(fa)$(x)$$(x)xyz",
					},
					{
						Name: "credential-username",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: optional(),
								},
								CredentialVars: appsv1.CredentialVars{
									Username: &appsv1.VarOptional,
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz"))
				checkEnvVarWithValue(envVars, "fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz")
			})
		})

		Context("vars expression", func() {
			It("simple format", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "port",
						Value:      "12345",
						Expression: expp("0{{ .port }}"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("port", "012345"))
				checkEnvVarWithValue(envVars, "port", "012345")
			})

			It("cluster domain", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "headless",
						Value:      "test-headless.default.svc",
						Expression: expp("{{ printf \"%s.%s\" .headless .ClusterDomain }}"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("headless", "test-headless.default.svc.cluster.local"))
				checkEnvVarWithValue(envVars, "headless", "test-headless.default.svc.cluster.local")
			})

			It("condition exp", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "port",
						Value:      "12345",
						Expression: expp("{{ if eq .port \"12345\" }}54321{{ else }}0{{ end }}"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("port", "54321"))
				checkEnvVarWithValue(envVars, "port", "54321")
			})

			It("exp only", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "port",
						Expression: expp("12345"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("port", "12345"))
				checkEnvVarWithValue(envVars, "port", "12345")
			})

			It("exp error", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "port",
						Expression: expp("{{ if eq .port 12345 }}54321{{ end }}"),
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(ContainSubstring("incompatible types for comparison"))
			})

			It("access another vars", func() {
				vars := []appsv1.EnvVar{
					{
						Name:  "host",
						Value: "localhost",
					},
					{
						Name:  "port",
						Value: "12345",
					},
					{
						Name:       "endpoint",
						Expression: expp("{{ .host }}:{{ .port }}"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("endpoint", "localhost:12345"))
				checkEnvVarWithValue(envVars, "endpoint", "localhost:12345")
			})

			It("access generated vars", func() {
				var (
					compName1 = synthesizedComp.Name + "-1"
					compName2 = synthesizedComp.Name + "-2"
					svcName1  = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName1, "")
					svcName2  = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName2, "")

					varName = func(compName, envName string) string {
						return fmt.Sprintf("%s_%s", envName, strings.ToUpper(strings.ReplaceAll(compName, "-", "_")))
					}
					svcVarName1 = varName(compName1, "host")
					svcVarName2 = varName(compName2, "host")
				)
				synthesizedComp.Comp2CompDefs = map[string]string{
					compName1: synthesizedComp.CompDefName,
					compName2: synthesizedComp.CompDefName,
				}
				vars := []appsv1.EnvVar{
					{
						Name: "host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									CompDef:  synthesizedComp.CompDefName,
									Name:     "",
									Optional: required(),
									MultipleClusterObjectOption: &appsv1.MultipleClusterObjectOption{
										Strategy: appsv1.MultipleClusterObjectStrategyIndividual,
									},
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarRequired,
								},
							},
						},
					},
					{
						Name:       "endpoints",
						Expression: expp(fmt.Sprintf("{{ .%s }},{{ .%s }}", svcVarName1, svcVarName2)),
					},
				}
				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      svcName1,
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Port: int32(12345),
									},
								},
							},
						},
						&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      svcName2,
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Port: int32(12345),
									},
								},
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				// the defined var will have empty values.
				Expect(templateVars).Should(HaveKeyWithValue("host", ""))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName1, svcName1))
				Expect(templateVars).Should(HaveKeyWithValue(svcVarName2, svcName2))
				Expect(templateVars).Should(HaveKeyWithValue("endpoints", fmt.Sprintf("%s,%s", svcName1, svcName2)))
				// the defined var will have empty values.
				checkEnvVarWithValue(envVars, "host", "")
				checkEnvVarWithValue(envVars, svcVarName1, svcName1)
				checkEnvVarWithValue(envVars, svcVarName2, svcName2)
				checkEnvVarWithValue(envVars, "endpoints", fmt.Sprintf("%s,%s", svcName1, svcName2))
			})

			It("exp for resolved but not-exist vars", func() {
				svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "")
				vars := []appsv1.EnvVar{
					{
						Name: "host",
						ValueFrom: &appsv1.VarSource{
							ServiceVarRef: &appsv1.ServiceVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "", // the default component service
									Optional: optional(),
								},
								ServiceVars: appsv1.ServiceVars{
									Host: &appsv1.VarOptional,
								},
							},
						},
						Expression: expp("{{ if index . \"host\" }}{{ .host }}{{ else }}localhost{{ end }}"),
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("host", "localhost"))
				checkEnvVarWithValue(envVars, "host", "localhost")

				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      svcName,
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Port: int32(12345),
									},
								},
							},
						},
					},
				}
				templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("host", svcName))
				checkEnvVarWithValue(envVars, "host", svcName)
			})

			It("exp for credential-vars", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "password",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: required(),
								},
								CredentialVars: appsv1.CredentialVars{
									Password: &appsv1.VarRequired,
								},
							},
						},
						Expression: expp("panic"), // the expression will not be evaluated
					},
				}
				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, "credential"),
							},
							Data: map[string][]byte{
								constant.AccountPasswdForSecret: []byte("password"),
							},
						},
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).ShouldNot(HaveKey("password"))
				checkEnvVarWithValueFrom(envVars, "password", &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: reader.objs[0].GetName(),
						},
						Key: constant.AccountPasswdForSecret,
					},
				})
			})

			It("depends on credential-vars", func() {
				vars := []appsv1.EnvVar{
					{
						Name: "raw",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{
									Name:     "credential",
									Optional: required(),
								},
								CredentialVars: appsv1.CredentialVars{
									Password: &appsv1.VarRequired,
								},
							},
						},
						Expression: expp("panic"),
					},
					{
						Name:       "password",
						Expression: expp("{{ .raw }}"), // depends on $raw which is a credential-var
					},
				}
				reader := &mockReader{
					cli: testCtx.Cli,
					objs: []client.Object{
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      constant.GenerateAccountSecretName(synthesizedComp.ClusterName, synthesizedComp.Name, "credential"),
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret:   []byte("username"),
								constant.AccountPasswdForSecret: []byte("password"),
							},
						},
					},
				}
				_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
				Expect(err).ShouldNot(Succeed())
				Expect(err.Error()).Should(And(ContainSubstring("map has no entry for key"), ContainSubstring("raw")))
			})

			It("depends on intermediate values", func() {
				vars := []appsv1.EnvVar{
					{
						Name:       "endpoint",
						Expression: expp("{{ .host }}:{{ .port }}"),
					},
					{
						Name:       "host",
						Value:      "localhost",
						Expression: expp("127.0.0.1"),
					},
					{
						Name:  "port",
						Value: "12345",
					},
				}
				templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
				Expect(err).Should(Succeed())
				Expect(templateVars).Should(HaveKeyWithValue("endpoint", "localhost:12345"))
				Expect(templateVars).Should(HaveKeyWithValue("host", "127.0.0.1"))
				checkEnvVarWithValue(envVars, "endpoint", "localhost:12345")
				checkEnvVarWithValue(envVars, "host", "127.0.0.1")
			})
		})
	})
})
