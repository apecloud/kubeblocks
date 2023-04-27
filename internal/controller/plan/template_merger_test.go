/*
Copyright ApeCloud, Inc.

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

package plan

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
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
	const (
		baseCMName    = "base-cm"
		updatedCMName = "updated-cm"

		testConfigSpecName = "test-config"
		testClusterName    = "test-cluster"
	)

	var (
		mockClient          *testutil.K8sClientMockHelper
		templateBuilder     *configTemplateBuilder
		configSpec          appsv1alpha1.ComponentConfigSpec
		configConstraintObj *appsv1alpha1.ConfigConstraint

		baseCMObject    *corev1.ConfigMap
		updatedCMObject *corev1.ConfigMap
	)

	BeforeEach(func() {
		mockClient = testutil.NewK8sMockClient()
		configConstraintObj = testapps.CheckedCreateCustomizedObj(&testCtx,
			"resources/mysql-config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{})
		baseCMObject = &corev1.ConfigMap{
			Data: map[string]string{
				"my.cnf": baseConfig,
			},
		}
		updatedCMObject = &corev1.ConfigMap{
			Data: map[string]string{
				"my.cnf": extendConfig,
			},
		}
		baseCMObject.SetName(baseCMName)
		baseCMObject.SetNamespace("default")
		updatedCMObject.SetName(updatedCMName)
		updatedCMObject.SetNamespace("default")

		configSpec = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        testConfigSpecName,
				TemplateRef: baseCMObject.GetName(),
				Namespace:   "default",
			},
			ConfigConstraintRef: configConstraintObj.GetName(),
		}

		templateBuilder = newTemplateBuilder(
			testClusterName,
			"default",
			&appsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterName,
					Namespace: "default",
				},
			}, nil, nil, nil)
		Expect(templateBuilder.injectBuiltInObjectsAndFunctions(
			&corev1.PodSpec{}, nil, &component.SynthesizedComponent{}, nil)).Should(Succeed())

		mockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
			baseCMObject,
			updatedCMObject,
			configConstraintObj,
		}), testutil.WithAnyTimes()))
	})

	AfterEach(func() {
		mockClient.Finish()
	})

	Context("with patch Merge", func() {
		It("test mergerConfigTemplate function", func() {
			importedTemplate := &appsv1alpha1.SecondaryRenderedTemplateSpec{
				Namespace: "default",
				// Name:        configSpec.Name,
				TemplateRef: updatedCMObject.GetName(),
				Policy:      appsv1alpha1.PatchPolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, ctx, mockClient.Client())
			Expect(err).To(Succeed())

			configReaders, err := cfgcore.LoadRawConfigObject(mergedData, configConstraintObj.Spec.FormatterConfig, configSpec.Keys)
			Expect(err).Should(Succeed())
			Expect(configReaders).Should(HaveLen(1))
			configObject := configReaders["my.cnf"]
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
		It("test mergerConfigTemplate function", func() {
			importedTemplate := &appsv1alpha1.SecondaryRenderedTemplateSpec{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      appsv1alpha1.ReplacePolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			mergedData, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, ctx, mockClient.Client())
			Expect(err).Should(Succeed())
			Expect(reflect.DeepEqual(mergedData, updatedCMObject.Data)).Should(BeTrue())

			configReaders, err := cfgcore.LoadRawConfigObject(mergedData, configConstraintObj.Spec.FormatterConfig, configSpec.Keys)
			Expect(err).Should(Succeed())
			Expect(configReaders).Should(HaveLen(1))
			configObject := configReaders["my.cnf"]
			Expect(configObject.Get("gtid_mode")).Should(BeNil())
			Expect(configObject.Get("consensus_auto_leader_transfer")).Should(BeNil())
			Expect(configObject.Get("default_storage_engine")).Should(BeEquivalentTo("xengine"))
			Expect(configObject.Get("max_connections")).Should(BeEquivalentTo("666"))
		})
	})

	Context("with only add Merge", func() {
		It("test mergerConfigTemplate function", func() {
			importedTemplate := &appsv1alpha1.SecondaryRenderedTemplateSpec{
				Namespace:   "default",
				TemplateRef: updatedCMObject.GetName(),
				Policy:      appsv1alpha1.OnlyAddPolicy,
			}

			tmpCM := baseCMObject.DeepCopy()
			_, err := mergerConfigTemplate(importedTemplate, templateBuilder, configSpec, tmpCM.Data, ctx, mockClient.Client())
			Expect(err).ShouldNot(Succeed())
		})
	})
})
