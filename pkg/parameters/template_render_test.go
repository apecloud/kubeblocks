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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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
	var pdcr *parametersv1alpha1.ParamConfigRenderer

	BeforeEach(func() {
		mockK8sCli = testutil.NewK8sMockClient()

		// Add any setup steps that needs to be executed before each test
		clusterObj, compDefObj, _ = newAllFieldsClusterObj(nil, false)
		paramsDef = testparameters.NewParametersDefinitionFactory(paramsDefName).
			SetReloadAction(testparameters.WithNoneAction()).
			GetObject()
		clusterComponent = newAllFieldsSynthesizedComponent(compDefObj, clusterObj)

		pdcr = testparameters.NewParamConfigRendererFactory(pdcrName).
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

	rerender := func(parameters parametersv1alpha1.ComponentParameters, tpl *corev1.ConfigMap, customTemplate *parametersv1alpha1.ConfigTemplateExtension) unstructured.ConfigObject {
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
			ConfigSpec: &appsv1.ComponentFileTemplate{
				Name:       configTemplateName,
				Namespace:  testCtx.DefaultNamespace,
				Template:   mysqlConfigName,
				VolumeName: testapps.ConfVolumeName,
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
		params, err := ClassifyComponentParameters(parameters, pds, []appsv1.ComponentFileTemplate{*item.ConfigSpec}, map[string]*corev1.ConfigMap{configTemplateName: tpl}, pdcr)
		Expect(err).Should(Succeed())

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

			renderedObj := rerender(nil, nil, &parametersv1alpha1.ConfigTemplateExtension{
				TemplateRef: customTemplate.Name,
				Namespace:   testCtx.DefaultNamespace,
				Policy:      parametersv1alpha1.ReplacePolicy,
			})
			Expect(renderedObj).ShouldNot(BeNil())
			Expect(renderedObj.Get("innodb_buffer_pool_size")).Should(BeEquivalentTo("512M"))
			Expect(renderedObj.Get("gtid_mode")).Should(BeEquivalentTo("OFF"))
			Expect(renderedObj.Get("max_connections")).Should(BeNil())
		})
	})
})
