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

package configuration

import (
	"reflect"
	"strings"

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

	const statefulCompType = "replicasets"

	const configTplName = "mysql-config-tpl"

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
		// must wait until resources deleted and no longer exist before the testcases start,
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
			"resources/mysql_config_cm.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		configConstraintObj = testapps.CreateCustomizedObj(&testCtx,
			"resources/mysql_config_template.yaml",
			&appsv1alpha1.ConfigConstraint{})

		By("Create a clusterDefinition obj")
		clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
			AddComponent(testapps.StatefulMySQLComponent, statefulCompType).
			AddConfigTemplate(configTplName, configMapObj.Name, configConstraintObj.Name, testCtx.DefaultNamespace, configVolumeName, nil).
			Create(&testCtx).GetObject()

		By("Create a clusterVersion obj")
		clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponent(statefulCompType).
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
			availableTPL.Status.Phase = appsv1alpha1.AvailablePhase

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

			_, err := CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCDLabelsByConfiguration(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCDConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())

			err = DeleteCDConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
		})
	})

	Context("clusterdefinition CR test without config Constraints", func() {
		It("Should success without error", func() {
			// remove ConfigConstraintRef
			_, err := handleConfigTemplate(clusterDefObj, func(templates []appsv1alpha1.ConfigTemplate) (bool, error) {
				return true, nil
			}, func(component *appsv1alpha1.ClusterComponentDefinition) error {
				if component.ConfigSpec == nil || len(component.ConfigSpec.ConfigTemplateRefs) == 0 {
					return nil
				}

				for i := range component.ConfigSpec.ConfigTemplateRefs {
					tpl := &component.ConfigSpec.ConfigTemplateRefs[i]
					tpl.ConfigConstraintRef = ""
				}
				return nil
			})
			Expect(err).Should(Succeed())

			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = appsv1alpha1.AvailablePhase

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

			_, err = CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			ok, err := CheckCDConfigTemplate(k8sMockClient.Client(), reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())
		})
	})

	updateAVTemplates := func() {
		var tpls []appsv1alpha1.ConfigTemplate
		_, err := handleConfigTemplate(clusterDefObj, func(templates []appsv1alpha1.ConfigTemplate) (bool, error) {
			tpls = templates
			return true, nil
		})
		Expect(err).Should(Succeed())

		if len(clusterVersionObj.Spec.ComponentVersions) == 0 {
			return
		}

		// mock clusterVersionObj config templates
		clusterVersionObj.Spec.ComponentVersions[0].ConfigTemplateRefs = tpls
	}

	Context("clusterversion CR test", func() {
		It("Should success without error", func() {
			updateAVTemplates()
			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = appsv1alpha1.AvailablePhase

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

			_, err := CheckCVConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCVConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCVConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCVConfigTemplate(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCVLabelsByConfiguration(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCVConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())

			err = DeleteCVConfigMapFinalizer(k8sMockClient.Client(), reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
		})
	})

	Context("common funcs test", func() {
		It("GetReloadOptions Should success without error", func() {
			mockTpl := appsv1alpha1.ConfigConstraint{
				Spec: appsv1alpha1.ConfigConstraintSpec{
					ReloadOptions: &appsv1alpha1.ReloadOptions{
						UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
							Signal:      "HUB",
							ProcessName: "for_test",
						},
					},
				},
			}
			tests := []struct {
				name    string
				tpls    []appsv1alpha1.ConfigTemplate
				want    *appsv1alpha1.ReloadOptions
				wantErr bool
			}{{
				// empty config templates
				name:    "test",
				tpls:    nil,
				want:    nil,
				wantErr: false,
			}, {
				// empty config templates
				name:    "test",
				tpls:    []appsv1alpha1.ConfigTemplate{},
				want:    nil,
				wantErr: false,
			}, {
				// config templates without configConstraintObj
				name: "test",
				tpls: []appsv1alpha1.ConfigTemplate{{
					Name: "for_test",
				}, {
					Name: "for_test2",
				}},
				want:    nil,
				wantErr: false,
			}, {
				// normal
				name: "test",
				tpls: []appsv1alpha1.ConfigTemplate{{
					Name:                "for_test",
					ConfigConstraintRef: "eg_v1",
				}},
				want:    mockTpl.Spec.ReloadOptions,
				wantErr: false,
			}, {
				// not exist config constraint
				name: "test",
				tpls: []appsv1alpha1.ConfigTemplate{{
					Name:                "for_test",
					ConfigConstraintRef: "not_exist",
				}},
				want:    nil,
				wantErr: true,
			}}

			k8sMockClient.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
				if strings.Contains(key.Name, "not_exist") {
					return cfgcore.MakeError("not exist config!")
				}
				testutil.SetGetReturnedObject(obj, &mockTpl)
				return nil
			}, testutil.WithMaxTimes(len(tests))))

			for _, tt := range tests {
				got, err := GetReloadOptions(k8sMockClient.Client(), ctx, tt.tpls)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				Expect(reflect.DeepEqual(got, tt.want)).Should(BeTrue())
			}
		})
	})
})
