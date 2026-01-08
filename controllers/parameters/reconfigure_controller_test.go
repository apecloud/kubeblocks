/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Reconfigure Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// TODO(component)
	PContext("When updating configmap", func() {
		It("Should rolling upgrade pod", func() {
			configmap, _, clusterObj, _, _ := mockReconcileResource()

			By("Check config for instance")
			var configHash string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cm.Labels[constant.AppInstanceLabelKey]).To(Equal(clusterObj.Name))
				g.Expect(cm.Labels[constant.CMConfigurationTemplateNameLabelKey]).To(Equal(configSpecName))
				g.Expect(cm.Labels[constant.CMConfigurationTypeLabelKey]).NotTo(Equal(""))
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(core.ReconfigureCreatedPhase))
				configHash = cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(configHash).NotTo(Equal(""))
				g.Expect(core.IsNotUserReconfigureOperation(cm)).To(BeTrue())
				// g.Expect(cm.Annotations[constant.KBParameterUpdateSourceAnnotationKey]).To(Equal(constant.ReconfigureManagerSource))
			}).Should(Succeed())

			By("manager changes will not change the phase of configmap.")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data["new_data"] = "###"
				core.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(core.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("recover normal update parameters")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				delete(cm.Data, "new_data")
				core.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).To(Equal(configHash))
				g.Expect(core.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("Update config, old version: " + configHash)
			updatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
				core.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check config new version")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(core.ReconfigureAutoReloadPhase))
				g.Expect(core.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
			}).Should(Succeed())

			By("invalid Update")
			invalidUpdatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-invalid-update.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = invalidUpdatedCM.Data
				core.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(core.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				// g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(cfgcore.ReconfigureNoChangeType))
			}).Should(Succeed())

			By("restart Update")
			restartUpdatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update-with-restart.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
				core.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(core.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(core.ReconfigureSimplePhase))
			}).Should(Succeed())
		})
	})
})

func Test_resolveReloadActionPolicy(t *testing.T) {
	type args struct {
		jsonPatch string
		format    *parametersv1alpha1.FileFormatConfig
		pd        *parametersv1alpha1.ParametersDefinitionSpec
	}
	tests := []struct {
		name    string
		args    args
		want    parametersv1alpha1.ReloadPolicy
		wantErr bool
	}{{
		name: "restart policy",
		args: args{
			jsonPatch: `{"static1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				StaticParameters: []string{
					"static1",
					"static2",
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
					},
				},
			},
		},
		want: parametersv1alpha1.RestartPolicy,
	}, {
		name: "restart and reload policy",
		args: args{
			jsonPatch: `{"static1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
					},
				},
				MergeReloadAndRestart: ptr.To(false),
			},
		},
		want: parametersv1alpha1.DynamicReloadAndRestartPolicy,
	}, {
		name: "hot update policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					AutoTrigger: &parametersv1alpha1.AutoTrigger{},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.AsyncDynamicReloadPolicy,
	}, {
		name: "sync reload policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
						Sync:    ptr.To(true),
					},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.SyncDynamicReloadPolicy,
	}, {
		name: "async reload policy",
		args: args{
			jsonPatch: `{"dynamic1": "value1"}`,
			format: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.JSON,
			},
			pd: &parametersv1alpha1.ParametersDefinitionSpec{
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"/bin/true"},
						Sync:    ptr.To(false),
					},
				},
				DynamicParameters: []string{
					"dynamic1",
					"dynamic2",
				},
			},
		},
		want: parametersv1alpha1.AsyncDynamicReloadPolicy,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := &ReconfigureReconciler{}
			got, err := rr.resolveReconfigurePolicy(tt.args.jsonPatch, tt.args.format, tt.args.pd)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveReloadActionPolicy(%v, %v, %v)", tt.args.jsonPatch, tt.args.format, tt.args.pd)
			}
			assert.Equalf(t, tt.want, got, "resolveReloadActionPolicy(%v, %v, %v)", tt.args.jsonPatch, tt.args.format, tt.args.pd)
		})
	}
}
