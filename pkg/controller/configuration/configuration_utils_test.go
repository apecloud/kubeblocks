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

package configuration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("resource Fetcher", func() {
	var mockK8sCli *testutil.K8sClientMockHelper
	var componentObj *appsv1.Component
	var compDefObj *appsv1.ComponentDefinition
	var clusterComponent *component.SynthesizedComponent
	var paramDef1, paramDef2 *configv1alpha1.ParametersDefinition

	BeforeEach(func() {
		var clusterObj *appsv1.Cluster

		mockK8sCli = testutil.NewK8sMockClient()
		paramDef1, paramDef2 = newParametersDefinition()
		compDefObj = allFieldsCompDefObj(false, func(factory *testapps.MockComponentDefinitionFactory) {
			factory.AddParametersDefinition("file1", paramDef1.Name)
		})
		clusterObj, _, _ = newAllFieldsClusterObj(compDefObj, false)
		clusterComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)
		componentObj = newAllFieldsComponent(clusterObj)
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	Context("ClassifyParamsFromConfigTemplate", func() {
		It("Should succeed with no error with one paramDef", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					paramDef1,
				},
			), testutil.WithAnyTimes()))

			componentObj.Spec.ComponentParameters = appsv1.ComponentParameters{
				"param1": cfgutil.ToPointer("value1"),
				"param2": cfgutil.ToPointer("value2"),
			}

			paramItems, err := ClassifyParamsFromConfigTemplate(testCtx.Ctx, mockK8sCli.Client(), componentObj, compDefObj, clusterComponent)
			Expect(err).Should(Succeed())
			Expect(paramItems).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file1"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param1"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param2"))
		})

		It("Should succeed with no error with multi paramDef", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult(
				[]client.Object{
					paramDef1,
					paramDef2,
				},
			), testutil.WithAnyTimes()))

			compDefObj.Spec.ParametersDescriptions = []appsv1.ComponentParametersDescription{
				{
					Name:              "file1",
					ParametersDefName: paramDef1.Name,
				}, {
					Name:              "file2",
					ParametersDefName: paramDef2.Name,
				},
			}
			clusterComponent.ConfigTemplates[0].ComponentConfigDescriptions = []appsv1.ComponentConfigDescription{
				{Name: "file1",
					ReRenderResourceTypes: []appsv1.RerenderResourceType{
						appsv1.ComponentVScaleType,
						appsv1.ComponentHScaleType,
					},
				},
				{Name: "file2"},
			}
			componentObj.Spec.ComponentParameters = appsv1.ComponentParameters{
				"param1":  cfgutil.ToPointer("value1"),
				"param12": cfgutil.ToPointer("value2"),
			}

			paramItems, err := ClassifyParamsFromConfigTemplate(testCtx.Ctx, mockK8sCli.Client(), componentObj, compDefObj, clusterComponent)
			Expect(err).Should(Succeed())
			Expect(paramItems).Should(HaveLen(1))
			Expect(paramItems[0].ConfigFileParams).Should(HaveLen(2))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file1"))
			Expect(paramItems[0].ConfigFileParams).Should(HaveKey("file2"))
			Expect(paramItems[0].ConfigFileParams["file1"].Parameters).Should(HaveKey("param1"))
			Expect(paramItems[0].ConfigFileParams["file2"].Parameters).Should(HaveKey("param12"))
		})
	})

})
