/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ParamConfigRenderer Controller", func() {

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initPDCRTest := func() {
		By("Create a config template obj")
		configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
			Create(&testCtx).
			GetObject()

		By("Create a parameters definition obj")
		paramsDef := testparameters.NewParametersDefinitionFactory(paramsDefName).
			SetReloadAction(testparameters.WithNoneAction()).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(paramsDef), func(obj *parametersv1alpha1.ParametersDefinition) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		By("Create a component definition obj and mock to available")
		compDefObj := testapps.NewComponentDefinitionFactory(compDefName).
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

	}

	Context("pdcr", func() {
		It("normal test", func() {
			initPDCRTest()

			pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
				SetParametersDefs(paramsDefName).
				SetComponentDefinition(compDefName).
				SetTemplateName(configSpecName).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pdcr), func(g Gomega, pdcr *parametersv1alpha1.ParamConfigRenderer) {
				g.Expect(pdcr.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
			})).Should(Succeed())

		})

		It("invalid config template", func() {
			initPDCRTest()

			pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
				SetParametersDefs(paramsDefName).
				SetComponentDefinition(compDefName).
				SetTemplateName(configSpecName).
				SetConfigDescription("test", "not_exist_template", parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini}).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pdcr), func(g Gomega, pdcr *parametersv1alpha1.ParamConfigRenderer) {
				g.Expect(pdcr.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
			})).ShouldNot(Succeed())
		})

		It("invalid parametersdefinitions", func() {
			initPDCRTest()

			pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
				SetParametersDefs("not_exist_pd").
				SetComponentDefinition(compDefName).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pdcr), func(g Gomega, pdcr *parametersv1alpha1.ParamConfigRenderer) {
				g.Expect(pdcr.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
			})).ShouldNot(Succeed())
		})

		It("invalid cmpd", func() {
			initPDCRTest()

			pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
				SetParametersDefs(paramsDefName).
				SetComponentDefinition("not_exist_cmpd").
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pdcr), func(g Gomega, pdcr *parametersv1alpha1.ParamConfigRenderer) {
				g.Expect(pdcr.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
			})).ShouldNot(Succeed())
		})
	})
})
