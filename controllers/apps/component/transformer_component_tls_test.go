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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("TLS self-signed cert function", func() {
	const (
		compDefName       = "test-compdef"
		clusterNamePrefix = "test-cluster"
		defaultCompName   = "mysql"
		caFile            = "ca.pem"
		certFile          = "cert.pem"
		keyFile           = "key.pem"
	)

	var (
		compDefObj *appsv1.ComponentDefinition
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("tls is enabled/disabled", func() {
		BeforeEach(func() {
			By("Create a componentDefinition obj")
			compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
				WithRandomName().
				AddAnnotations(constant.SkipImmutableCheckAnnotationKey, "true").
				SetDefaultSpec().
				Create(&testCtx).
				GetObject()
		})

		Context("when issuer is UserProvided", func() {
			var (
				synthesizedComp *component.SynthesizedComponent
				secretObj       *corev1.Secret
			)

			BeforeEach(func() {
				// prepare self provided tls certs secret
				var err error
				compDef := &appsv1.ComponentDefinition{
					Spec: appsv1.ComponentDefinitionSpec{
						TLS: &appsv1.TLS{
							CAFile:   ptr.To(caFile),
							CertFile: ptr.To(certFile),
							KeyFile:  ptr.To(keyFile),
						},
					},
				}
				synthesizedComp = &component.SynthesizedComponent{
					Namespace:    testCtx.DefaultNamespace,
					ClusterName:  "test",
					Name:         "self-provided",
					FullCompName: "test-self-provided",
					CompDefName:  compDefObj.Name,
				}
				secretObj, err = plan.ComposeTLSSecret(compDef, *synthesizedComp, nil)
				Expect(err).Should(BeNil())
				Expect(k8sClient.Create(testCtx.Ctx, secretObj)).Should(Succeed())
			})

			AfterEach(func() {
				// delete self provided tls secret
				Expect(k8sClient.Delete(testCtx.Ctx, secretObj)).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(secretObj), secretObj, false)).Should(Succeed())
			})

			It("should create the component when secret referenced exist", func() {
				issuer := &appsv1.Issuer{
					Name: appsv1.IssuerUserProvided,
					SecretRef: &appsv1.TLSSecretReference{
						SecretReference: corev1.SecretReference{
							Namespace: testCtx.DefaultNamespace,
							Name:      secretObj.Name,
						},
						CA:   caFile,
						Cert: certFile,
						Key:  keyFile,
					},
				}
				By("create component obj")
				compObj := testapps.NewComponentFactory(synthesizedComp.Namespace, synthesizedComp.FullCompName, synthesizedComp.CompDefName).
					WithRandomName().
					SetReplicas(3).
					SetTLSConfig(true, issuer).
					Create(&testCtx).
					GetObject()
				Eventually(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(compObj), compObj)).Should(Succeed())
			})
		})
	})
})
