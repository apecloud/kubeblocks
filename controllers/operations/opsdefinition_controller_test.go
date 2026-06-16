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

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("OpsDefinition Controller", func() {

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// resources should be released in following order
		testapps.ClearResources(&testCtx, intctrlutil.OpsDefinitionSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("Test OpsDefinition", func() {
		It("Test OpsDefinition", func() {
			opsDef := testapps.CreateCustomizedObj(&testCtx, "resources/mysql-opsdefinition-sql.yaml",
				&opsv1alpha1.OpsDefinition{}, testCtx.UseDefaultNamespace())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsDef), func(g Gomega, opsD *opsv1alpha1.OpsDefinition) {
				g.Expect(opsD.Status.Phase).Should(Equal(opsv1alpha1.AvailablePhase))
			}))
		})

		It("marks OpsDefinition unavailable when a precondition template is invalid", func() {
			scheme := newOperationsTestScheme()
			opsDef := &opsv1alpha1.OpsDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "invalid-template",
					Generation: 3,
				},
				Spec: opsv1alpha1.OpsDefinitionSpec{
					PreConditions: []opsv1alpha1.PreCondition{
						{Rule: &opsv1alpha1.Rule{Expression: "{{ if }}"}},
					},
				},
			}
			reconciler := &OpsDefinitionReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithStatusSubresource(&opsv1alpha1.OpsDefinition{}).
					WithObjects(opsDef).
					Build(),
				Scheme:   scheme,
				Recorder: record.NewFakeRecorder(1),
			}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(opsDef)})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).Should(Equal(ctrl.Result{}))

			fetched := &opsv1alpha1.OpsDefinition{}
			Expect(reconciler.Client.Get(ctx, client.ObjectKeyFromObject(opsDef), fetched)).Should(Succeed())
			Expect(fetched.Status.ObservedGeneration).Should(Equal(opsDef.Generation))
			Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.UnavailablePhase))
			Expect(fetched.Status.Message).Should(ContainSubstring("missing value for if"))
		})
	})

})
