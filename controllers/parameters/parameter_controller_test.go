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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Parameter Controller", func() {

	var compParamKey types.NamespacedName
	var comp *component.SynthesizedComponent

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	prepareTestEnv := func() {
		_, _, _, _, comp = mockReconcileResource()
		compParamKey = types.NamespacedName{
			Namespace: testCtx.DefaultNamespace,
			Name:      configcore.GenerateComponentConfigurationName(comp.ClusterName, comp.Name),
		}

		Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
			g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(1)))
		})).Should(Succeed())
	}

	Context("parameter update", func() {
		It("Should reconcile success", func() {
			prepareTestEnv()

			By("submit the parameter update request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddParameters("innodb-buffer-pool-size", "1024M").
				AddParameters("max_connections", "100").
				Create(&testCtx).
				GetObject()

			By("check component parameter status")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			}), time.Second*10).Should(Succeed())

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

		It("parameters validate fails", func() {
			prepareTestEnv()

			By("submit the parameter update request with invalid max_connection")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddParameters("max_connections", "-100").
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
			})).Should(Succeed())
		})
	})

	Context("custom template update", func() {
		It("update user template", func() {
			prepareTestEnv()

			By("create custom template object")
			configmap := testparameters.NewComponentTemplateFactory(configSpecName, testCtx.DefaultNamespace).
				WithRandomName().
				Create(&testCtx).
				GetObject()

			By("submit the custom template request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddCustomTemplate(configSpecName, configmap.Name, configmap.Namespace).
				Create(&testCtx).
				GetObject()

			By("check component parameter status")
			Eventually(testapps.CheckObj(&testCtx, compParamKey, func(g Gomega, compParameter *parametersv1alpha1.ComponentParameter) {
				g.Expect(compParameter.Status.ObservedGeneration).Should(BeEquivalentTo(int64(2)))
				g.Expect(compParameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			}), time.Second*10).Should(Succeed())

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		})

		It("custom template failed", func() {
			prepareTestEnv()

			By("submit the custom template request")
			key := testapps.GetRandomizedKey(comp.Namespace, comp.FullCompName)
			parameterObj := testparameters.NewParameterFactory(key.Name, key.Namespace, comp.ClusterName, comp.Name).
				AddCustomTemplate(configSpecName, "not-exist-tpl", testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()

			By("check parameter status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameterObj), func(g Gomega, parameter *parametersv1alpha1.Parameter) {
				g.Expect(parameter.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.CMergeFailedPhase))
			})).Should(Succeed())
		})
	})
})
