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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ConfigWrapper util test", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const statefulCompDefName = "replicasets"
	const configSpecName = "mysql-config-tpl"
	const configVolumeName = "mysql-config"

	var (
		// ctrl       *gomock.Controller
		// mockClient *mock_client.MockClient
		k8sMockClient *testutil.K8sClientMockHelper

		reqCtx = intctrlutil.RequestCtx{
			Ctx: ctx,
			Log: log.FromContext(ctx).WithValues("reconfigure_for_test", testCtx.DefaultNamespace),
		}
	)

	var (
		configMapObj        *corev1.ConfigMap
		configConstraintObj *appsv1alpha1.ConfigConstraint
		clusterDefObj       *appsv1alpha1.ClusterDefinition
		clusterVersionObj   *appsv1alpha1.ClusterVersion
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ClusterVersionSignature, ml)
		testapps.ClearResources(&testCtx, generics.ClusterDefinitionSignature, ml)
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

		// Add any setup steps that needs to be executed before each test
		k8sMockClient = testutil.NewK8sMockClient()

		By("creating a cluster")
		configMapObj = testapps.CreateCustomizedObj(&testCtx,
			"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		configConstraintObj = testapps.CreateCustomizedObj(&testCtx,
			"resources/mysql-config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{})

		By("Create a clusterDefinition obj")
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponentDef(testapps.StatefulMySQLComponent, statefulCompDefName).
			AddConfigTemplate(configSpecName, configMapObj.Name, configConstraintObj.Name, testCtx.DefaultNamespace, configVolumeName).
			Create(&testCtx).GetObject()

		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponentVersion(statefulCompDefName).
			Create(&testCtx).GetObject()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanEnv()

		k8sMockClient.Finish()
	})

	Context("clusterdefinition CR test", func() {
		It("Should success without error", func() {
			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = appsv1alpha1.CCAvailablePhase

			k8sMockClient.MockPatchMethod(testutil.WithSucceed())
			k8sMockClient.MockListMethod(testutil.WithSucceed())
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSequenceResult(
				map[client.ObjectKey][]testutil.MockGetReturned{
					client.ObjectKeyFromObject(configMapObj): {{
						Object: nil,
						Err:    cfgcore.MakeError("failed to get cc object"),
					}, {
						Object: configMapObj,
						Err:    nil,
					}},
					client.ObjectKeyFromObject(configConstraintObj): {{
						Object: nil,
						Err:    cfgcore.MakeError("failed to get cc object"),
					}, {
						Object: configConstraintObj,
						Err:    nil,
					}, {
						Object: availableTPL,
						Err:    nil,
					}},
				},
			), testutil.WithAnyTimes()))

			_, err := checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = updateLabelsByConfigSpec(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			_, err = updateLabelsByConfigSpec(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())

			err = DeleteConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
		})
	})

	Context("clusterdefinition CR test without config Constraints", func() {
		It("Should success without error", func() {
			// remove ConfigConstraintRef
			_, err := handleConfigTemplate(clusterDefObj, func(templates []appsv1alpha1.ComponentConfigSpec) (bool, error) {
				return true, nil
			}, func(component *appsv1alpha1.ClusterComponentDefinition) error {
				if len(component.ConfigSpecs) == 0 {
					return nil
				}
				for i := range component.ConfigSpecs {
					tpl := &component.ConfigSpecs[i]
					tpl.ConfigConstraintRef = ""
				}
				return nil
			})
			Expect(err).Should(Succeed())

			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = appsv1alpha1.CCAvailablePhase

			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSequenceResult(
				map[client.ObjectKey][]testutil.MockGetReturned{
					client.ObjectKeyFromObject(configMapObj): {{
						Object: nil,
						Err:    cfgcore.MakeError("failed to get cc object"),
					}, {
						Object: configMapObj,
						Err:    nil,
					}}},
			), testutil.WithAnyTimes()))

			_, err = checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			ok, err := checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())
		})
	})

	updateAVTemplates := func() {
		var tpls []appsv1alpha1.ComponentConfigSpec
		_, err := handleConfigTemplate(clusterDefObj, func(templates []appsv1alpha1.ComponentConfigSpec) (bool, error) {
			tpls = templates
			return true, nil
		})
		Expect(err).Should(Succeed())

		if len(clusterVersionObj.Spec.ComponentVersions) == 0 {
			return
		}

		// mock clusterVersionObj config templates
		clusterVersionObj.Spec.ComponentVersions[0].ConfigSpecs = tpls
	}

	Context("clusterversion CR test", func() {
		It("Should success without error", func() {
			updateAVTemplates()
			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = appsv1alpha1.CCAvailablePhase

			k8sMockClient.MockPatchMethod(testutil.WithSucceed())
			k8sMockClient.MockListMethod(testutil.WithSucceed())
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSequenceResult(
				map[client.ObjectKey][]testutil.MockGetReturned{
					client.ObjectKeyFromObject(configMapObj): {{
						Object: nil,
						Err:    cfgcore.MakeError("failed to get cc object"),
					}, {
						Object: configMapObj,
						Err:    nil,
					}},
					client.ObjectKeyFromObject(configConstraintObj): {{
						Object: nil,
						Err:    cfgcore.MakeError("failed to get cc object"),
					}, {
						Object: configConstraintObj,
						Err:    nil,
					}, {
						Object: availableTPL,
						Err:    nil,
					}},
				},
			), testutil.WithAnyTimes()))

			_, err := checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := checkConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = updateLabelsByConfigSpec(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			_, err = updateLabelsByConfigSpec(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())

			err = DeleteConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
		})
	})

})
