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

package operations

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Reconfigure util test", func() {

	var (
		k8sMockClient *testutil.K8sClientMockHelper
	)

	mockCfgTplObj := func(tpl appsv1alpha1.ComponentConfigSpec) (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint) {
		By("By assure an cm obj")
		cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
			&corev1.ConfigMap{},
			testapps.WithNamespacedName(tpl.TemplateRef, tpl.Namespace))
		cfgTpl := testapps.NewCustomizedObj("operations_config/config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{},
			testapps.WithNamespacedName(tpl.ConfigConstraintRef, tpl.Namespace))
		return cfgCM, cfgTpl
	}

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	Context("updateCfgParams test", func() {
		It("Should success without error", func() {
			tpl := appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "for_test",
					TemplateRef: "cm_obj",
				},
				ConfigConstraintRef: "cfg_constraint_obj",
			}
			updatedCfg := appsv1alpha1.Configuration{
				Keys: []appsv1alpha1.ParameterConfig{{
					Key: "my.cnf",
					Parameters: []appsv1alpha1.ParameterPair{
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
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSequenceResult(map[client.ObjectKey][]testutil.MockGetReturned{
				// for cm
				client.ObjectKeyFromObject(cmObj): {{
					Object: nil,
					Err:    cfgcore.MakeError("failed to get cm object"),
				}, {
					Object: cmObj,
					Err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(tplObj): {{
					Object: nil,
					Err:    cfgcore.MakeError("failed to get tpl object"),
				}, {
					Object: tplObj,
					Err:    nil,
				}},
			}), testutil.WithAnyTimes()))

			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				cm, _ := obj.(*corev1.ConfigMap)
				cmObj.Data = cm.Data
				return nil
			}), testutil.WithAnyTimes())

			By("CM object failed.")
			// mock failed
			r := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test")
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get cm object"))

			By("TPL object failed.")
			// mock failed
			r = updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test")
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get tpl object"))

			By("update validate failed.")
			// check diff
			r = updateCfgParams(appsv1alpha1.Configuration{
				Keys: []appsv1alpha1.ParameterConfig{{
					Key: "my.cnf",
					Parameters: []appsv1alpha1.ParameterPair{
						{
							Key:   "innodb_autoinc_lock_mode",
							Value: func() *string { v := "100"; return &v }(), // invalid value
						},
					},
				}},
			}, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test")
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
				r := updateCfgParams(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test")
				Expect(r.err).Should(Succeed())
				diff, err := cfgcore.CreateMergePatch(
					cfgcore.FromConfigData(oldConfig, nil),
					cfgcore.FromConfigData(cmObj.Data, nil),
					cfgcore.CfgOption{
						Type:    cfgcore.CfgTplType,
						CfgType: appsv1alpha1.Ini,
						Log:     log.FromContext(context.Background()),
					})
				Expect(err).Should(Succeed())
				Expect(diff.IsModify).Should(BeTrue())
				Expect(diff.UpdateConfig["my.cnf"]).Should(BeEquivalentTo(diffCfg))
			}
		})
	})

})
