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

package dataprotection

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("BackupPolicy Controller test", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)

		// namespaced
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("create a backup policy", func() {
		It("test backup policy without setting backoffLimit", func() {
			By("creating backupPolicy without setting backoffLimit")
			bp := testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
				backupPolicy.Spec.BackoffLimit = nil
			})
			By("expect its backoffLimit should be set to the value of DefaultBackOffLimit")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(bp),
				func(g Gomega, bp *dpv1alpha1.BackupPolicy) {
					g.Expect(*bp.Spec.BackoffLimit).Should(Equal(dptypes.DefaultBackOffLimit))
				})).Should(Succeed())
		})

		It("backup policy should be available for target", func() {
			By("creating actionSet used by backup policy")
			as := testdp.NewFakeActionSet(&testCtx, nil)
			Expect(as).ShouldNot(BeNil())

			By("creating backupPolicy and its status should be available")
			bp := testdp.NewFakeBackupPolicy(&testCtx, nil)
			Expect(bp).ShouldNot(BeNil())
		})

		It("test backup policy with targets", func() {
			By("creating actionSet used by backup policy")
			as := testdp.NewFakeActionSet(&testCtx, nil)
			Expect(as).ShouldNot(BeNil())

			By("creating backupPolicy")
			targetName := "test"
			podSelector := &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppInstanceLabelKey: testdp.ClusterName,
					},
				},
			}
			// duplicated target name
			bp := testdp.NewFakeBackupPolicy(&testCtx, func(backupPolicy *dpv1alpha1.BackupPolicy) {
				backupPolicy.Spec.Targets = []dpv1alpha1.BackupTarget{
					{Name: targetName, PodSelector: podSelector},
					{Name: targetName, PodSelector: podSelector},
				}
				backupPolicy.Spec.Target = nil
			}, true)
			By("expect status of backupPolicy to unavailable")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(bp),
				func(g Gomega, bp *dpv1alpha1.BackupPolicy) {
					g.Expect(bp.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			By("expect status of backupPolicy to available")
			Expect(testapps.ChangeObj(&testCtx, bp, func(policy *dpv1alpha1.BackupPolicy) {
				policy.Spec.Targets[0].Name = "test-0"
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(bp),
				func(g Gomega, bp *dpv1alpha1.BackupPolicy) {
					g.Expect(bp.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})

	Context("CRD schema validation for optional name patterns (#10232)", func() {
		It("accepts empty string for spec.backupRepoName", func() {
			By("creating a BackupPolicy with backupRepoName set to empty string")
			emptyRepo := ""
			bp := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bp-empty-repo",
					Namespace: testCtx.DefaultNamespace,
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupRepoName: &emptyRepo,
					Target: &dpv1alpha1.BackupTarget{
						PodSelector: &dpv1alpha1.PodSelector{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									constant.AppInstanceLabelKey: testdp.ClusterName,
								},
							},
						},
					},
					BackupMethods: []dpv1alpha1.BackupMethod{
						{
							Name:            testdp.BackupMethodName,
							SnapshotVolumes: ptr.To(false),
							ActionSetName:   testdp.ActionSetName,
						},
					},
				},
			}
			Expect(testCtx.Cli.Create(testCtx.Ctx, bp)).Should(Succeed())
		})

		It("accepts empty string for spec.backupMethods[].compatibleMethod", func() {
			By("creating a BackupPolicy with compatibleMethod literally serialized as empty string")
			// CompatibleMethod has json:"compatibleMethod,omitempty" so a typed
			// client would strip the empty value before it reaches the API server.
			// Use an unstructured resource to send the field on the wire and
			// exercise the OpenAPI pattern validator.
			bp := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": dpv1alpha1.GroupVersion.String(),
					"kind":       "BackupPolicy",
					"metadata": map[string]interface{}{
						"name":      "bp-empty-compatible",
						"namespace": testCtx.DefaultNamespace,
					},
					"spec": map[string]interface{}{
						"target": map[string]interface{}{
							"podSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{
									constant.AppInstanceLabelKey: testdp.ClusterName,
								},
							},
						},
						"backupMethods": []interface{}{
							map[string]interface{}{
								"name":             testdp.BackupMethodName,
								"snapshotVolumes":  false,
								"actionSetName":    testdp.ActionSetName,
								"compatibleMethod": "",
							},
						},
					},
				},
			}
			Expect(testCtx.Cli.Create(testCtx.Ctx, bp)).Should(Succeed())
		})
	})
})
