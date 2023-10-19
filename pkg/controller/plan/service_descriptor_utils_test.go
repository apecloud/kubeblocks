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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
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
		clusterName                      = "cluster"
		beReferencedClusterName          = "cluster-be-referenced"
		clusterDefName                   = "test-cd"
		clusterVersionName               = "test-cv"
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
		redisServiceRefDeclarationName   = "redis"
		mysqlServiceRefDeclarationName   = "mysql"
	)

	BeforeEach(func() {
		cleanEnv()
		mockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		mockClient.Finish()
		cleanEnv()
	})

	// for test GetContainerWithVolumeMount
	Context("generate service descriptor test", func() {
		BeforeEach(func() {
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
		})

		It("serviceRefDeclaration serviceVersion regex validation test", func() {
			type versionCmp struct {
				serviceRefDeclRegex      string
				serviceDescriptorVersion string
			}
			tests := []struct {
				name   string
				fields versionCmp
				want   bool
			}{{
				name: "version string test true",
				fields: versionCmp{
					serviceRefDeclRegex:      "8.0.8",
					serviceDescriptorVersion: "8.0.8",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: versionCmp{
					serviceRefDeclRegex:      "8.0.8",
					serviceDescriptorVersion: "8.0.7",
				},
				want: false,
			}, {
				name: "version string test false",
				fields: versionCmp{
					serviceRefDeclRegex:      "^8.0.8$",
					serviceDescriptorVersion: "v8.0.8",
				},
				want: false,
			}, {
				name: "version string test true",
				fields: versionCmp{
					serviceRefDeclRegex:      "8.0.\\d{1,2}$",
					serviceDescriptorVersion: "8.0.6",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: versionCmp{
					serviceRefDeclRegex:      "8.0.\\d{1,2}$",
					serviceDescriptorVersion: "8.0.8.8.8",
				},
				want: false,
			}, {
				name: "version string test true",
				fields: versionCmp{
					serviceRefDeclRegex:      "^[v\\-]*?(\\d{1,2}\\.){0,3}\\d{1,2}$",
					serviceDescriptorVersion: "v-8.0.8.0",
				},
				want: true,
			}, {
				name: "version string test false",
				fields: versionCmp{
					serviceRefDeclRegex:      "^[v\\-]*?(\\d{1,2}\\.){0,3}\\d{1,2}$",
					serviceDescriptorVersion: "mysql-8.0.8",
				},
				want: false,
			}}
			for _, tt := range tests {
				match := verifyServiceVersion(tt.fields.serviceDescriptorVersion, tt.fields.serviceRefDeclRegex)
				Expect(match).Should(Equal(tt.want))
			}
		})

		It("generate service descriptor test", func() {
			By("Create cluster and beReferencedCluster object")
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

			clusterKey := client.ObjectKeyFromObject(cluster)
			req := ctrl.Request{
				NamespacedName: clusterKey,
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
			}
			By("GenServiceReferences failed because external service descriptor not found")
			serviceReferences, err := GenServiceReferences(reqCtx, testCtx.Cli, cluster, &clusterDef.Spec.ComponentDefs[0], &cluster.Spec.ComponentSpecs[0])
			Expect(err).ShouldNot(Succeed())
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			Expect(serviceReferences).Should(BeNil())

			By("create external service descriptor")
			endpoint := appsv1alpha1.CredentialVar{
				Value: "mock-endpoint",
			}
			port := appsv1alpha1.CredentialVar{
				Value: "mock-port",
			}
			auth := appsv1alpha1.ConnectionCredentialAuth{
				Username: &appsv1alpha1.CredentialVar{
					Value: "mock-username",
				},
				Password: &appsv1alpha1.CredentialVar{
					Value: "mock-password",
				},
			}
			externalServiceDescriptor := testapps.NewServiceDescriptorFactory(testCtx.DefaultNamespace, externalServiceDescriptorName).
				SetEndpoint(endpoint).
				SetPort(port).
				SetAuth(auth).
				Create(&testCtx).GetObject()

			By("GenServiceReferences failed because external service descriptor status is not available")
			serviceReferences, err = GenServiceReferences(reqCtx, testCtx.Cli, cluster, &clusterDef.Spec.ComponentDefs[0], &cluster.Spec.ComponentSpecs[0])
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status is not available"))
			Expect(serviceReferences).Should(BeNil())

			By("update external service descriptor status to available")
			Expect(testapps.ChangeObjStatus(&testCtx, externalServiceDescriptor, func() {
				externalServiceDescriptor.Status.Phase = appsv1alpha1.AvailablePhase
			})).Should(Succeed())

			By("GenServiceReferences failed because external service descriptor kind and version not match")
			serviceReferences, err = GenServiceReferences(reqCtx, testCtx.Cli, cluster, &clusterDef.Spec.ComponentDefs[0], &cluster.Spec.ComponentSpecs[0])
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("kind or version is not match with"))
			Expect(serviceReferences).Should(BeNil())

			By("update external service descriptor kind and version")
			Expect(testapps.ChangeObj(&testCtx, externalServiceDescriptor, func(externalServiceDescriptor *appsv1alpha1.ServiceDescriptor) {
				externalServiceDescriptor.Spec.ServiceKind = externalServiceDescriptorKind
				externalServiceDescriptor.Spec.ServiceVersion = externalServiceDescriptorVersion
			})).Should(Succeed())

			By("GenServiceReferences succeed because external service descriptor found and internal cluster reference found")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					constant.ServiceDescriptorPasswordKey: []byte("NHpycWZsMnI="),
					constant.ServiceDescriptorUsernameKey: []byte("cm9vdA=="),
					constant.ServiceDescriptorEndpointKey: []byte("my-mysql-0.default.svc.cluster.local"),
					constant.ServiceDescriptorPortKey:     []byte("3306"),
				},
			}
			Expect(testCtx.CheckedCreateObj(ctx, secret)).Should(Succeed())
			Expect(k8sClient.Get(context.Background(), client.ObjectKey{Name: secret.Name,
				Namespace: secret.Namespace}, secret)).Should(Succeed())
			serviceReferences, err = GenServiceReferences(reqCtx, testCtx.Cli, cluster, &clusterDef.Spec.ComponentDefs[0], &cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(serviceReferences).ShouldNot(BeNil())
			Expect(len(serviceReferences)).Should(Equal(2))
			Expect(serviceReferences[redisServiceRefDeclarationName]).ShouldNot(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Endpoint).ShouldNot(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Endpoint.Value).ShouldNot(BeEmpty())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Endpoint.ValueFrom).Should(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Port).ShouldNot(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Port.Value).ShouldNot(BeEmpty())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Port.ValueFrom).Should(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Auth).ShouldNot(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Auth.Username.Value).ShouldNot(BeEmpty())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Auth.Username.ValueFrom).Should(BeNil())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Auth.Password.Value).ShouldNot(BeEmpty())
			Expect(serviceReferences[redisServiceRefDeclarationName].Spec.Auth.Password.ValueFrom).Should(BeNil())

			Expect(serviceReferences[mysqlServiceRefDeclarationName]).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint.Value).Should(BeEmpty())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint.ValueFrom).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Endpoint.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Port).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Port.Value).Should(BeEmpty())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Port.ValueFrom).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Port.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Username.Value).Should(BeEmpty())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Username.ValueFrom).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Username.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Password.Value).Should(BeEmpty())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Password.ValueFrom).ShouldNot(BeNil())
			Expect(serviceReferences[mysqlServiceRefDeclarationName].Spec.Auth.Password.ValueFrom.SecretKeyRef).ShouldNot(BeNil())
		})
	})
})
