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

package operations

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
	mock_client "github.com/apecloud/kubeblocks/internal/testutil/k8s/mocks"
)

var _ = Describe("Reconfigure RollingPolicy", func() {

	var (
		mockClient *mock_client.MockClient
		ctrl       *gomock.Controller
	)

	mockCfgTplObj := func(tpl dbaasv1alpha1.ConfigTemplate) (*corev1.ConfigMap, *dbaasv1alpha1.ConfigConstraint) {
		By("By assure an cm obj")
		cfgCM := testdbaas.NewCustomizedObj("operations_config/configcm.yaml",
			&corev1.ConfigMap{},
			testdbaas.WithNamespacedName(tpl.ConfigTplRef, tpl.Namespace))
		cfgTpl := testdbaas.NewCustomizedObj("operations_config/configtpl.yaml",
			&dbaasv1alpha1.ConfigConstraint{},
			testdbaas.WithNamespacedName(tpl.ConfigConstraintRef, tpl.Namespace))
		return cfgCM, cfgTpl
	}

	BeforeEach(func() {
		ctrl, mockClient = testutil.SetupK8sMock()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		ctrl.Finish()
	})

	_ = mockClient

	Context("updateCfgParams test", func() {
		It("Should success without error", func() {
			tpl := dbaasv1alpha1.ConfigTemplate{
				Name:                "for_test",
				ConfigTplRef:        "cm_obj",
				ConfigConstraintRef: "cfg_constraint_obj",
			}
			updatedCfg := dbaasv1alpha1.Configuration{
				Keys: []dbaasv1alpha1.ParameterConfig{{
					Parameters: []dbaasv1alpha1.ParameterPair{
						{
							Key:   "x1",
							Value: func() *string { v := "y1"; return &v }(),
						},
						{
							Key:   "x2",
							Value: func() *string { v := "y2"; return &v }(),
						},
						{
							Key:   "server-id",
							Value: nil, // delete parameter
						}},
				}},
			}
			diffCfg := `{"mysqld":{"x1":"y1","x2":"y2"}}`

			cmObj, tplObj := mockCfgTplObj(tpl)
			mockK8sObjs := map[client.ObjectKey][]struct {
				object client.Object
				err    error
			}{
				// for cm
				client.ObjectKeyFromObject(cmObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get cm object"),
				}, {
					object: cmObj,
					err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(tplObj): {{
					object: nil,
					err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					object: tplObj,
					err:    nil,
				}},
			}
			accessCounter := map[client.ObjectKey]int{}

			mockClient.EXPECT().
				Get(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					tests, ok := mockK8sObjs[key]
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

			mockClient.EXPECT().
				Patch(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					cm, _ := obj.(*corev1.ConfigMap)
					cmObj.Data = cm.Data
					return nil
				}).AnyTimes()

			By("CM object failed.")
			// mock failed
			r := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get cm object"))

			By("TPL object failed.")
			// mock failed
			r = updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get tpl object"))

			By("update validate failed.")
			// check diff
			r = updateCfgParams(dbaasv1alpha1.Configuration{
				Keys: []dbaasv1alpha1.ParameterConfig{{
					Parameters: []dbaasv1alpha1.ParameterPair{
						{
							Key:   "innodb_autoinc_lock_mode",
							Value: func() *string { v := "100"; return &v }(), // invalid value
						},
					},
				}},
			}, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
			Expect(r.failed).Should(BeTrue())
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring(`
mysqld.innodb_autoinc_lock_mode: conflicting values 0 and 100:
    9:36
    12:18
mysqld.innodb_autoinc_lock_mode: conflicting values 1 and 100:
    9:40
    12:18
mysqld.innodb_autoinc_lock_mode: conflicting values 2 and 100:
    9:44
    12:18
`))

			By("normal update.")
			{
				oldConfig := cmObj.Data
				r := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, mockClient, "test")
				Expect(r.err).Should(Succeed())
				option := cfgcore.CfgOption{
					Type:    cfgcore.CfgTplType,
					CfgType: dbaasv1alpha1.INI,
					Log:     log.FromContext(context.Background()),
				}
				diff, err := cfgcore.CreateMergePatch(&cfgcore.K8sConfig{
					CfgKey:         client.ObjectKeyFromObject(cmObj),
					Configurations: oldConfig,
				}, &cfgcore.K8sConfig{
					CfgKey:         client.ObjectKeyFromObject(cmObj),
					Configurations: cmObj.Data,
				}, option)
				Expect(err).Should(Succeed())
				Expect(diff.IsModify).Should(BeTrue())
				Expect(diff.UpdateConfig["my.cnf"]).Should(BeEquivalentTo(diffCfg))
			}
		})
	})

})
