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
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ConfigurationPipelineTest", func() {
	const testConfigFile = "postgresql.conf"

	var clusterObj *appsv1.Cluster
	var componentObj *appsv1.Component
	var compDefObj *appsv1.ComponentDefinition
	var synthesizedComponent *component.SynthesizedComponent
	var configMapObj *corev1.ConfigMap
	var parametersDef *parametersv1alpha1.ParametersDefinition
	var componentParameter *parametersv1alpha1.ComponentParameter
	var configRender *parametersv1alpha1.ParamConfigRenderer
	var k8sMockClient *testutil.K8sClientMockHelper

	mockAPIResource := func(lazyFetcher testutil.Getter) {
		k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
			[]client.Object{
				compDefObj,
				clusterObj,
				clusterObj,
				configMapObj,
				parametersDef,
				componentObj,
				componentParameter,
				configRender,
			}, lazyFetcher), testutil.WithAnyTimes()))
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
		}, testutil.WithAnyTimes()))
		k8sMockClient.MockNListMethod(0, testutil.WithListReturned(
			testutil.WithConstructListReturnedResult([]runtime.Object{configRender}),
			testutil.WithAnyTimes(),
		))
		k8sMockClient.MockStatusMethod().
			EXPECT().
			Patch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				switch v := obj.(type) {
				case *parametersv1alpha1.ComponentParameter:
					if client.ObjectKeyFromObject(obj) == client.ObjectKeyFromObject(componentParameter) {
						componentParameter.Status = *v.Status.DeepCopy()
					}
				}
				return nil
			}).AnyTimes()
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		componentObj = newAllFieldsComponent(clusterObj)
		synthesizedComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)
		configMapObj = testapps.NewConfigMap("default", mysqlConfigName,
			testapps.SetConfigMapData(testConfigFile, `
bgwriter_delay = '200ms'
bgwriter_flush_after = '64'
bgwriter_lru_maxpages = '1000'
bgwriter_lru_multiplier = '10.0'
bytea_output = 'hex'
check_function_bodies = 'True'
checkpoint_completion_target = '0.9'
checkpoint_flush_after = '32'
checkpoint_timeout = '15min'
max_connections = '1000'
`))
		componentParameter = builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
			cfgcore.GenerateComponentConfigurationName(clusterName, mysqlCompName)).
			ClusterRef(clusterName).
			Component(mysqlCompName).
			GetObject()

		parametersDef = testparameters.NewParametersDefinitionFactory(paramsDefName).GetObject()
		configRender = testparameters.NewParamConfigRendererFactory(pdcrName).
			SetConfigDescription(testConfigFile, configTemplateName, parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Properties}).
			SetComponentDefinition(compDefObj.Name).
			SetParametersDefs(paramsDefName).
			GetObject()
		parametersDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
		configRender.Status.Phase = parametersv1alpha1.PDAvailablePhase
	})

	AfterEach(func() {
		k8sMockClient.Finish()
	})

	Context("ConfigPipelineTest", func() {
		It("NormalTest", func() {
			By("create configuration resource")
			createPipeline := NewCreatePipeline(render.ReconcileCtx{
				ResourceCtx: &render.ResourceCtx{
					Client:        k8sMockClient.Client(),
					Context:       ctx,
					Namespace:     testCtx.DefaultNamespace,
					ClusterName:   clusterName,
					ComponentName: mysqlCompName,
				},
				Cluster:              clusterObj,
				Component:            componentObj,
				SynthesizedComponent: synthesizedComponent,
				PodSpec:              synthesizedComponent.PodSpec,
			})

			By("mock api resource for configuration")
			mockAPIResource(func(key client.ObjectKey, obj client.Object) (bool, error) {
				switch obj.(type) {
				case *corev1.ConfigMap:
					for _, renderedObj := range createPipeline.renderWrapper.renderedObjs {
						if client.ObjectKeyFromObject(renderedObj) == key {
							testutil.SetGetReturnedObject(obj, renderedObj)
							return true, nil
						}
					}
				}
				return false, nil
			})

			err := createPipeline.
				ComponentAndComponentDef().
				Prepare().
				SyncComponentParameter().
				ComponentParameter().
				CreateConfigTemplate().
				UpdatePodVolumes().
				BuildConfigManagerSidecar().
				UpdateConfigRelatedObject().
				Complete()
			Expect(err).Should(Succeed())

			By("update configuration resource for mocking reconfiguring")
			item := componentParameter.Spec.ConfigItemDetails[0]
			status := &parametersv1alpha1.ConfigTemplateItemDetailStatus{
				Name:  item.Name,
				Phase: parametersv1alpha1.CInitPhase,
			}
			item.ConfigFileParams = map[string]parametersv1alpha1.ParametersInFile{
				testConfigFile: {
					Parameters: map[string]*string{
						"max_connections": cfgutil.ToPointer("2000"),
					},
				},
				"other.conf": {
					Content: cfgutil.ToPointer(`for test`),
				},
			}
			reconcileTask := NewReconcilePipeline(render.ReconcileCtx{
				ResourceCtx:          createPipeline.ResourceCtx,
				Cluster:              clusterObj,
				Component:            componentObj,
				SynthesizedComponent: synthesizedComponent,
				PodSpec:              synthesizedComponent.PodSpec,
			}, item, status, configMapObj, componentParameter)

			By("update configuration resource")
			err = reconcileTask.
				ComponentAndComponentDef().
				PrepareForTemplate().
				RerenderTemplate().
				ApplyParameters().
				UpdateConfigVersion(strconv.FormatInt(reconcileTask.ComponentParameterObj.GetGeneration(), 10)).
				Sync().
				Complete()
			Expect(err).Should(Succeed())
		})
	})

})
