/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	ops "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/operations"
	"github.com/apecloud/kubeblocks/pkg/parameters"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Schema-backed parameter arrays", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	It("applies the same array field through ParameterView, Ops, and ComponentParameter", func() {
		const file = "config.json"
		_, cluster, comp, _, its := mockReconcileResource(
			func(template *corev1.ConfigMap, compDef *appsv1.ComponentDefinition) {
				Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(template), func(cm *corev1.ConfigMap) {
					cm.Data = map[string]string{file: `{"peer_hosts":["peer-default"],"mode":7}`}
				})).Should(Succeed())
				definitionKey := client.ObjectKey{Name: paramsDefName}
				Eventually(testapps.GetAndChangeObj(&testCtx, definitionKey, func(def *parametersv1alpha1.ParametersDefinition) {
					def.Spec.ComponentDef = "unused-component-definition"
				})).Should(Succeed())
				jsonDefinition := testparameters.NewParametersDefinitionFactory("json-parameters").
					SetComponentDefinition(compDef.Name).
					SetTemplateName(configSpecName).
					SetConfigFile(file).
					SetFileFormatConfig(parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.JSON}).
					Schema(`#Config: { hosts?: [...string], peer_hosts?: [...string], mode?: int }`).
					Create(&testCtx).GetObject()
				Eventually(func(g Gomega) {
					_, definitions, err := parameters.ResolveCmpdParametersDefs(ctx, k8sClient, compDef)
					g.Expect(err).ShouldNot(HaveOccurred())
					g.Expect(definitions).Should(ConsistOf(HaveField("Name", jsonDefinition.Name)))
				}).Should(Succeed())
			})
		cpKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GenerateComponentConfigurationName(cluster.Name, defaultCompName)}
		cmKey := client.ObjectKey{Namespace: comp.Namespace, Name: core.GetComponentCfgName(cluster.Name, defaultCompName, configSpecName)}
		const base = `{"peer_hosts":["peer-default"],"mode":7}`
		checkRendered := func(expected string) {
			GinkgoHelper()
			var hash *string
			Eventually(testapps.CheckObj(&testCtx, cmKey, func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Data[file]).To(MatchJSON(expected))
				hash = computeTargetConfigHash(nil, cm.Data)
				g.Expect(hash).NotTo(BeNil())
			})).Should(Succeed())
			mockReconfigureDone(its.Namespace, its.Name, configSpecName, *hash)
			Eventually(testapps.CheckObj(&testCtx, cpKey, func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.Phase).To(Equal(parametersv1alpha1.CFinishedPhase))
				g.Expect(cp.Status.ObservedGeneration).To(Equal(cp.Generation))
			})).Should(Succeed())
		}
		checkRendered(base)

		view := &parametersv1alpha1.ParameterView{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: comp.Namespace, Name: "schema-array",
				Labels: map[string]string{testCtx.TestObjLabelKey: "true"},
			},
			Spec: parametersv1alpha1.ParameterViewSpec{
				ParameterRef: corev1.LocalObjectReference{Name: cpKey.Name},
				TemplateName: configSpecName,
				FileName:     file,
			},
		}
		Expect(k8sClient.Create(ctx, view)).To(Succeed())
		viewKey := client.ObjectKeyFromObject(view)
		checkView := func(expected string) {
			GinkgoHelper()
			Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, fetched *parametersv1alpha1.ParameterView) {
				g.Expect(fetched.Status.Phase).To(Equal(parametersv1alpha1.ParameterViewSyncedPhase))
				g.Expect(fetched.Spec.Content.Text).To(MatchJSON(expected))
			})).Should(Succeed())
		}
		editView := func(content string) {
			GinkgoHelper()
			Eventually(testapps.GetAndChangeObj(&testCtx, viewKey, func(fetched *parametersv1alpha1.ParameterView) {
				fetched.Spec.Content.Text = content
			})).Should(Succeed())
		}
		checkView(base)

		By("rejecting an array whose element violates the schema")
		editView(`{"hosts":[1],"peer_hosts":["peer-default"],"mode":7}`)
		Eventually(testapps.CheckObj(&testCtx, viewKey, func(g Gomega, fetched *parametersv1alpha1.ParameterView) {
			g.Expect(fetched.Status.Phase).To(Equal(parametersv1alpha1.ParameterViewInvalidPhase))
			g.Expect(fetched.Status.Message).To(ContainSubstring("hosts"))
			g.Expect(fetched.Status.Message).To(ContainSubstring("string"))
		})).Should(Succeed())
		checkRendered(base)

		By("adding an array through ParameterView")
		const assigned = `{"hosts":["db1","db2,tag"],"peer_hosts":["peer-default"],"mode":7}`
		editView(assigned)
		checkRendered(assigned)
		checkView(assigned)

		By("replacing the existing array through ParameterView")
		const replaced = `{"hosts":["db3"],"peer_hosts":["peer-default"],"mode":7}`
		editView(replaced)
		checkRendered(replaced)
		checkView(replaced)

		By("removing the array through Ops while updating a numeric parameter")
		scheme := runtime.NewScheme()
		Expect(ops.AddToScheme(scheme)).To(Succeed())
		opsClient, err := client.New(cfg, client.Options{Scheme: scheme})
		Expect(err).NotTo(HaveOccurred())
		request := &ops.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{Name: "array-remove", Namespace: comp.Namespace},
			Spec:       ops.OpsRequestSpec{ClusterName: cluster.Name, Type: ops.ReconfiguringType},
		}
		request.Spec.Reconfigures = []ops.Reconfigure{{
			ComponentOps: ops.ComponentOps{ComponentName: defaultCompName},
			Parameters:   []ops.ParameterPair{{Key: "hosts"}, {Key: "mode", Value: ptr.To("8")}},
		}}
		Expect(opsClient.Create(ctx, request)).To(Succeed())
		DeferCleanup(func() { Expect(opsClient.Delete(ctx, request)).To(Succeed()) })
		Expect(opsClient.Get(ctx, client.ObjectKeyFromObject(request), request)).To(Succeed())
		handler := operations.GetOpsManager().OpsMap[ops.ReconfiguringType].OpsHandler
		Expect(handler.Action(intctrlutil.RequestCtx{Ctx: ctx}, k8sClient, &operations.OpsResource{Cluster: cluster, OpsRequest: request})).To(Succeed())
		const removed = `{"peer_hosts":["peer-default"],"mode":8}`
		checkRendered(removed)
		checkView(removed)

		By("setting an empty array through ComponentParameter after the removal")
		Eventually(testapps.GetAndChangeObj(&testCtx, cpKey, func(cp *parametersv1alpha1.ComponentParameter) {
			cp.Spec.Desired.Updates = append(cp.Spec.Desired.Updates, parametersv1alpha1.ParameterUpdate{
				Type: parametersv1alpha1.ParameterUpdateSet, Key: "@hosts", Value: ptr.To(`[]`),
			})
		})).Should(Succeed())
		const empty = `{"hosts":[],"peer_hosts":["peer-default"],"mode":8}`
		checkRendered(empty)
		checkView(empty)

		By("removing the empty array through ParameterView")
		editView(removed)
		checkRendered(removed)
		checkView(removed)
	})
})
