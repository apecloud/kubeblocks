/*
Copyright ApeCloud Inc.

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
	test "github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("ConfigWrapper util test", func() {

	var (
		ctrl       *gomock.Controller
		mockClient *mock_client.MockClient

		reqCtx = intctrlutil.RequestCtx{
			Ctx: ctx,
			Log: log.FromContext(ctx).WithValues("reconfigure_for_test", testCtx.DefaultNamespace),
		}
	)

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		ctrl, mockClient = func() (*gomock.Controller, *mock_client.MockClient) {
			ctrl := gomock.NewController(GinkgoT())
			client := mock_client.NewMockClient(ctrl)
			return ctrl, client
		}()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		ctrl.Finish()
	})

	Context("clusterdefinition CR test", func() {
		It("Should success without error", func() {
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
					StsYaml:         "mysql_sts.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// clean all cr after finished
			defer func() {
				Expect(testWrapper.DeleteAllCR()).Should(Succeed())
			}()

			availableTPL := testWrapper.tpl.DeepCopy()
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(testWrapper.cm): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: testWrapper.cm,
					err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(testWrapper.tpl): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: testWrapper.tpl,
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

			_, err := CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCDLabelsWithUsingConfiguration(mockClient, reqCtx, testWrapper.cd)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCDConfigMapFinalizer(mockClient, reqCtx, testWrapper.cd)
			Expect(err).Should(Succeed())

			err = DeleteCDConfigMapFinalizer(mockClient, reqCtx, testWrapper.cd)
			Expect(err).Should(Succeed())
		})
	})

	Context("clusterdefinition CR test without config Constraints", func() {
		It("Should success without error", func() {
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
					StsYaml:         "mysql_sts.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// remove ConfigConstraintRef
			_, err := handleConfigTemplate(testWrapper.cd, func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
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

			// clean all cr after finished
			defer func() {
				Expect(testWrapper.DeleteAllCR()).Should(Succeed())
			}()

			availableTPL := testWrapper.tpl.DeepCopy()
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(testWrapper.cm): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: testWrapper.cm,
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

			_, err = CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			ok, err := CheckCDConfigTemplate(mockClient, reqCtx, testWrapper.cd)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())
		})
	})

	Context("clusterversion CR test", func() {
		It("Should success without error", func() {
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
					StsYaml:         "mysql_sts.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// clean all cr after finished
			defer func() {
				Expect(testWrapper.DeleteAllCR()).Should(Succeed())
			}()

			updateAVTemplates(testWrapper)

			availableTPL := testWrapper.tpl.DeepCopy()
			availableTPL.Status.Phase = dbaasv1alpha1.AvailablePhase

			testDatas := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(testWrapper.cm): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: testWrapper.cm,
					err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(testWrapper.tpl): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: testWrapper.tpl,
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

			_, err := CheckCVConfigTemplate(mockClient, reqCtx, testWrapper.cv)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			_, err = CheckCVConfigTemplate(mockClient, reqCtx, testWrapper.cv)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("failed to get tpl object"))

			_, err = CheckCVConfigTemplate(mockClient, reqCtx, testWrapper.cv)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("status not ready"))

			ok, err := CheckCVConfigTemplate(mockClient, reqCtx, testWrapper.cv)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			ok, err = UpdateCVLabelsWithUsingConfiguration(mockClient, reqCtx, testWrapper.cv)
			Expect(err).Should(Succeed())
			Expect(ok).Should(BeTrue())

			err = UpdateCVConfigMapFinalizer(mockClient, reqCtx, testWrapper.cv)
			Expect(err).Should(Succeed())

			err = DeleteCVConfigMapFinalizer(mockClient, reqCtx, testWrapper.cv)
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
				// config templates without configConstraint
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

func updateAVTemplates(wrapper *TestWrapper) {
	var tpls []dbaasv1alpha1.ConfigTemplate
	_, err := handleConfigTemplate(wrapper.cd, func(templates []dbaasv1alpha1.ConfigTemplate) (bool, error) {
		tpls = templates
		return true, nil
	})
	Expect(err).Should(Succeed())

	if len(wrapper.cv.Spec.Components) == 0 {
		return
	}

	// mock cv config templates
	wrapper.cv.Spec.Components[0].ConfigTemplateRefs = tpls
}
