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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("TemplateWrapperTest", func() {

	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"
	const mysqlCompDefName = "replicasets"
	const scriptConfigName = "test-script-config"
	const configSpecName = "test-config-spec"
	const mysqlCompName = "mysql"

	var mockK8sCli *testutil.K8sClientMockHelper
	var clusterObj *appsv1alpha1.Cluster
	var clusterVersionObj *appsv1alpha1.ClusterVersion
	var ClusterDefObj *appsv1alpha1.ClusterDefinition
	var clusterComponent *component.SynthesizedComponent

	allFieldsClusterDefObj := func(needCreate bool) *appsv1alpha1.ClusterDefinition {
		By("By assure an clusterDefinition obj")
		clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
			AddScriptTemplate(scriptConfigName, scriptConfigName, testCtx.DefaultNamespace, testapps.ScriptsVolumeName, nil).
			AddConfigTemplate(configSpecName, configSpecName, configSpecName, testCtx.DefaultNamespace, testapps.ConfVolumeName).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterDefObj)).Should(Succeed())
		}
		return clusterDefObj
	}

	allFieldsClusterVersionObj := func(needCreate bool) *appsv1alpha1.ClusterVersion {
		By("By assure an clusterVersion obj")
		clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
			AddComponentVersion(mysqlCompDefName).
			AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
			GetObject()
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
		}
		return clusterVersionObj
	}

	newAllFieldsClusterObj := func(
		clusterDefObj *appsv1alpha1.ClusterDefinition,
		clusterVersionObj *appsv1alpha1.ClusterVersion,
		needCreate bool,
	) (*appsv1alpha1.Cluster, *appsv1alpha1.ClusterDefinition, *appsv1alpha1.ClusterVersion, types.NamespacedName) {
		// setup Cluster obj requires default ClusterDefinition and ClusterVersion objects
		if clusterDefObj == nil {
			clusterDefObj = allFieldsClusterDefObj(needCreate)
		}
		if clusterVersionObj == nil {
			clusterVersionObj = allFieldsClusterVersionObj(needCreate)
		}
		pvcSpec := testapps.NewPVCSpec("1Gi")
		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).
			AddComponent(mysqlCompName, mysqlCompDefName).SetReplicas(1).
			AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
			AddService(testapps.ServiceVPCName, corev1.ServiceTypeLoadBalancer).
			AddService(testapps.ServiceInternetName, corev1.ServiceTypeLoadBalancer).
			GetObject()
		key := client.ObjectKeyFromObject(clusterObj)
		if needCreate {
			Expect(testCtx.CreateObj(testCtx.Ctx, clusterObj)).Should(Succeed())
		}
		return clusterObj, clusterDefObj, clusterVersionObj, key
	}

	newAllFieldsComponent := func(clusterDef *appsv1alpha1.ClusterDefinition, clusterVersion *appsv1alpha1.ClusterVersion) *component.SynthesizedComponent {
		cluster, clusterDef, clusterVersion, _ := newAllFieldsClusterObj(clusterDef, clusterVersion, false)
		By("assign every available fields")
		component, err := component.BuildComponent(
			intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Log: logger,
			},
			*cluster,
			*clusterDef,
			clusterDef.Spec.ComponentDefs[0],
			cluster.Spec.ComponentSpecs[0],
			&clusterVersion.Spec.ComponentVersions[0])
		Expect(err).Should(Succeed())
		Expect(component).ShouldNot(BeNil())
		return component
	}

	mockTemplateWrapper := func() renderWrapper {
		mockConfigTemplater := newTemplateBuilder(clusterName, testCtx.DefaultNamespace, clusterObj, clusterVersionObj, ctx, mockK8sCli.Client())
		Expect(mockConfigTemplater.injectBuiltInObjectsAndFunctions(&corev1.PodSpec{}, clusterComponent.ConfigTemplates, clusterComponent, nil)).Should(Succeed())
		return newTemplateRenderWrapper(mockConfigTemplater, clusterObj, ctx, mockK8sCli.Client())
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()

		clusterObj, ClusterDefObj, clusterVersionObj, _ = newAllFieldsClusterObj(nil, nil, false)
		clusterComponent = newAllFieldsComponent(ClusterDefObj, clusterVersionObj)
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
						"test-config-spec": "test-config-spec",
					},
				},
			}), testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil)).ShouldNot(Succeed())
		})

		It("TestConfigSpec with exist configmap", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cfgcore.GetComponentCfgName(clusterName, clusterComponent.Name, clusterComponent.ConfigTemplates[0].Name),
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{
						"test-config-spec": "test-config-spec",
					},
				},
			}), testutil.WithAnyTimes()))

			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil)).Should(Succeed())
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
						"test-config-spec": "test-config-spec-update",
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
			Expect(tplWrapper.renderConfigTemplate(clusterObj, clusterComponent, nil)).Should(Succeed())
		})

	})

	Context("TestScriptsSpec", func() {

		It("TestScriptSpec", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      scriptConfigName,
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{
						"test-config-spec": "test-config-spec",
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
					"test-config-spec": "test-config-spec",
				},
			}
			tplWrapper := mockTemplateWrapper()
			Expect(tplWrapper.renderScriptTemplate(clusterObj, clusterComponent, []client.Object{cmObj})).Should(Succeed())
		})
	})
})
