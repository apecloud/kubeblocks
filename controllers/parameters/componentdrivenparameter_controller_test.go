/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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

		pdcr := testparameters.NewParamConfigRendererFactory(pdcrName).
			SetParametersDefs(paramsDef.Name).
			SetComponentDefinition(compDefObj.GetName()).
			SetTemplateName(configSpecName).
			HScaleEnabled().
			VScaleEnabled().
			TLSEnabled().
			Create(&testCtx).
			GetObject()
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(pdcr), func(obj *parametersv1alpha1.ParamConfigRenderer) {
			obj.Status.Phase = parametersv1alpha1.PDAvailablePhase
		})()).Should(Succeed())

		By("Create init parameters")
		key := testapps.GetRandomizedKey(testCtx.DefaultNamespace, defaultCompName)
		testparameters.NewParameterFactory(key.Name, key.Namespace, clusterName, defaultCompName).
			AddParameters("innodb-buffer-pool-size", "1024M").
			AddParameters("max_connections", "100").
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			AddLabels(constant.ParametersInitLabelKey, "true").
			Create(&testCtx)

		By("Create a custom template cm")
		tplKey := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "custom-tpl")
		tpl := testparameters.NewComponentTemplateFactory(tplKey.Name, testCtx.DefaultNamespace).
			AddConfigFile(testparameters.MysqlConfigFile, "abcde=1234").
			Create(&testCtx).
			GetObject()

		customTemplate := parametersv1alpha1.ConfigTemplateExtension{
			TemplateRef: tpl.Name,
			Namespace:   tpl.Namespace,
			Policy:      parametersv1alpha1.ReplacePolicy,
		}
		annotationValue, _ := json.Marshal(map[string]parametersv1alpha1.ConfigTemplateExtension{
			configSpecName: customTemplate,
		})

		By("Create a component obj")
		fullCompName := constant.GenerateClusterComponentName(clusterName, defaultCompName)
		compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, fullCompName, compDefObj.Name).
			AddLabels(constant.AppInstanceLabelKey, clusterName).
			SetUID(types.UID("test-uid")).
			SetReplicas(1).
			SetResources(corev1.ResourceRequirements{Limits: corev1.ResourceList{"memory": resource.MustParse("2Gi")}}).
			SetAnnotations(map[string]string{constant.CustomParameterTemplateAnnotationKey: string(annotationValue)}).
			Create(&testCtx).
			GetObject()
		return compObj
	}

	Context("Generate ComponentParameter", func() {
		It("Should reconcile success", func() {
			component := initTestResource()
			parameterKey := types.NamespacedName{
				Namespace: component.Namespace,
				Name:      configcore.GenerateComponentConfigurationName(clusterName, defaultCompName),
			}

			Eventually(testapps.CheckObj(&testCtx, parameterKey, func(g Gomega, parameter *parametersv1alpha1.ComponentParameter) {
				item := intctrlutil.GetConfigTemplateItem(&parameter.Spec, configSpecName)
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.Payload).Should(HaveKey(constant.ReplicasPayload))
				g.Expect(item.Payload).Should(HaveKey(constant.ComponentResourcePayload))
				g.Expect(item.ConfigFileParams).Should(HaveKey(testparameters.MysqlConfigFile))
				g.Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("innodb-buffer-pool-size", pointer.String("1024M")))
				g.Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
			})).Should(Succeed())
		})
	})
})
