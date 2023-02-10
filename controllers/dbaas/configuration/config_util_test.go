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
	"context"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

var _ = Describe("ConfigWrapper util test", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"

	const statefulCompType = "replicasets"

	const configTplName = "mysql-config-tpl"

	const configVolumeName = "mysql-config"

	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient

		reqCtx = intctrlutil.RequestCtx{
			Ctx: ctx,
			Log: log.FromContext(ctx).WithValues("reconfigure_for_test", testCtx.DefaultNamespace),
		}
	)

	var (
		configMapObj        *corev1.ConfigMap
		configConstraintObj *dbaasv1alpha1.ConfigConstraint
		clusterDefObj       *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj   *dbaasv1alpha1.ClusterVersion
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
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ClusterVersionSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.ClusterDefinitionSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

		// Add any setup steps that needs to be executed before each test
		ctrl, mockClient = func() (*gomock.Controller, *mock_client.MockClient) {
			ctrl := gomock.NewController(GinkgoT())
			client := mock_client.NewMockClient(ctrl)
			return ctrl, client
		}()

		By("creating a cluster")
		configMapObj = testdbaas.CreateCustomizedObj(&testCtx,
			"resources/mysql_config_cm.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		configConstraintObj = testdbaas.CreateCustomizedObj(&testCtx,
			"resources/mysql_config_template.yaml",
			&dbaasv1alpha1.ConfigConstraint{})

		By("Create a clusterDefinition obj")
		clusterDefObj = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
			AddComponent(testdbaas.StatefulMySQLComponent, statefulCompType).
			AddConfigTemplate(configTplName, configMapObj.Name, configConstraintObj.Name, configVolumeName, nil).
			Create(&testCtx).GetClusterDef()

		By("Create a clusterVersion obj")
		clusterVersionObj = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
			AddComponent(statefulCompType).
			Create(&testCtx).GetClusterVersion()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanEnv()

		ctrl.Finish()
	})

	Context("clusterdefinition CR test", func() {
		It("Should success without error", func() {
			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(configMapObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cc object"),
				}, {
					object: configMapObj,
					err:    nil,
				}},
				// for cc
				client.ObjectKeyFromObject(configConstraintObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cc object"),
				}, {
					object: configConstraintObj,
					err:    nil,
				}, {
					object: availableTPL,
					err:    nil,
				}},
			}

			accessCounter := map[client.ObjectKey]int{}

			mockClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(nil)

			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(nil)

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					tests, ok := testDatas[key]
					if !ok {
						return cfgcore.MakeError("not exist")
					}
					index := accessCounter[key]
					tt := tests[index]
					if tt.err == nil {
						// mock data
						testutil.SetGetReturnedObject(obj, tt.object)
					}
					if index < len(tests)-1 {
						accessCounter[key]++
					}
					return tt.err
				}).AnyTimes()

			_, err := CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCDLabelsByConfiguration(mockClient, reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCDConfigMapFinalizer(mockClient, reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())

			err = DeleteCDConfigMapFinalizer(mockClient, reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
		})
	})

	Context("clusterdefinition CR test without config Constraints", func() {
		It("Should success without error", func() {
			// remove ConfigConstraintRef
			_, err := handleConfigTemplate(clusterDefObj, func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
				return true, nil
			}, func(component *dbaasv1alpha1.ClusterDefinitionComponent) error {
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
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(configMapObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cc object"),
				}, {
					object: configMapObj,
					err:    nil,
				}},
			}
			accessCounter := map[client.ObjectKey]int{}

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					tests, ok := testDatas[key]
					if !ok {
						return cfgcore.MakeError("not exist")
					}
					index := accessCounter[key]
					tt := tests[index]
					if tt.err == nil {
						// mock data
						testutil.SetGetReturnedObject(obj, tt.object)
					}
					if index < len(tests)-1 {
						accessCounter[key]++
					}
					return tt.err
				}).AnyTimes()

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			ok, err := CheckCDConfigTemplate(mockClient, reqCtx, clusterDefObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())
		})
	})

	updateAVTemplates := func() {
		var tpls []dbaasv1alpha1.ConfigTemplate
		_, err := handleConfigTemplate(clusterDefObj, func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
			tpls = templates
			return true, nil
		})
		Expect(err).Should(Succeed())

		if len(clusterVersionObj.Spec.Components) == 0 {
			return
		}

		// mock clusterVersionObj config templates
		clusterVersionObj.Spec.Components[0].ConfigTemplateRefs = tpls
	}

	Context("clusterversion CR test", func() {
		It("Should success without error", func() {
			updateAVTemplates()
			availableTPL := configConstraintObj.DeepCopy()
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(configMapObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cc object"),
				}, {
					object: configMapObj,
					err:    nil,
				}},
				// for cc
				client.ObjectKeyFromObject(configConstraintObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cc object"),
				}, {
					object: configConstraintObj,
					err:    nil,
				}, {
					object: availableTPL,
					err:    nil,
				}},
			}

			accessCounter := map[client.ObjectKey]int{}

			mockClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(nil)

			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(nil)

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					tests, ok := testDatas[key]
					if !ok {
						return cfgcore.MakeError("not exist")
					}
					index := accessCounter[key]
					tt := tests[index]
					if tt.err == nil {
						// mock data
						testutil.SetGetReturnedObject(obj, tt.object)
					}
					if index < len(tests)-1 {
						accessCounter[key]++
					}
					return tt.err
				}).AnyTimes()

			_, err := CheckCVConfigTemplate(mockClient, reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCVConfigTemplate(mockClient, reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get cc object"))

			_, err = CheckCVConfigTemplate(mockClient, reqCtx, clusterVersionObj)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCVConfigTemplate(mockClient, reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCVLabelsByConfiguration(mockClient, reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCVConfigMapFinalizer(mockClient, reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())

			err = DeleteCVConfigMapFinalizer(mockClient, reqCtx, clusterVersionObj)
			Expect(err).Should(Succeed())
		})
	})

	Context("common funcs test", func() {
		It("GetReloadOptions Should success without error", func() {
			mockTpl := dbaasv1alpha1.ConfigConstraint{
				Spec: dbaasv1alpha1.ConfigConstraintSpec{
					ReloadOptions: &dbaasv1alpha1.ReloadOptions{
						UnixSignalTrigger: &dbaasv1alpha1.UnixSignalTrigger{
							Signal:      "HUB",
							ProcessName: "for_test",
						},
					},
				},
			}
			tests := []struct {
				name    string
				tpls    []dbaasv1alpha1.ConfigTemplate
				want    *dbaasv1alpha1.ReloadOptions
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
				tpls:    []dbaasv1alpha1.ConfigTemplate{},
				want:    nil,
				wantErr: false,
			}, {
				// config templates without configConstraintObj
				name: "test",
				tpls: []dbaasv1alpha1.ConfigTemplate{
					{
						Name: "for_test",
					},
					{
						Name: "for_test2",
					},
				},
				want:    nil,
				wantErr: false,
			}, {
				// normal
				name: "test",
				tpls: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:                "for_test",
						ConfigConstraintRef: "eg_v1",
					},
				},
				want:    mockTpl.Spec.ReloadOptions,
				wantErr: false,
			}, {
				// not exist config constraint
				name: "test",
				tpls: []dbaasv1alpha1.ConfigTemplate{
					{
						Name:                "for_test",
						ConfigConstraintRef: "not_exist",
					},
				},
				want:    nil,
				wantErr: true,
			}}

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if strings.Contains(key.Name, "not_exist") {
						return cfgcore.MakeError("not exist config!")
					}
					testutil.SetGetReturnedObject(obj, &mockTpl)
					return nil
				}).
				MaxTimes(len(tests))

			for _, tt := range tests {
				got, err := GetReloadOptions(mockClient, ctx, tt.tpls)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				Expect(reflect.DeepEqual(got, tt.want)).Should(BeTrue())
			}
		})
	})
})
