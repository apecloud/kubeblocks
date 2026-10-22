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
	"encoding/json"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Initial immutable parameters", func() {
	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	initialInputs := func() *parametersv1alpha1.ParameterInputs {
		return &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("1000")},
		}
	}
	parameterKey := func() client.ObjectKey {
		return client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GenerateComponentConfigurationName(clusterName, defaultCompName),
		}
	}
	configKey := func() client.ObjectKey {
		return client.ObjectKey{
			Namespace: testCtx.DefaultNamespace,
			Name:      core.GetComponentCfgName(clusterName, defaultCompName, configSpecName),
		}
	}
	createInitial := func() (*appsv1.Component, string) {
		_, _, comp, _, its := mockReconcileResourceWithInitial(initialInputs(), func(_ *corev1.ConfigMap, _ *appsv1.ComponentDefinition) {
			key := client.ObjectKey{Name: paramsDefName}
			Eventually(testapps.GetAndChangeObj(&testCtx, key, func(pd *parametersv1alpha1.ParametersDefinition) {
				pd.Spec.ImmutableParameters = []string{"max_connections"}
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pd *parametersv1alpha1.ParametersDefinition) {
				g.Expect(pd.Spec.ImmutableParameters).Should(ConsistOf("max_connections"))
			})).Should(Succeed())
		})
		Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
			g.Expect(cp.Spec.Initial).Should(Equal(initialInputs()))
			g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
		})).Should(Succeed())
		waitRenderedConfigHash(comp.Namespace, clusterName, defaultCompName, configSpecName, "max_connections=1000")
		return comp, its.Name
	}

	It("accepts an initial immutable value and preserves it across rerender and ConfigMap recreation", func() {
		comp, _ := createInitial()
		By("rerender the same initial inputs for successive component generations")
		for _, replicas := range []int32{2, 3} {
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(comp), func(obj *appsv1.Component) {
				obj.Spec.Replicas = replicas
			})).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(comp), comp)).Should(Succeed())
			generation := strconv.FormatInt(comp.Generation, 10)
			Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Annotations[constant.ParametersAppliedComponentGenerationKey]).Should(Equal(generation))
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
			})).Should(Succeed())
		}

		By("recreate a deleted managed ConfigMap with the same initial immutable value")
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, configKey(), cm)).Should(Succeed())
		oldUID := cm.UID
		Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())
		By("complete ConfigMap removal to exercise recovery from a missing rendered resource")
		Eventually(testapps.GetAndChangeObj(&testCtx, configKey(), func(deleting *corev1.ConfigMap) {
			deleting.Finalizers = nil
		})).Should(Succeed())
		Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, recreated *corev1.ConfigMap) {
			g.Expect(recreated.UID).ShouldNot(Equal(oldUID))
			g.Expect(recreated.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
		})).Should(Succeed())
	})

	It("preserves initial immutable values when desired updates another parameter in the same file", func() {
		comp, itsName := createInitial()
		for _, value := range []string{"ON", "OFF"} {
			Eventually(testapps.GetAndChangeObj(&testCtx, parameterKey(), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{"gtid_mode": cfgutil.ToPointer(value)},
				}
			})).Should(Succeed())
			hash := waitRenderedConfigHash(comp.Namespace, clusterName, defaultCompName, configSpecName,
				"max_connections=1000", "gtid_mode="+value)
			mockReconfigureDone(comp.Namespace, itsName, configSpecName, hash)
			Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.ObservedGeneration).Should(Equal(cp.Generation))
				g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CFinishedPhase))
			})).Should(Succeed())
		}
	})

	It("rejects desired set, remove, and same-value writes to an initial immutable parameter", func() {
		createInitial()
		for _, desired := range []*parametersv1alpha1.ParameterInputs{
			{Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("2000")}},
			{Updates: []parametersv1alpha1.ParameterUpdate{{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "max_connections"}}},
			{Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("1000")}},
		} {
			Eventually(testapps.GetAndChangeObj(&testCtx, parameterKey(), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = desired
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, parameterKey(), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.ObservedGeneration).Should(Equal(cp.Generation))
				g.Expect(cp.Status.Phase).Should(Equal(parametersv1alpha1.CMergeFailedPhase))
				g.Expect(cp.Status.Message).Should(ContainSubstring("max_connections"))
				g.Expect(cp.Status.Message).Should(ContainSubstring("immutable"))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, configKey(), func(g Gomega, cm *corev1.ConfigMap) {
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).Should(ContainSubstring("max_connections=1000"))
				g.Expect(cm.Data[testparameters.MysqlConfigFile]).ShouldNot(ContainSubstring("max_connections=2000"))
			})).Should(Succeed())
		}
	})

	It("rejects adding, changing, or removing Initial after ComponentParameter creation at the API", func() {
		for _, tc := range []struct {
			name    string
			initial *parametersv1alpha1.ParameterInputs
			next    *parametersv1alpha1.ParameterInputs
		}{
			{name: "add", next: initialInputs()},
			{name: "add-empty", next: &parametersv1alpha1.ParameterInputs{}},
			{name: "change", initial: initialInputs(), next: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"max_connections": cfgutil.ToPointer("2000")},
			}},
			{name: "remove", initial: initialInputs()},
			{name: "clear", initial: initialInputs(), next: &parametersv1alpha1.ParameterInputs{}},
		} {
			cp := &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{Name: "initial-validation-" + tc.name, Namespace: testCtx.DefaultNamespace},
				Spec: parametersv1alpha1.ComponentParameterSpec{
					ComponentName: "unbound-component", Initial: tc.initial,
				},
			}
			Expect(k8sClient.Create(ctx, cp)).Should(Succeed())
			patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/spec/initial", "value": tc.next}})
			Expect(err).ShouldNot(HaveOccurred())
			err = k8sClient.Patch(ctx, cp, client.RawPatch(types.JSONPatchType, patch))
			Expect(apierrors.IsInvalid(err)).Should(BeTrue(), "Initial %s must be rejected by API validation, got: %v", tc.name, err)
		}
	})

	It("rejects removing spec instead of updating Initial", func() {
		cp := &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{Name: "initial-validation-spec", Namespace: testCtx.DefaultNamespace},
			Spec: parametersv1alpha1.ComponentParameterSpec{
				ComponentName: "unbound-component", Initial: initialInputs(),
			},
		}
		Expect(k8sClient.Create(ctx, cp)).Should(Succeed())
		err := k8sClient.Patch(ctx, cp, client.RawPatch(types.MergePatchType, []byte(`{"spec":null}`)))
		Expect(apierrors.IsInvalid(err)).Should(BeTrue(), "removing spec must not bypass Initial validation, got: %v", err)
	})
})
