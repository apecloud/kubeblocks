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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: "test-cluster",
				ClusterUID:  string(uuid.NewUUID()),
				Name:        "comp",
				CompDefName: "compDef",
				Replicas:    1,
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
			checkTemplateVars(templateVars, builtinTemplateVars(synthesizedComp))

			By("check default env vars")
			targetEnvVars := builtinTemplateVars(synthesizedComp)
			targetEnvVars = append(targetEnvVars, buildDefaultEnvVars(synthesizedComp, false)...)
			checkEnvVars(envVars, targetEnvVars)
		})

		It("TLS env vars", func() {
			synthesizedComp.TLSConfig = &appsv1alpha1.TLSConfig{
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
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-cm-var",
					ValueFrom: &appsv1alpha1.VarSource{
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
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-cm-var",
					ValueFrom: &appsv1alpha1.VarSource{
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
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "cm-var",
					ValueFrom: &appsv1alpha1.VarSource{
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
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-secret-var",
					ValueFrom: &appsv1alpha1.VarSource{
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
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-secret-var",
					ValueFrom: &appsv1alpha1.VarSource{
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
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "secret-var",
					ValueFrom: &appsv1alpha1.VarSource{
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

		It("pod vars", func() {
			By("ok")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "pod-container-port",
					ValueFrom: &appsv1alpha1.VarSource{
						PodVarRef: &appsv1alpha1.PodVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Optional: required(),
							},
							PodVars: appsv1alpha1.PodVars{
								Container: &appsv1alpha1.ContainerVars{
									Name: "default",
									Port: &appsv1alpha1.NamedVar{
										Name:   "default",
										Option: &appsv1alpha1.VarRequired,
									},
								},
							},
						},
					},
				},
			}
			synthesizedComp.PodSpec.Containers = append(synthesizedComp.PodSpec.Containers, corev1.Container{
				Name: "default",
				Ports: []corev1.ContainerPort{
					{
						Name:          "default",
						ContainerPort: 3306,
					},
				},
			})
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("pod-container-port", "3306"))
			checkEnvVarWithValue(envVars, "pod-container-port", "3306")
		})

		It("service vars", func() {
			By("non-exist service with optional")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-service-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: optional(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-service-var"))
			checkEnvVarNotExist(envVars, "non-exist-service-var")

			By("non-exist service with required")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-service-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "service-port",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Port: &appsv1alpha1.NamedVar{
									Name:   "default",
									Option: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
				{
					Name: "service-port-wo-name",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "service-wo-port-name",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Port: &appsv1alpha1.NamedVar{
									Name:   "default",
									Option: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
			}
			svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")
			svcPort := 3306
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
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
			Expect(templateVars).Should(HaveKeyWithValue("service-port", strconv.Itoa(svcPort)))
			Expect(templateVars).Should(HaveKeyWithValue("service-port-wo-name", strconv.Itoa(svcPort+1)))
			checkEnvVarWithValue(envVars, "service-host", svcName)
			checkEnvVarWithValue(envVars, "service-port", strconv.Itoa(svcPort))
			checkEnvVarWithValue(envVars, "service-port-wo-name", strconv.Itoa(svcPort+1))

			By("pod service")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "pod-service-endpoint",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "pod-service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "pod-service-port",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "pod-service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Port: &appsv1alpha1.NamedVar{
									Name:   "default",
									Option: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
			}
			svcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "pod-service-0")
			svcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "pod-service-1")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
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
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "pod-service-endpoint", strings.Join([]string{svcName0, svcName1}, ","))
			checkEnvVarWithValue(envVars, "pod-service-port", strings.Join([]string{fmt.Sprintf("%s:300001", svcName0), fmt.Sprintf("%s:300002", svcName1)}, ","))

			By("load balancer")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "lb",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "lb",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								LoadBalancer: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			lbSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
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
					},
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "lb", "127.0.0.1")

			By("load balancer - pod service")
			lbSvcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-0")
			lbSvcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-1")
			lbSvcName2 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "lb-2")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
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
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			endpoints := []string{
				fmt.Sprintf("%s:127.0.0.1", lbSvcName0),
				fmt.Sprintf("%s:127.0.0.2", lbSvcName1),
				fmt.Sprintf("%s:127.0.0.4", lbSvcName2),
			}
			checkEnvVarWithValue(envVars, "lb", strings.Join(endpoints, ","))

			By("load balancer - pod service in provisioning")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
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
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("the required var is not found"))

			By("adaptive - has load balancer pod service")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "advertised",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "advertised",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host:         &appsv1alpha1.VarRequired, // both host and loadBalancer
								LoadBalancer: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			advertisedSvcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "advertised-0")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
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
					},
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			endpoints = []string{
				fmt.Sprintf("%s:127.0.0.1", advertisedSvcName),
			}
			checkEnvVarWithValue(envVars, "advertised", strings.Join(endpoints, ","))

			By("adaptive - has no load balancer service")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
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
					},
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "advertised", advertisedSvcName)

			By("adaptive - has load balancer service in provisioning")
			// change to non pod-service
			advertisedSvcName = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "advertised")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
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
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("the required var is not found"))
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
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
					},
				},
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "advertised", "127.0.0.1")
		})

		It("credential vars", func() {
			By("non-exist credential with optional")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-credential-var",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: optional(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-credential-var"))
			checkEnvVarNotExist(envVars, "non-exist-credential-var")

			By("non-exist credential with required")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-credential-var",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: required(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "credential-username",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "credential",
								Optional: required(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "credential-password",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "credential",
								Optional: required(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Password: &appsv1alpha1.VarRequired,
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
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
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

		It("serviceref vars", func() {
			By("non-exist serviceref with optional")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-serviceref-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: optional(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								Endpoint: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-serviceref-var"))
			checkEnvVarNotExist(envVars, "non-exist-serviceref-var")

			By("non-exist serviceref with required")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-serviceref-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: required(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								Endpoint: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "serviceref-endpoint",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "serviceref",
								Optional: required(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								Endpoint: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "serviceref-port",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "serviceref",
								Optional: required(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								Port: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "serviceref-username",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "serviceref",
								Optional: required(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								CredentialVars: appsv1alpha1.CredentialVars{
									Username: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
				{
					Name: "serviceref-password",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceRefVarRef: &appsv1alpha1.ServiceRefVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "serviceref",
								Optional: required(),
							},
							ServiceRefVars: appsv1alpha1.ServiceRefVars{
								CredentialVars: appsv1alpha1.CredentialVars{
									Password: &appsv1alpha1.VarRequired,
								},
							},
						},
					},
				},
			}
			synthesizedComp.ServiceReferences = map[string]*appsv1alpha1.ServiceDescriptor{
				"serviceref": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      "serviceref",
					},
					Spec: appsv1alpha1.ServiceDescriptorSpec{
						ServiceKind:    "",
						ServiceVersion: "",
						Endpoint: &appsv1alpha1.CredentialVar{
							Value: "endpoint",
						},
						Port: &appsv1alpha1.CredentialVar{
							Value: "port",
						},
						Auth: &appsv1alpha1.ConnectionCredentialAuth{
							Username: &appsv1alpha1.CredentialVar{
								Value: "username",
							},
							Password: &appsv1alpha1.CredentialVar{
								Value: "password",
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("serviceref-endpoint", "endpoint"))
			Expect(templateVars).Should(HaveKeyWithValue("serviceref-port", "port"))
			Expect(templateVars).ShouldNot(HaveKey("serviceref-username"))
			Expect(templateVars).ShouldNot(HaveKey("serviceref-password"))
			checkEnvVarWithValue(envVars, "serviceref-endpoint", "endpoint")
			checkEnvVarWithValue(envVars, "serviceref-port", "port")
			checkEnvVarWithValue(envVars, "serviceref-username", "username")
			checkEnvVarWithValue(envVars, "serviceref-password", "password")
		})

		It("component vars", func() {
			By("non-exist component with optional")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-component-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ComponentVarRef: &appsv1alpha1.ComponentVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: optional(),
							},
							ComponentVars: appsv1alpha1.ComponentVars{
								Replicas: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("non-exist-component-var"))
			checkEnvVarNotExist(envVars, "non-exist-component-var")

			By("non-exist component with required")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "non-exist-component-var",
					ValueFrom: &appsv1alpha1.VarSource{
						ComponentVarRef: &appsv1alpha1.ComponentVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "non-exist",
								Optional: required(),
							},
							ComponentVars: appsv1alpha1.ComponentVars{
								Replicas: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "component-replicas",
					ValueFrom: &appsv1alpha1.VarSource{
						ComponentVarRef: &appsv1alpha1.ComponentVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Optional: required(),
							},
							ComponentVars: appsv1alpha1.ComponentVars{
								Replicas: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "component-podNames",
					ValueFrom: &appsv1alpha1.VarSource{
						ComponentVarRef: &appsv1alpha1.ComponentVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Optional: required(),
							},
							ComponentVars: appsv1alpha1.ComponentVars{
								PodNames: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			reader := &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&appsv1alpha1.Component{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, synthesizedComp.Name),
						},
						Spec: appsv1alpha1.ComponentSpec{
							CompDef:  synthesizedComp.CompDefName,
							Replicas: 3,
						},
					},
				},
			}
			mockPodList := []string{
				constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 0),
				constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 1),
				constant.GeneratePodName(synthesizedComp.ClusterName, synthesizedComp.Name, 2),
			}
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "component-replicas", fmt.Sprintf("%d", 3))
			checkEnvVarWithValue(envVars, "component-podNames", strings.Join(mockPodList, ","))
		})

		It("resolve component", func() {
			By("component not found w/ optional")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  "non-exist",
								Name:     "service",
								Optional: optional(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("service-hst"))
			checkEnvVarNotExist(envVars, "service-host")

			By("component not found w/ required")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  "non-exist",
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())

			By("default component")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								// don't specify the comp def, it will match self by default
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			svcName := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service")
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
									Port: int32(3306),
								},
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
			checkEnvVarWithValue(envVars, "service-host", svcName)
		})

		It("multiple components", func() {
			var (
				compName1             = synthesizedComp.Name
				compName2             = synthesizedComp.Name + "-other"
				compName3             = synthesizedComp.Name + "-other-not-exist"
				svcName1              = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName1, "service")
				svcName2              = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName2, "service")
				svcName3              = constant.GenerateComponentServiceName(synthesizedComp.ClusterName, compName3, "service")
				credentialSecretName1 = constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName1, "credential")
				credentialSecretName2 = constant.GenerateAccountSecretName(synthesizedComp.ClusterName, compName2, "credential")

				compVarName = func(compName, envName string) string {
					return fmt.Sprintf("%s_%s", envName, strings.ToUpper(strings.ReplaceAll(compName, "-", "_")))
				}
				compSvcVarName1        = compVarName(compName1, "service-host")
				compSvcVarName2        = compVarName(compName2, "service-host")
				compSvcVarName3        = compVarName(compName3, "service-host")
				compCredentialVarName1 = compVarName(compName1, "credential-username")
				compCredentialVarName2 = compVarName(compName2, "credential-username")

				combinedSvcVarValue                 = fmt.Sprintf("%s:%s,%s:%s", compName1, svcName1, compName2, svcName2)
				combinedSvcVarValueWithComp3KeyOnly = fmt.Sprintf("%s:%s,%s:%s,%s:", compName1, svcName1, compName2, svcName2, compName3)
				combinedSvcVarValueWithComp3        = fmt.Sprintf("%s:%s,%s:%s,%s:%s", compName1, svcName1, compName2, svcName2, compName3, svcName3)

				newVarSuffix          = "suffix"
				newCombinedSvcVarName = fmt.Sprintf("%s_%s", "service-host", newVarSuffix)

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
								Name:      credentialSecretName1,
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret: []byte("username"),
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: testCtx.DefaultNamespace,
								Name:      credentialSecretName2,
							},
							Data: map[string][]byte{
								constant.AccountNameForSecret: []byte("username"),
							},
						},
					},
				}
			)
			synthesizedComp.Comp2CompDefs = map[string]string{
				compName1:       synthesizedComp.CompDefName,
				compName2:       synthesizedComp.CompDefName,
				"comp-other-01": "abc" + synthesizedComp.CompDefName,
				"comp-other-02": "abc" + synthesizedComp.CompDefName,
			}

			By("w/o option - ref self")
			vars := []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName, // same as synthesizedComp
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName1))
			checkEnvVarWithValue(envVars, "service-host", svcName1)

			By("w/ option - ref others")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  "abc" + synthesizedComp.CompDefName, // different with synthesizedComp
								Name:     "service",
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("more than one referent component found"))

			By("individual")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: required(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyIndividual,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
				{
					Name: "credential-username",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "credential",
								Optional: required(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyIndividual,
								},
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			// the defined var will have empty values.
			Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName1, svcName1))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName2, svcName2))
			// the defined var will have empty values.
			checkEnvVarWithValue(envVars, "service-host", "")
			checkEnvVarWithValue(envVars, compSvcVarName1, svcName1)
			checkEnvVarWithValue(envVars, compSvcVarName2, svcName2)
			// the defined var will have empty values.
			checkEnvVarWithValueFrom(envVars, "credential-username", nil)
			checkEnvVarWithValueFrom(envVars, compCredentialVarName1, &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: credentialSecretName1,
					},
					Key: constant.AccountNameForSecret,
				},
			})
			checkEnvVarWithValueFrom(envVars, compCredentialVarName2, &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: credentialSecretName2,
					},
					Key: constant.AccountNameForSecret,
				},
			})

			By("combined - reuse")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: required(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyCombined,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValue))
			// check that per-component vars not been created.
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName1))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName2))
			checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValue)
			// check that per-component vars not been created.
			checkEnvVarNotExist(envVars, compSvcVarName1)
			checkEnvVarNotExist(envVars, compSvcVarName2)

			By("combined - new")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: required(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyCombined,
									CombinedOption: &appsv1alpha1.MultipleClusterObjectCombinedOption{
										NewVarSuffix: &newVarSuffix,
									},
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			// the defined var will have empty values.
			Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
			Expect(templateVars).Should(HaveKeyWithValue(newCombinedSvcVarName, combinedSvcVarValue))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName1))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName2))
			// the defined var will have empty values.
			checkEnvVarWithValue(envVars, "service-host", "")
			checkEnvVarWithValue(envVars, newCombinedSvcVarName, combinedSvcVarValue)
			checkEnvVarNotExist(envVars, compSvcVarName1)
			checkEnvVarNotExist(envVars, compSvcVarName2)

			By("combined - value from error")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "credential-username",
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "credential",
								Optional: required(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyCombined,
								},
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("combined strategy doesn't support vars with valueFrom values"))

			By("individual - optional partial objects")
			synthesizedComp.Comp2CompDefs = map[string]string{
				compName1: synthesizedComp.CompDefName,
				compName2: synthesizedComp.CompDefName,
				compName3: synthesizedComp.CompDefName, // there is no service object for comp3.
			}
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: optional(), // optional
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyIndividual,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName1, svcName1))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName2, svcName2))
			// the new var for comp3 will still be created, but its values will be empty.
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName3, ""))
			checkEnvVarWithValue(envVars, "service-host", "")
			checkEnvVarWithValue(envVars, compSvcVarName1, svcName1)
			checkEnvVarWithValue(envVars, compSvcVarName2, svcName2)
			// the new var for comp3 will still be created, but its values will be empty.
			checkEnvVarWithValue(envVars, compSvcVarName3, "")

			By("individual - required partial objects")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: required(), // required
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyIndividual,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not found when resolving vars"))
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(BeNil())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", ""))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName1, svcName1))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName2, svcName2))
			Expect(templateVars).Should(HaveKeyWithValue(compSvcVarName3, svcName3))
			checkEnvVarWithValue(envVars, "service-host", "")
			checkEnvVarWithValue(envVars, compSvcVarName1, svcName1)
			checkEnvVarWithValue(envVars, compSvcVarName2, svcName2)
			checkEnvVarWithValue(envVars, compSvcVarName3, svcName3)
			// remove service for comp3
			reader.objs = reader.objs[:len(reader.objs)-1]

			By("combined - optional partial objects")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: optional(),
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyCombined,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			// the combined value will have comp3 in it, but its value will be empty: "comp1:val1,comp2:val2,comp3:"
			Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValueWithComp3KeyOnly))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName1))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName2))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName3))
			// the combined value will have comp3 in it, but its value will be empty: "comp1:val1,comp2:val2,comp3:"
			checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValueWithComp3KeyOnly)
			checkEnvVarNotExist(envVars, compSvcVarName1)
			checkEnvVarNotExist(envVars, compSvcVarName2)
			checkEnvVarNotExist(envVars, compSvcVarName3)

			By("combined - required partial objects")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
								Name:     "service",
								Optional: required(), // required
								MultipleClusterObjectOption: &appsv1alpha1.MultipleClusterObjectOption{
									Strategy: appsv1alpha1.MultipleClusterObjectStrategyCombined,
								},
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								Host: &appsv1alpha1.VarRequired,
							},
						},
					},
				},
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("not found when resolving vars"))
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", combinedSvcVarValueWithComp3))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName1))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName2))
			Expect(templateVars).ShouldNot(HaveKey(compSvcVarName3))
			checkEnvVarWithValue(envVars, "service-host", combinedSvcVarValueWithComp3)
			checkEnvVarNotExist(envVars, compSvcVarName1)
			checkEnvVarNotExist(envVars, compSvcVarName2)
			checkEnvVarNotExist(envVars, compSvcVarName3)
			// remove service for comp3
			reader.objs = reader.objs[:len(reader.objs)-1]
		})

		It("vars reference and escaping", func() {
			By("reference")
			vars := []appsv1alpha1.EnvVar{
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

			By("reference not defined")
			vars = []appsv1alpha1.EnvVar{
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("bb", "$(x)"))
			Expect(templateVars).Should(HaveKeyWithValue("bc", "abc$(x)xyz"))
			checkEnvVarWithValue(envVars, "bb", "$(x)")
			checkEnvVarWithValue(envVars, "bc", "abc$(x)xyz")

			By("reference credential var")
			vars = []appsv1alpha1.EnvVar{
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
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "credential",
								Optional: optional(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarOptional,
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
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

			By("escaping")
			vars = []appsv1alpha1.EnvVar{
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("db", "$(da)"))
			Expect(templateVars).Should(HaveKeyWithValue("dc", "abc$(da)xyz"))
			checkEnvVarWithValue(envVars, "db", "$(da)")
			checkEnvVarWithValue(envVars, "dc", "abc$(da)xyz")

			By("reference and escaping")
			vars = []appsv1alpha1.EnvVar{
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("eb", "~$(ea)$(ea)~~$(ea)"))
			Expect(templateVars).Should(HaveKeyWithValue("ec", "abc~xyz$(ea)"))
			Expect(templateVars).Should(HaveKeyWithValue("ed", "$(x)$(x)"))
			checkEnvVarWithValue(envVars, "eb", "~$(ea)$(ea)~~$(ea)")
			checkEnvVarWithValue(envVars, "ec", "abc~xyz$(ea)")
			checkEnvVarWithValue(envVars, "ed", "$(x)$(x)")

			By("all in one")
			vars = []appsv1alpha1.EnvVar{
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
					ValueFrom: &appsv1alpha1.VarSource{
						CredentialVarRef: &appsv1alpha1.CredentialVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     "credential",
								Optional: optional(),
							},
							CredentialVars: appsv1alpha1.CredentialVars{
								Username: &appsv1alpha1.VarOptional,
							},
						},
					},
				},
			}
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz"))
			checkEnvVarWithValue(envVars, "fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz")
		})
	})
})
