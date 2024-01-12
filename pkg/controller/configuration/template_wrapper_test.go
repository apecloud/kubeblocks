/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("TemplateWrapperTest", func() {

	var mockK8sCli *testutil.K8sClientMockHelper
	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var clusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent

	mockTemplateWrapper := func() renderWrapper {
		mockConfigTemplater := newTemplateBuilder(clusterName, testCtx.DefaultNamespace, clusterObj, ctx, mockK8sCli.Client(), nil)
		Expect(mockConfigTemplater.injectBuiltInObjectsAndFunctions(&corev1.PodSpec{}, clusterComponent.ConfigTemplates, clusterComponent, nil)).Should(Succeed())
		return newTemplateRenderWrapper(mockConfigTemplater, clusterObj, ctx, mockK8sCli.Client())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()

		clusterObj, clusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(clusterDefObj, clusterVersionObj, clusterObj)
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	Context("TestConfigSpec", func() {
		It("TestConfigSpec", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configSpecName,
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{
						configSpecName: testConfigContent,
					},
				},
			}), testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil, nil)).ShouldNot(Succeed())
		})

		It("TestConfigSpec with exist configmap", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cfgcore.GetComponentCfgName(clusterName, clusterComponent.Name, clusterComponent.ConfigTemplates[0].Name),
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{
						configSpecName: testConfigContent,
					},
				},
			}), testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil, nil)).Should(Succeed())
		})

		It("TestConfigSpec update", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cfgcore.GetComponentCfgName(clusterName, clusterComponent.Name, clusterComponent.ConfigTemplates[0].Name),
						Namespace: testCtx.DefaultNamespace,
						Labels:    make(map[string]string),
						Annotations: map[string]string{
							constant.CMInsEnableRerenderTemplateKey:       "true",
							constant.KBParameterUpdateSourceAnnotationKey: constant.ReconfigureManagerSource,
						},
					},
					Data: map[string]string{
						configSpecName: testConfigContent,
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:        configSpecName,
						Namespace:   testCtx.DefaultNamespace,
						Labels:      make(map[string]string),
						Annotations: make(map[string]string),
					},
					Data: map[string]string{
						"test-config-spec-new": "test-config-spec-update",
					},
				},
				&appsv1alpha1.ConfigConstraint{
					ObjectMeta: metav1.ObjectMeta{
						Name: configSpecName,
					},
					Spec: appsv1alpha1.ConfigConstraintSpec{
						FormatterConfig: &appsv1alpha1.FormatterConfig{
							Format: appsv1alpha1.Ini,
						},
					},
				},
			}), testutil.WithAnyTimes()))
			mockK8sCli.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				return nil
			}, testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil, nil)).Should(Succeed())
		})

	})

	Context("TestScriptsSpec", func() {

		It("TestScriptSpec", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      mysqlScriptsConfigName,
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{
						configSpecName: testConfigContent,
					},
				},
			}), testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderScriptTemplate(clusterObj, clusterComponent, nil)).Should(Succeed())
		})

		It("TestScriptSpec with exist", func() {
			cmObj := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cfgcore.GetComponentCfgName(clusterName, clusterComponent.Name, clusterComponent.ScriptTemplates[0].Name),
					Namespace: testCtx.DefaultNamespace,
				},
				Data: map[string]string{
					configSpecName: testConfigContent,
				},
			}
			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderScriptTemplate(clusterObj, clusterComponent, []client.Object{cmObj})).Should(Succeed())
		})
	})
})
