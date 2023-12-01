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

	checkTemplateVars := func(synthesizedComp *SynthesizedComponent, targetVars []corev1.EnvVar) {
		vars := make([]corev1.EnvVar, 0)
		for _, v := range targetVars {
			if a, ok := synthesizedComp.TemplateVars[v.Name]; ok {
				val := ""
				if a != nil {
					val = a.(string)
				}
				vars = append(vars, corev1.EnvVar{Name: v.Name, Value: val})
			}
		}
		Expect(vars).Should(BeEquivalentTo(targetVars))
	}

	// without the order check
	checkEnvVars := func(synthesizedComp *SynthesizedComponent, targetEnvVars []corev1.EnvVar) {
		targetEnvVarMapping := map[string]corev1.EnvVar{}
		for i, env := range targetEnvVars {
			targetEnvVarMapping[env.Name] = targetEnvVars[i]
		}

		for _, cc := range [][]corev1.Container{synthesizedComp.PodSpec.InitContainers, synthesizedComp.PodSpec.Containers} {
			for _, c := range cc {
				envVarMapping := map[string]corev1.EnvVar{}
				for i, env := range c.Env {
					if _, ok := targetEnvVarMapping[env.Name]; ok {
						envVarMapping[env.Name] = c.Env[i]
					}
				}
				Expect(envVarMapping).Should(BeEquivalentTo(targetEnvVarMapping))
			}
		}
	}

	checkEnvVarNotExist := func(synthesizedComp *SynthesizedComponent, envName string) {
		for _, cc := range [][]corev1.Container{synthesizedComp.PodSpec.InitContainers, synthesizedComp.PodSpec.Containers} {
			for _, c := range cc {
				envVarMapping := map[string]any{}
				for _, env := range c.Env {
					envVarMapping[env.Name] = true
				}
				Expect(envVarMapping).ShouldNot(HaveKey(envName))
			}
		}
	}

	checkEnvVarExist := func(synthesizedComp *SynthesizedComponent, envName, envValue string) {
		for _, cc := range [][]corev1.Container{synthesizedComp.PodSpec.InitContainers, synthesizedComp.PodSpec.Containers} {
			for _, c := range cc {
				envVarMapping := map[string]string{}
				for _, env := range c.Env {
					envVarMapping[env.Name] = env.Value
				}
				Expect(envVarMapping).Should(HaveKeyWithValue(envName, envValue))
			}
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, nil)).Should(Succeed())

			By("check default template vars")
			checkTemplateVars(synthesizedComp, builtinTemplateVars(synthesizedComp))

			By("check default env vars")
			targetEnvVars := builtinTemplateVars(synthesizedComp)
			targetEnvVars = append(targetEnvVars, buildDefaultEnv()...)
			checkEnvVars(synthesizedComp, targetEnvVars)
		})

		It("TLS env vars", func() {
			synthesizedComp.TLSConfig = &appsv1alpha1.TLSConfig{
				Enable: true,
			}
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, nil)).Should(Succeed())
			checkEnvVars(synthesizedComp, buildEnv4TLS(synthesizedComp))
		})

		It("user-defined env vars", func() {
			By("invalid")
			annotations := map[string]string{
				constant.ExtraEnvAnnotationKey: "invalid-json-format",
			}
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, annotations, nil)).ShouldNot(Succeed())

			By("ok")
			data, _ := json.Marshal(map[string]string{
				"user-defined-var": "user-defined-value",
			})
			annotations = map[string]string{
				constant.ExtraEnvAnnotationKey: string(data),
			}
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, annotations, nil)).Should(Succeed())
			checkEnvVars(synthesizedComp, []corev1.EnvVar{{Name: "user-defined-var", Value: "user-defined-value"}})
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("non-exist-cm-var"))
			checkEnvVarNotExist(synthesizedComp, "non-exist-cm-var")

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).ShouldNot(Succeed())

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("cm-var", "cm-var-value"))
			checkEnvVarExist(synthesizedComp, "cm-var", "cm-var-value")
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("non-exist-secret-var"))
			checkEnvVarNotExist(synthesizedComp, "non-exist-secret-var")

			By("non-exist configmap with required")
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).ShouldNot(Succeed())

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("secret-var", "secret-var-value"))
			checkEnvVarExist(synthesizedComp, "secret-var", "secret-var-value")
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("non-exist-service-var"))
			checkEnvVarNotExist(synthesizedComp, "non-exist-service-var")

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).ShouldNot(Succeed())

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("service-host", svcName))
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("service-port", strconv.Itoa(svcPort)))
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("service-port-wo-name", strconv.Itoa(svcPort+1)))
			checkEnvVarExist(synthesizedComp, "service-host", svcName)
			checkEnvVarExist(synthesizedComp, "service-port", strconv.Itoa(svcPort))
			checkEnvVarExist(synthesizedComp, "service-port-wo-name", strconv.Itoa(svcPort+1))
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("non-exist-credential-var"))
			checkEnvVarNotExist(synthesizedComp, "non-exist-credential-var")

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).ShouldNot(Succeed())

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, reader, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("credential-username"))
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("credential-password"))
			checkEnvVarExist(synthesizedComp, "credential-username", "username")
			checkEnvVarExist(synthesizedComp, "credential-password", "password")
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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("non-exist-serviceref-var"))
			checkEnvVarNotExist(synthesizedComp, "non-exist-serviceref-var")

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).ShouldNot(Succeed())

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
			Expect(ResolveEnvNTemplateVars(testCtx.Ctx, testCtx.Cli, synthesizedComp, nil, vars)).Should(Succeed())
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("serviceref-endpoint", "endpoint"))
			Expect(synthesizedComp.TemplateVars).Should(HaveKeyWithValue("serviceref-port", "port"))
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("serviceref-username"))
			Expect(synthesizedComp.TemplateVars).ShouldNot(HaveKey("serviceref-password"))
			checkEnvVarExist(synthesizedComp, "serviceref-endpoint", "endpoint")
			checkEnvVarExist(synthesizedComp, "serviceref-port", "port")
			checkEnvVarExist(synthesizedComp, "serviceref-username", "username")
			checkEnvVarExist(synthesizedComp, "serviceref-password", "password")
		})
	})
})
