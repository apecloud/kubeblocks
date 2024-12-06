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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ConfigurationOperatorTest", func() {
	var clusterObj *appsv1.Cluster
	var compDefObj *appsv1.ComponentDefinition
	var componentObj *appsv1.Component
	var synthesizedComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var scriptsObj *corev1.ConfigMap
	var parametersDef *parametersv1alpha1.ParametersDefinition
	var configRender *parametersv1alpha1.ParameterDrivenConfigRender
	var componentParameter *parametersv1alpha1.ComponentParameter
	var k8sMockClient *testutil.K8sClientMockHelper

	createConfigReconcileTask := func() *configOperator {
		task := NewConfigReconcileTask(&render.ResourceCtx{
			Client:        k8sMockClient.Client(),
			Context:       ctx,
			Namespace:     testCtx.DefaultNamespace,
			ClusterName:   clusterName,
			ComponentName: mysqlCompName,
		},
			clusterObj,
			componentObj,
			synthesizedComponent,
			synthesizedComponent.PodSpec,
			nil)
		return task
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		synthesizedComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)
		componentObj = newAllFieldsComponent(clusterObj)
		configMapObj = testapps.NewConfigMap("default", mysqlConfigName,
			testapps.SetConfigMapData("test", "test"))
		scriptsObj = testapps.NewConfigMap("default", mysqlScriptsTemplateName,
			testapps.SetConfigMapData("script.sh", "echo \"hello\""))
		componentParameter = builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
			cfgcore.GenerateComponentConfigurationName(clusterName, mysqlCompName)).
			ClusterRef(clusterName).
			Component(mysqlCompName).
			GetObject()
		parametersDef = testparameters.NewParametersDefinitionFactory(paramsDefName).GetObject()
		configRender = testparameters.NewParametersDrivenConfigFactory(pdcrName).
			SetComponentDefinition(compDefObj.Name).
			SetParametersDefs(paramsDefName).
			GetObject()
		parametersDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
		configRender.Status.Phase = parametersv1alpha1.PDAvailablePhase
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ConfigOperatorTest", func() {
		It("NormalTest", func() {
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					compDefObj,
					componentObj,
					clusterObj,
					scriptsObj,
					configMapObj,
					parametersDef,
					configRender,
					componentParameter,
				},
			), testutil.WithAnyTimes()))
			k8sMockClient.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithAnyTimes()))
			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				switch v := obj.(type) {
				case *parametersv1alpha1.ComponentParameter:
					if client.ObjectKeyFromObject(obj) == client.ObjectKeyFromObject(componentParameter) {
						componentParameter.Spec = *v.Spec.DeepCopy()
						componentParameter.Status = *v.Status.DeepCopy()
					}
				}
				return nil
			}))
			k8sMockClient.MockNListMethod(0, testutil.WithListReturned(
				testutil.WithConstructListReturnedResult([]runtime.Object{configRender}),
				testutil.WithAnyTimes(),
			))
			Expect(createConfigReconcileTask().Reconcile()).Should(Succeed())
		})

		It("EmptyConfigSpecTest", func() {

			// k8sMockClient.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithTimes(1)))
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					compDefObj,
					componentObj,
					clusterObj,
					configMapObj,
					componentParameter,
				},
			), testutil.WithAnyTimes()))
			k8sMockClient.MockPatchMethod(testutil.WithSucceed(), testutil.WithAnyTimes())
			k8sMockClient.MockNListMethod(0, testutil.WithListReturned(
				testutil.WithConstructListReturnedResult([]runtime.Object{configRender}),
				testutil.WithAnyTimes(),
			))

			synthesizedComponent.ConfigTemplates = nil
			synthesizedComponent.ScriptTemplates = nil
			Expect(createConfigReconcileTask().Reconcile()).Should(Succeed())
		})

	})

})
