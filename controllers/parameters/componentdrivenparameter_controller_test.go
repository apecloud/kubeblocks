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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ComponentParameterGenerator Controller", func() {

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initTestResource := func() *appsv1.Component {
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
			WithRandomName().
			SetDefaultSpec().
			AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName).
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDefObj), func(obj *appsv1.ComponentDefinition) {
			obj.Status.Phase = appsv1.AvailablePhase
		})()).Should(Succeed())

		pdcr := testparameters.NewParametersDrivenConfigFactory(pdcrName).
			SetParametersDefs(paramsDef.Name).
			SetComponentDefinition(compDefObj.GetName()).
			SetTemplateName(configSpecName).
			HScaleEnabled().
			VScaleEnabled().
			TLSEnabled().
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pdcr), func(obj *parametersv1alpha1.ParameterDrivenConfigRender) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		By("Create a component obj")
		fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
		compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetUID(types.UID(fmt.Sprintf("test-uid"))).
			SetReplicas(1).
			SetResources(corev1.ResourceRequirements{Limits: corev1.ResourceList{"memory": resource.MustParse("2Gi")}}).
			Create(&testCtx).
			GetObject()

		return compObj
	}

	Context("Generate ComponentParameter", func() {
		It("Should reconcile success", func() {
			component := initTestResource()
			parameterKey := types.NamespacedName{
				Namespace: component.Namespace,
				Name:      configcore.GenerateComponentParameterName(clusterName, defaultCompName),
			}

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				item := intctrlutil.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.Payload.Data).Should(HaveKey(constant.ReplicasPayload))
				g.Expect(item.Payload.Data).Should(HaveKey(constant.ComponentResourcePayload))
			})).Should(Succeed())
		})
	})
})
