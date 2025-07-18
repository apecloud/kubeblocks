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

package configuration

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/configuration/openapi"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("TemplateMergerTest", func() {

	baseConfig := `
[mysqld]
log-bin=master-bin
gtid_mode=OFF
consensus_auto_leader_transfer=ON
max_connections=1000
`
	extendConfig := `
[mysqld]
default_storage_engine=xengine
log_error_verbosity=3
xengine_compression_per_level=kNoCompression:KZSTD:kZSTD
xengine_db_total_write_buffer_size=307
xengine_block_cache_size=307
xengine_row_cache_size=102
max_connections=666
`

	jsonExtenConfig := `
{
    "boolparamf": false,
    "boolparamt": true,
    "intparam": 123,
    "floatparam": 123.456,
    "stringparam": "123"
}
`
	schmaCUE := `
#MysqlParameter: {
  boolparamf: bool
  boolparamt: bool
  intparam: int
  stringparam: string
  floatparam: float
}
`

	testString := "// this is a test string"

	const (
		baseCMName        = "base-cm"
		updatedCMName     = "updated-cm"
		jsonUpdatedCMName = "updated-cm2"

		testConfigSpecName = "test-config"
		testClusterName    = "test-cluster"
		testConfigName     = "my.cnf"
		testConfig2Name    = "test.txt"
	)

	var (
		mockClient      *testutil.K8sClientMockHelper
		templateBuilder render.TemplateRender
		configSpec      appsv1.ComponentFileTemplate
		paramsDefs      *parametersv1alpha1.ParametersDefinition
		pdcr            *parametersv1alpha1.ParamConfigRenderer

		baseCMObject    *corev1.ConfigMap
		updatedCMObject *corev1.ConfigMap

		jsonUpdatedCMObject *corev1.ConfigMap
	)

	BeforeEach(func() {
		mockClient = testutil.NewK8sMockClient()
		paramsDefs = testparameters.NewParametersDefinitionFactory("test-pd").
			SetConfigFile(testConfigName).
			GetObject()
		pdcr = testparameters.NewParamConfigRendererFactory("test-pdcr").
			SetTemplateName(testConfigSpecName).
			GetObject()

		baseCMObject = &corev1.ConfigMap{
			Data: map[string]string{
				testConfigName:  baseConfig,
				testConfig2Name: testString,
			},
		}
		updatedCMObject = &corev1.ConfigMap{
			Data: map[string]string{
				testConfigName:  extendConfig,
				testConfig2Name: testString,
			},
		}
		jsonUpdatedCMObject = &corev1.ConfigMap{
			Data: map[string]string{
				testConfigName: jsonExtenConfig,
			},
		}
		baseCMObject.SetName(baseCMName)
		baseCMObject.SetNamespace(testCtx.DefaultNamespace)
		updatedCMObject.SetName(updatedCMName)
		updatedCMObject.SetNamespace(testCtx.DefaultNamespace)
		jsonUpdatedCMObject.SetName(jsonUpdatedCMName)
		jsonUpdatedCMObject.SetNamespace(testCtx.DefaultNamespace)

		configSpec = appsv1.ComponentFileTemplate{
			Name:      testConfigSpecName,
			Template:  baseCMObject.GetName(),
			Namespace: "default",
		}

		templateBuilder = render.NewTemplateBuilder(&render.ReconcileCtx{
			ResourceCtx: &render.ResourceCtx{
				Context:     ctx,
				Client:      mockClient.Client(),
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: testClusterName,
			},
			Cluster: &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterName,
					Namespace: "default",
				},
			},
			SynthesizedComponent: &component.SynthesizedComponent{},
			PodSpec:              nil,
		})

		mockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
			baseCMObject,
			updatedCMObject,
			jsonUpdatedCMObject,
		}), testutil.WithAnyTimes()))
		mockClient.MockNListMethod(0, testutil.WithListReturned(
			testutil.WithConstructListReturnedResult([]runtime.Object{pdcr}),
			testutil.WithAnyTimes(),
		))
	})

	AfterEach(func() {
		mockClient.Finish()
	})

	Context("with patch Merge", func() {
		It("mergerConfigTemplate patch policy", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace: "default",
				// Name:        configSpec.Name,
				TemplateRef: updatedCMObject.GetName(),
				Policy:      parametersv1alpha1.PatchPolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, pdcr)
			Expect(err).To(Succeed())
			Expect(mergedData).Should(HaveLen(2))

			configReaders, err := cfgcore.LoadRawConfigObject(mergedData, pdcr.Spec.Configs[0].FileFormatConfig, []string{testConfigName})
			Expect(err).Should(Succeed())
			Expect(configReaders).Should(HaveLen(1))
			configObject := configReaders[testConfigName]
			Expect(configObject.Get("gtid_mode")).Should(BeEquivalentTo("OFF"))
			Expect(configObject.Get("consensus_auto_leader_transfer")).Should(BeEquivalentTo("ON"))
			Expect(configObject.Get("default_storage_engine")).Should(BeEquivalentTo("xengine"))
			Expect(configObject.Get("log_error_verbosity")).Should(BeEquivalentTo("3"))
			Expect(configObject.Get("max_connections")).Should(BeEquivalentTo("666"))
			Expect(configObject.Get("xengine_compression_per_level")).Should(BeEquivalentTo("kNoCompression:KZSTD:kZSTD"))
			Expect(configObject.Get("xengine_block_cache_size")).Should(BeEquivalentTo("307"))
		})
	})

	Context("with replace Merge", func() {
		It("test mergerConfigTemplate replace policy", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      parametersv1alpha1.ReplacePolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, pdcr)
			Expect(err).Should(Succeed())
			Expect(mergedData).Should(HaveLen(2))
			Expect(reflect.DeepEqual(mergedData, updatedCMObject.Data)).Should(BeTrue())

			configReaders, err := cfgcore.LoadRawConfigObject(mergedData, pdcr.Spec.Configs[0].FileFormatConfig, []string{testConfigName})
			Expect(err).Should(Succeed())
			Expect(configReaders).Should(HaveLen(1))
			configObject := configReaders[testConfigName]
			Expect(configObject.Get("gtid_mode")).Should(BeNil())
			Expect(configObject.Get("consensus_auto_leader_transfer")).Should(BeNil())
			Expect(configObject.Get("default_storage_engine")).Should(BeEquivalentTo("xengine"))
			Expect(configObject.Get("max_connections")).Should(BeEquivalentTo("666"))
		})
	})

	Context("with only add Merge", func() {
		It("test mergerConfigTemplate add policy", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      parametersv1alpha1.OnlyAddPolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			_, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, pdcr)
			Expect(err).ShouldNot(Succeed())
		})
	})

	Context("with none Merge", func() {
		It("test mergerConfigTemplate none policy", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      parametersv1alpha1.NoneMergePolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, pdcr)
			Expect(err).Should(Succeed())
			Expect(reflect.DeepEqual(mergedData, updatedCMObject.Data)).Should(BeTrue())
		})
	})

	Context("failed test", func() {
		It("test mergerConfigTemplate function", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      "",
			}

			tmpCM := baseCMObject.DeepCopy()
			_, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, pdcr)
			Expect(err).ShouldNot(Succeed())
		})

		It("not parameterDrivenConfigRender", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      "none",
			}

			tmpCM := baseCMObject.DeepCopy()
			_, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, &parametersv1alpha1.ParamConfigRenderer{})
			Expect(err).Should(Succeed())
		})
	})

	Context("json patch Merge", func() {
		It("mergerConfigTemplate patch policy for json format", func() {
			importedTemplate := parametersv1alpha1.ConfigTemplateExtension{
				Namespace:   testCtx.DefaultNamespace,
				TemplateRef: jsonUpdatedCMObject.GetName(),
				Policy:      parametersv1alpha1.PatchPolicy,
			}

			openAPISchema, err := openapi.GenerateOpenAPISchema(schmaCUE, "")
			Expect(err).Should(Succeed())
			paramsDefs.Spec.ParametersSchema = &parametersv1alpha1.ParametersSchema{
				CUE:          schmaCUE,
				SchemaInJSON: openAPISchema,
			}

			mockData := map[string]string{
				testConfigName: "{}",
			}
			mockPcr := pdcr.DeepCopy()
			mockPcr.Spec.Configs[0].FileFormatConfig = &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			}
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, mockData, []*parametersv1alpha1.ParametersDefinition{paramsDefs}, mockPcr)
			Expect(err).To(Succeed())
			Expect(mergedData).Should(HaveLen(1))

			configReaders, err := cfgcore.LoadRawConfigObject(mergedData, mockPcr.Spec.Configs[0].FileFormatConfig, []string{testConfigName})
			Expect(err).Should(Succeed())
			Expect(configReaders).Should(HaveLen(1))
			configObject := configReaders[testConfigName]
			Expect(configObject.Get("boolparamf")).Should(BeFalse())
			Expect(configObject.Get("boolparamt")).Should(BeTrue())
			Expect(configObject.Get("intparam")).Should(BeEquivalentTo(123))
			Expect(configObject.Get("floatparam")).Should(Equal(123.456))
			Expect(configObject.Get("stringparam")).Should(Equal("123"))
		})
	})

})
