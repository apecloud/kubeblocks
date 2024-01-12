/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"context"
	"encoding/json"
	"reflect"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type mockReader struct {
	cli  client.Reader
	objs []client.Object
}

func (r *mockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	for _, o := range r.objs {
		// ignore the GVK check
		if client.ObjectKeyFromObject(o) == key {
			reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(o).Elem())
			return nil
		}
	}
	return r.cli.Get(ctx, key, obj, opts...)
}

func (r *mockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return r.cli.List(ctx, list, opts...)
}

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

	checkEnvVarWithValueFrom := func(envVars []corev1.EnvVar, envName string, envValue corev1.EnvVarSource) {
		envVarMapping := map[string]corev1.EnvVarSource{}
		for _, env := range envVars {
			if env.ValueFrom != nil {
				envVarMapping[env.Name] = *env.ValueFrom
			}
		}
		Expect(envVarMapping).Should(HaveKeyWithValue(envName, envValue))
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, nil)
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
			_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, nil)
			Expect(err).Should(Succeed())
			checkEnvVars(envVars, buildEnv4TLS(synthesizedComp))
		})

		It("user-defined env vars", func() {
			By("invalid")
			annotations := map[string]string{
				constant.ExtraEnvAnnotationKey: "invalid-json-format",
			}
			_, _, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, annotations, nil)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			data, _ := json.Marshal(map[string]string{
				"user-defined-var": "user-defined-value",
			})
			annotations = map[string]string{
				constant.ExtraEnvAnnotationKey: string(data),
			}
			_, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, annotations, nil)
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKeyWithValue("secret-var", "secret-var-value"))
			checkEnvVarWithValueFrom(envVars, "secret-var", corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "secret",
					},
					Key: "secret-key",
				},
			})
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
			Expect(templateVars).Should(HaveKeyWithValue("service-port", strconv.Itoa(svcPort)))
			Expect(templateVars).Should(HaveKeyWithValue("service-port-wo-name", strconv.Itoa(svcPort+1)))
			checkEnvVarWithValue(envVars, "service-host", svcName)
			checkEnvVarWithValue(envVars, "service-port", strconv.Itoa(svcPort))
			checkEnvVarWithValue(envVars, "service-port-wo-name", strconv.Itoa(svcPort+1))

			By("service var ref with pod ordinal")
			svcNameRefPrefix := "service-node-port"
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-node-port",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								Name:     svcNameRefPrefix,
								Optional: required(),
							},
							ServiceVars: appsv1alpha1.ServiceVars{
								NodePort: &appsv1alpha1.NamedVar{
									Name:   "default",
									Option: &appsv1alpha1.VarRequired,
								},
							},
							GeneratePodOrdinalServiceVar: true,
						},
					},
				},
			}
			svcName0 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service-node-port-0")
			svcName1 := constant.GenerateComponentServiceName(synthesizedComp.ClusterName, synthesizedComp.Name, "service-node-port-1")
			reader = &mockReader{
				cli: testCtx.Cli,
				objs: []client.Object{
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: testCtx.DefaultNamespace,
							Name:      svcName0,
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
						},
						Spec: corev1.ServiceSpec{
							Type: corev1.ServiceTypeNodePort,
							Ports: []corev1.ServicePort{
								{
									Name:     "default",
									Port:     int32(svcPort + 1),
									NodePort: 300002,
								},
							},
						},
					},
				},
			}
			synthesizedComp.Replicas = 2
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("the name and compDef of ServiceVarRef is required"))
			vars[0].ValueFrom.ServiceVarRef.ClusterObjectReference.CompDef = synthesizedComp.CompDefName
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			checkEnvVarWithValue(envVars, "service-node-port_0", "300001")
			checkEnvVarWithValue(envVars, "service-node-port_1", "300002")
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).ShouldNot(HaveKey("credential-username"))
			Expect(templateVars).ShouldNot(HaveKey("credential-password"))
			checkEnvVarWithValueFrom(envVars, "credential-username", corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: reader.objs[0].GetName(),
					},
					Key: "username",
				},
			})
			checkEnvVarWithValueFrom(envVars, "credential-password", corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: reader.objs[0].GetName(),
					},
					Key: "password",
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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

		It("referent component", func() {
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
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
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
			Expect(err).ShouldNot(Succeed())

			By("more than one component found")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
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
			synthesizedComp.Comp2CompDefs = map[string]string{
				synthesizedComp.Name: synthesizedComp.CompDefName,
				"other-comp":         synthesizedComp.CompDefName,
			}
			_, _, err = ResolveTemplateNEnvVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)
			Expect(err).ShouldNot(Succeed())

			By("ok")
			vars = []appsv1alpha1.EnvVar{
				{
					Name: "service-host",
					ValueFrom: &appsv1alpha1.VarSource{
						ServiceVarRef: &appsv1alpha1.ServiceVarSelector{
							ClusterObjectReference: appsv1alpha1.ClusterObjectReference{
								CompDef:  synthesizedComp.CompDefName,
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("service-host", svcName))
			checkEnvVarWithValue(envVars, "service-host", svcName)
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
			templateVars, envVars, err := ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("cb", "$(credential-username)"))
			checkEnvVarWithValueFrom(envVars, "cb", corev1.EnvVarSource{
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, nil, synthesizedComp, nil, vars)
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
			templateVars, envVars, err = ResolveTemplateNEnvVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)
			Expect(err).Should(Succeed())
			Expect(templateVars).Should(HaveKeyWithValue("fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz"))
			checkEnvVarWithValue(envVars, "fb", "abc~$(fa)$(fa)$(credential-username)~$(x)$(x)xyz")
		})
	})
})
