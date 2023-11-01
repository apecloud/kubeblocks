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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Reconfigure util test", func() {

	var (
		k8sMockClient *testutil.K8sClientMockHelper
		tpl           appsv1alpha1.ComponentConfigSpec
		tpl2          appsv1alpha1.ComponentConfigSpec
		updatedCfg    appsv1alpha1.ConfigurationItem
	)

	const (
		clusterName   = "mysql-test"
		componentName = "mysql"
	)

	mockCfgTplObj := func(tpl appsv1alpha1.ComponentConfigSpec) (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint, *appsv1alpha1.Configuration) {
		By("By assure an cm obj")

		cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
			&corev1.ConfigMap{},
			testapps.WithNamespacedName(core.GetComponentCfgName(clusterName, componentName, tpl.Name), testCtx.DefaultNamespace))
		cfgTpl := testapps.NewCustomizedObj("operations_config/config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{},
			testapps.WithNamespacedName(tpl.ConfigConstraintRef, tpl.Namespace))

		configuration := builder.NewConfigurationBuilder(testCtx.DefaultNamespace,
			core.GenerateComponentConfigurationName(clusterName, componentName)).
			ClusterRef(clusterName).
			Component(componentName).
			AddConfigurationItem(tpl).
			AddConfigurationItem(tpl2)
		return cfgCM, cfgTpl, configuration.GetObject()
	}

	BeforeEach(func() {
		k8sMockClient = testutil.NewK8sMockClient()
		tpl = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "for_test",
				TemplateRef: "cm_obj",
			},
			ConfigConstraintRef: "cfg_constraint_obj",
			Keys:                []string{"my.cnf"},
		}
		tpl2 = appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "for_test2",
				TemplateRef: "cm_obj",
			},
		}
		updatedCfg = appsv1alpha1.ConfigurationItem{
			Name: tpl.Name,
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
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		k8sMockClient.Finish()
	})

	Context("updateConfigConfigmapResource test", func() {
		It("Should success without error", func() {
			diffCfg := `{"mysqld":{"x1":"y1","x2":"y2"}}`

			cmObj, tplObj, configObj := mockCfgTplObj(tpl)
			tpl2Key := client.ObjectKey{
				Namespace: cmObj.Namespace,
				Name:      core.GetComponentCfgName(clusterName, componentName, tpl2.Name),
			}
			k8sMockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSequenceResult(map[client.ObjectKey][]testutil.MockGetReturned{
				// for cm
				client.ObjectKeyFromObject(cmObj): {{
					Object: nil,
					Err:    core.MakeError("failed to get cm object"),
				}, {
					Object: cmObj,
					Err:    nil,
				}},
				tpl2Key: {{
					Object: cmObj,
					Err:    nil,
				}},
				// for tpl
				client.ObjectKeyFromObject(tplObj): {{
					Object: nil,
					Err:    core.MakeError("failed to get tpl object"),
				}, {
					Object: tplObj,
					Err:    nil,
				}},
				// for configuration
				client.ObjectKeyFromObject(configObj): {{
					Object: nil,
					// Err:    core.MakeError("failed to get configuration object"),
				}, {
					Object: configObj,
				}},
			}), testutil.WithAnyTimes()))

			k8sMockClient.MockPatchMethod(testutil.WithPatchReturned(func(obj client.Object, patch client.Patch) error {
				if cm, ok := obj.(*corev1.ConfigMap); ok {
					cmObj.Data = cm.Data
				}
				return nil
			}, testutil.WithAnyTimes()))

			opsRes := &OpsResource{
				Recorder: k8sManager.GetEventRecorderFor("Reconfiguring"),
				OpsRequest: testapps.NewOpsRequestObj("reconfigure-ops-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
					clusterName, appsv1alpha1.ReconfiguringType),
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Log:      log.FromContext(ctx).WithName("Reconfiguring"),
				Recorder: opsRes.Recorder,
			}

			By("Configuration object failed.")
			// mock failed
			// r := updateConfigConfigmapResource(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test", mockUpdate)
			r := testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedCfg, clusterName, componentName)
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to found configuration of component"))

			By("CM object failed.")
			// mock failed
			// r := updateConfigConfigmapResource(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test", mockUpdate)
			r = testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedCfg, clusterName, componentName)
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get cm object"))

			By("TPL object failed.")
			// mock failed
			r = testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedCfg, clusterName, componentName)
			Expect(r.err).ShouldNot(Succeed())
			Expect(r.err.Error()).Should(ContainSubstring("failed to get tpl object"))

			By("update validate failed.")
			r = testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, appsv1alpha1.ConfigurationItem{
				Name: tpl.Name,
				Keys: []appsv1alpha1.ParameterConfig{{
					Key: "my.cnf",
					Parameters: []appsv1alpha1.ParameterPair{
						{
							Key:   "innodb_autoinc_lock_mode",
							Value: func() *string { v := "100"; return &v }(), // invalid value
						},
					},
				}},
			}, clusterName, componentName)
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
    12:18`))

			By("normal params update")
			{
				// r := updateConfigConfigmapResource(updatedCfg, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test", mockUpdate)
				r := testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedCfg, clusterName, componentName)
				Expect(r.err).Should(Succeed())
				Expect(r.noFormatFilesUpdated).Should(BeFalse())
				Expect(r.configPatch).ShouldNot(BeNil())
				diff := r.configPatch
				Expect(diff.IsModify).Should(BeTrue())
				Expect(diff.UpdateConfig["my.cnf"]).Should(BeEquivalentTo(diffCfg))
			}

			// normal params update
			By("normal file update with configSpec keys")
			{
				updatedFiles := appsv1alpha1.ConfigurationItem{
					Name: tpl2.Name,
					Keys: []appsv1alpha1.ParameterConfig{{
						Key: "my.cnf",
						FileContent: `
[mysqld]
x1=y1
z2=y2
`,
					}},
				}

				_ = updatedFiles
				// r := updateConfigConfigmapResource(updatedFiles, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test", mockUpdate)
				r := testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedFiles, clusterName, componentName)
				Expect(r.err).Should(Succeed())
			}

			// not params update, but file update
			By("normal file update with configSpec keys")
			{
				oldConfig := cmObj.Data
				newMyCfg := oldConfig["my.cnf"]
				newMyCfg += `
# for test
# not valid parameter
`
				updatedFiles := appsv1alpha1.ConfigurationItem{
					Name: tpl2.Name,
					Keys: []appsv1alpha1.ParameterConfig{{
						Key:         "my.cnf",
						FileContent: newMyCfg,
					}},
				}

				_ = updatedFiles
				// r := updateConfigConfigmapResource(updatedFiles, tpl, client.ObjectKeyFromObject(cmObj), ctx, k8sMockClient.Client(), "test", mockUpdate)
				r := testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedFiles, clusterName, componentName)
				Expect(r.err).Should(Succeed())
				Expect(r.configPatch).Should(BeNil())
				Expect(r.noFormatFilesUpdated).Should(BeTrue())
			}

			By("normal file update without configSpec keys")
			{
				updatedFiles := appsv1alpha1.ConfigurationItem{
					Name: tpl.Name,
					Keys: []appsv1alpha1.ParameterConfig{{
						Key:         "config2.txt",
						FileContent: `# for test`,
					}},
				}

				_ = updatedFiles
				r := testUpdateConfigConfigmapResource(reqCtx, k8sMockClient.Client(), opsRes, updatedFiles, clusterName, componentName)
				Expect(r.err).Should(Succeed())
				diff := r.configPatch
				Expect(diff.IsModify).Should(BeFalse())
			}
		})
	})

})
