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

package plan

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("generate service descriptor", func() {

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// resources should be released in following order
		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterVersionSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterDefinitionSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ConfigMapSignature, true, inNS, ml)
	}

	var (
		mockClient          *testutil.K8sClientMockHelper
		clusterDef          *appsv1alpha1.ClusterDefinition
		clusterVersion      *appsv1alpha1.ClusterVersion
		cluster             *appsv1alpha1.Cluster
		beReferencedCluster *appsv1alpha1.Cluster
	)

	var (
		namespace                        = "default"
		clusterName                      = "mycluster"
		beReferencedClusterName          = "mycluster-be-referenced"
		clusterDefName                   = "test-clusterdef"
		clusterVersionName               = "test-clusterversion"
		nginxCompName                    = "nginx"
		nginxCompDefName                 = "nginx"
		mysqlCompName                    = "mysql"
		mysqlCompDefName                 = "mysql"
		externalServiceDescriptorName    = "mock-external-service-descriptor-name"
		externalServiceDescriptorKind    = "redis"
		externalServiceDescriptorVersion = "7.0.1"
		internalClusterServiceRefKind    = "mysql"
		internalClusterServiceRefVersion = "8.0.2"
		secretName                       = beReferencedClusterName + "-conn-credential"
		configMapRefName                 = beReferencedClusterName + "-configmap-ref"
		redisServiceRefDeclarationName   = "redis"
		mysqlServiceRefDeclarationName   = "mysql"

		serviceRefEndpointValue = "my-mysql-0.default.svc.cluster.local"
		serviceRefPortValue     = "3306"
		serviceRefUsernameValue = "mock-username"
		serviceRefPasswordValue = "mock-password"
	)

	BeforeEach(func() {
		cleanEnv()
		mockClient = testutil.NewK8sMockClient()
		serviceRefDeclarations := []appsv1alpha1.ServiceRefDeclaration{
			{
				Name: redisServiceRefDeclarationName,
				ServiceRefDeclarationSpecs: []appsv1alpha1.ServiceRefDeclarationSpec{
					{
						ServiceKind:    externalServiceDescriptorKind,
						ServiceVersion: externalServiceDescriptorVersion,
					},
				},
			},
			{
				Name: mysqlServiceRefDeclarationName,
				ServiceRefDeclarationSpecs: []appsv1alpha1.ServiceRefDeclarationSpec{
					{
						ServiceKind:    internalClusterServiceRefKind,
						ServiceVersion: internalClusterServiceRefVersion,
					},
				},
			},
		}
		clusterDef = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatelessNginxComponent, nginxCompDefName).
			AddServiceRefDeclarations(serviceRefDeclarations).
			Create(&testCtx).GetObject()
		clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(nginxCompDefName).
			AddInitContainerShort("nginx-init", testapps.NginxImage).
			AddContainerShort("nginx", testapps.NginxImage).
			Create(&testCtx).GetObject()
		beReferencedCluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, beReferencedClusterName,
			clusterDef.Name, clusterVersion.Name).
			AddComponent(mysqlCompName, mysqlCompDefName).
			Create(&testCtx).GetObject()

		serviceRefs := []appsv1alpha1.ServiceRef{
			{
				Name:              redisServiceRefDeclarationName,
				ServiceDescriptor: externalServiceDescriptorName,
			},
			{
				Name:    mysqlServiceRefDeclarationName,
				Cluster: beReferencedCluster.Name,
			},
		}
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDef.Name, clusterVersion.Name).
			AddComponent(nginxCompName, nginxCompDefName).
			SetServiceRefs(serviceRefs).
			Create(&testCtx).GetObject()
	})

	AfterEach(func() {
		mockClient.Finish()
		cleanEnv()
	})

	// for test GetContainerWithVolumeMount
	Context("config template utils test", func() {
		It("service reference config template render test", func() {
			clusterKey := client.ObjectKeyFromObject(cluster)
			req := ctrl.Request{
				NamespacedName: clusterKey,
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
			}
			By("Create a serviceReferencesMap with SecretKeyRef and ConfigMapKeyRef for building a SynthesizedComponent Component")
			serviceReferencesMap := map[string]*appsv1alpha1.ServiceDescriptor{
				redisServiceRefDeclarationName: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      externalServiceDescriptorName,
						Namespace: namespace,
					},
					Spec: appsv1alpha1.ServiceDescriptorSpec{
						ServiceKind:    externalServiceDescriptorKind,
						ServiceVersion: externalServiceDescriptorVersion,
						Endpoint: &appsv1alpha1.CredentialVar{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: constant.ServiceDescriptorEndpointKey,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
								},
							},
						},
						Port: &appsv1alpha1.CredentialVar{
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: constant.ServiceDescriptorPortKey,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
								},
							},
						},
						Auth: &appsv1alpha1.ConnectionCredentialAuth{
							Username: &appsv1alpha1.CredentialVar{
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Key: constant.ServiceDescriptorUsernameKey,
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
									},
								},
							},
							Password: &appsv1alpha1.CredentialVar{
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Key: constant.ServiceDescriptorPasswordKey,
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
									},
								},
							},
						},
					},
				},
				mysqlServiceRefDeclarationName: {
					ObjectMeta: metav1.ObjectMeta{
						Name:      externalServiceDescriptorName,
						Namespace: namespace,
					},
					Spec: appsv1alpha1.ServiceDescriptorSpec{
						ServiceKind:    externalServiceDescriptorKind,
						ServiceVersion: externalServiceDescriptorVersion,
						Endpoint: &appsv1alpha1.CredentialVar{
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									Key: constant.ServiceDescriptorEndpointKey,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapRefName,
									},
								},
							},
						},
						Port: &appsv1alpha1.CredentialVar{
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									Key: constant.ServiceDescriptorPortKey,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapRefName,
									},
								},
							},
						},
						Auth: &appsv1alpha1.ConnectionCredentialAuth{
							Username: &appsv1alpha1.CredentialVar{
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										Key: constant.ServiceDescriptorUsernameKey,
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapRefName,
										},
									},
								},
							},
							Password: &appsv1alpha1.CredentialVar{
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										Key: constant.ServiceDescriptorPasswordKey,
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMapRefName,
										},
									},
								},
							},
						},
					},
				},
			}
			component, err := component.BuildComponent(
				reqCtx,
				nil,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				serviceReferencesMap,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.ServiceReferences).ShouldNot(BeNil())

			By("create a secret and a configmap for service reference")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					constant.ServiceDescriptorPasswordKey: []byte(serviceRefPasswordValue),
					constant.ServiceDescriptorUsernameKey: []byte(serviceRefUsernameValue),
					constant.ServiceDescriptorEndpointKey: []byte(serviceRefEndpointValue),
					constant.ServiceDescriptorPortKey:     []byte(serviceRefPortValue),
				},
			}
			Expect(testCtx.CheckedCreateObj(ctx, secret)).Should(Succeed())
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: secret.Name,
				Namespace: secret.Namespace}, secret)).Should(Succeed())

			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapRefName,
					Namespace: namespace,
				},
				Data: map[string]string{
					constant.ServiceDescriptorPasswordKey: serviceRefPasswordValue,
					constant.ServiceDescriptorUsernameKey: serviceRefUsernameValue,
					constant.ServiceDescriptorEndpointKey: serviceRefEndpointValue,
					constant.ServiceDescriptorPortKey:     serviceRefPortValue,
				},
			}
			Expect(testCtx.CheckedCreateObj(ctx, configMap)).Should(Succeed())
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: configMap.Name,
				Namespace: configMap.Namespace}, configMap)).Should(Succeed())

			var v Visitor = &ComponentVisitor{component: component}
			err = v.Visit(resolveServiceReferences(k8sClient, ctx, namespace))
			Expect(err).Should(Succeed())
			Expect(component.ServiceReferences).ShouldNot(BeNil())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Endpoint.Value).Should(Equal(serviceRefEndpointValue))
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Endpoint.ValueFrom).Should(BeNil())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Port.Value).Should(Equal(serviceRefPortValue))
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Port.ValueFrom).Should(BeNil())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Auth.Username.Value).Should(BeEmpty())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Auth.Username.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Auth.Password.Value).Should(BeEmpty())
			Expect(component.ServiceReferences[redisServiceRefDeclarationName].Spec.Auth.Password.ValueFrom.SecretKeyRef).ShouldNot(BeNil())

			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint.Value).Should(Equal(serviceRefEndpointValue))
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint.ValueFrom).Should(BeNil())
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Port.Value).Should(Equal(serviceRefPortValue))
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Port.ValueFrom).Should(BeNil())
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Username.Value).Should(BeEmpty())
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Username.ValueFrom.ConfigMapKeyRef).ShouldNot(BeNil())
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Password.Value).Should(BeEmpty())
			Expect(component.ServiceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Password.ValueFrom.ConfigMapKeyRef).ShouldNot(BeNil())
		})
	})
})
