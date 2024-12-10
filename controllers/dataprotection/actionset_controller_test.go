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

package dataprotection

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("ActionSet Controller test", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ActionSetSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("create a actionSet", func() {
		It("should be available", func() {
			as := testdp.NewFakeActionSet(&testCtx)
			Expect(as).ShouldNot(BeNil())
		})
	})

	Context("validate a actionSet", func() {
		It("validate withParameters", func() {
			as := testdp.NewFakeActionSet(&testCtx)
			Expect(as).ShouldNot(BeNil())
			By("set invalid withParameters and schema")
			Expect(testapps.ChangeObj(&testCtx, as, func(action *dpv1alpha1.ActionSet) {
				as.Spec.ParametersSchema = &dpv1alpha1.ActionSetParametersSchema{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							testdp.ParameterString: {
								Type: testdp.ParameterStringType,
							},
							testdp.ParameterArray: {
								Type: testdp.ParameterArrayType,
								Items: &apiextensionsv1.JSONSchemaPropsOrArray{
									Schema: &apiextensionsv1.JSONSchemaProps{
										Type: testdp.ParameterStringType,
									},
								},
							},
						},
					},
				}
				as.Spec.Backup.WithParameters = []string{testdp.InvalidParameter}
			})).Should(Succeed())
			By("should be unavailable with invalid withParameters")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(as),
				func(g Gomega, as *dpv1alpha1.ActionSet) {
					g.Expect(as.Status.ObservedGeneration).Should(Equal(as.Generation))
					g.Expect(as.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.UnavailablePhase))
					g.Expect(as.Status.Message).ShouldNot(BeEmpty())
				})).Should(Succeed())
			By("set valid parameters")
			Expect(testapps.ChangeObj(&testCtx, as, func(action *dpv1alpha1.ActionSet) {
				as.Spec.Backup.WithParameters = []string{testdp.ParameterString, testdp.ParameterArray}
			})).Should(Succeed())
			By("should be available")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(as),
				func(g Gomega, as *dpv1alpha1.ActionSet) {
					g.Expect(as.Status.ObservedGeneration).Should(Equal(as.Generation))
					g.Expect(as.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.AvailablePhase))
					g.Expect(as.Status.Message).Should(BeEmpty())
				})).Should(Succeed())
		})
	})
})
