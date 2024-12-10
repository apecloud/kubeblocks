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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
	"github.com/apecloud/kubeblocks/pkg/unstructured"
)

var _ = Describe("ToolsImageBuilderTest", func() {

	var mockK8sCli *testutil.K8sClientMockHelper
	var clusterObj *appsv1.Cluster
	var compDefObj *appsv1.ComponentDefinition
	var clusterComponent *component.SynthesizedComponent
	var paramsDef *parametersv1alpha1.ParametersDefinition
	var pdcr *parametersv1alpha1.ParameterDrivenConfigRender

	BeforeEach(func() {
		mockK8sCli = testutil.NewK8sMockClient()

		// Add any setup steps that needs to be executed before each test
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		paramsDef = testparameters.NewParametersDefinitionFactory(paramsDefName).
			SetReloadAction(testparameters.WithNoneAction()).
			GetObject()
		clusterComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)

		pdcr = testparameters.NewParametersDrivenConfigFactory(pdcrName).
			SetParametersDefs(paramsDef.Name).
			SetComponentDefinition(compDefObj.GetName()).
			SetTemplateName(configTemplateName).
			GetObject()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		mockK8sCli.Finish()
	})

	deRef := func(m map[string]*parametersv1alpha1.ParametersInFile) map[string]parametersv1alpha1.ParametersInFile {
		r := make(map[string]parametersv1alpha1.ParametersInFile, len(m))
		for key, param := range m {
			r[key] = *param
		}
		return r
	}

	rerender := func(parameters parametersv1alpha1.ComponentParameters, tpl *corev1.ConfigMap, customTemplate *appsv1.ConfigTemplateExtension) unstructured.ConfigObject {
		rctx := &render.ReconcileCtx{
			ResourceCtx: &render.ResourceCtx{
				Context:       testCtx.Ctx,
				Client:        mockK8sCli.Client(),
				ClusterName:   clusterObj.Name,
				ComponentName: mysqlCompName,
			},
			Cluster:              clusterObj,
			SynthesizedComponent: clusterComponent,
			PodSpec:              &corev1.PodSpec{},
		}
		item := parametersv1alpha1.ConfigTemplateItemDetail{
			Name: configTemplateName,
			ConfigSpec: &appsv1.ComponentTemplateSpec{
				Name:        configTemplateName,
				Namespace:   testCtx.DefaultNamespace,
				TemplateRef: mysqlConfigName,
				VolumeName:  testapps.ConfVolumeName,
			},
			CustomTemplates: customTemplate,
		}
		pds := []*parametersv1alpha1.ParametersDefinition{paramsDef}
		cmObj, err := RerenderParametersTemplate(rctx, item, pdcr, pds)
		Expect(err).Should(Succeed())
		configdesc := pdcr.Spec.Configs[0]
		if len(parameters) == 0 {
			configReaders, err := cfgcore.LoadRawConfigObject(cmObj.Data, configdesc.FileFormatConfig, []string{configdesc.Name})
			Expect(err).Should(Succeed())
			return configReaders[configdesc.Name]
		}
		params := ClassifyComponentParameters(parameters, pds, []appsv1.ComponentTemplateSpec{*item.ConfigSpec}, map[string]*corev1.ConfigMap{configTemplateName: tpl})

		tplParams, ok := params[configTemplateName]
		Expect(ok).Should(BeTrue())
		item.ConfigFileParams = deRef(tplParams)
		result, err := ApplyParameters(item, cmObj, pdcr, pds)
		Expect(err).Should(Succeed())
		configReaders, err := cfgcore.LoadRawConfigObject(result.Data, configdesc.FileFormatConfig, []string{configdesc.Name})
		Expect(err).Should(Succeed())
		return configReaders[configdesc.Name]
	}

	Context("RerenderParametersTemplateTest", func() {
		It("Render config with parameters", func() {
			configMapObj := testparameters.NewComponentTemplateFactory(mysqlConfigName, testCtx.DefaultNamespace).
				GetObject()

			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				configMapObj,
			}), testutil.WithAnyTimes()))

			renderedObj := rerender(map[string]*string{
				"max_connections":  pointer.String("100"),
				"read_buffer_size": pointer.String("55288"),
			}, configMapObj, nil)
			Expect(renderedObj).ShouldNot(BeNil())
			Expect(renderedObj.Get("max_connections")).Should(BeEquivalentTo("100"))
			Expect(renderedObj.Get("read_buffer_size")).Should(BeEquivalentTo("55288"))
		})

		It("Render config with custom template", func() {
			configMapObj := testparameters.NewComponentTemplateFactory(mysqlConfigName, testCtx.DefaultNamespace).
				GetObject()
			customTemplate := testparameters.NewComponentTemplateFactory(mysqlConfigName, testCtx.DefaultNamespace).
				AddConfigFile(testparameters.MysqlConfigFile, `
[mysqld]
innodb_buffer_pool_size=512M
gtid_mode=OFF
`).
				GetObject()

			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				configMapObj,
				customTemplate,
			}), testutil.WithAnyTimes()))

			renderedObj := rerender(nil, nil, &appsv1.ConfigTemplateExtension{
				TemplateRef: customTemplate.Name,
				Namespace:   testCtx.DefaultNamespace,
				Policy:      appsv1.ReplacePolicy,
			})
			Expect(renderedObj).ShouldNot(BeNil())
			Expect(renderedObj.Get("innodb_buffer_pool_size")).Should(BeEquivalentTo("512M"))
			Expect(renderedObj.Get("gtid_mode")).Should(BeEquivalentTo("OFF"))
			Expect(renderedObj.Get("max_connections")).Should(BeNil())
		})
	})
})
