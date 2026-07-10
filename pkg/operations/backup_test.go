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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("Backup OpsRequest", func() {

	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr //nolint:goconst
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest for backup", func() {
		var (
			opsRes *OpsResource
			reqCtx intctrlutil.RequestCtx
		)

		BeforeEach(func() {
			By("init operations resources ")
			opsRes, _, _ = initOperationsResources(compDefName, clusterName)
			reqCtx = intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		})

		testBackupOps := func(opsRes *OpsResource) {
			By("create Backup OpsRequest")
			opsRes.OpsRequest = createBackupOpsObj(clusterName, "backup-ops-"+randomStr)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock backup OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("test backup action and reconcile function")
			bHandler := BackupOpsHandler{}
			_ = bHandler.Action(reqCtx, k8sClient, opsRes)

			By("test backup reconcile action")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		}

		It("should create a backup resource for cluster", func() {
			testBackupOps(opsRes)
		})

		It("should create a backup resource when cluster phase is Updating", func() {
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1.UpdatingClusterPhase
			})).Should(Succeed())
			testBackupOps(opsRes)
		})

		It("should failed when cluster phase is Failed", func() {
			Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
				opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
			})).Should(Succeed())

			By("create Backup OpsRequest")
			opsRes.OpsRequest = createBackupOpsObj(clusterName, "backup-ops-"+randomStr)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("expect ops phase to Failed")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("builds backup specs from default policy, retention, and parent backup", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			parentBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "parent-backup",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy, parentBackup).Build()

			ops := createBackupOpsObj(clusterName, "backup-build-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{
				BackupName:       "explicit-backup",
				RetentionPeriod:  "1d",
				DeletionPolicy:   string(dpv1alpha1.BackupDeletionPolicyRetain),
				ParentBackupName: parentBackup.Name,
			}
			backup, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(backup.Name).Should(Equal("explicit-backup"))
			Expect(backup.Spec.BackupPolicyName).Should(Equal(policy.Name))
			Expect(backup.Spec.BackupMethod).Should(Equal("snapshot"))
			Expect(backup.Spec.RetentionPeriod).Should(Equal(dpv1alpha1.RetentionPeriod("1d")))
			Expect(backup.Spec.DeletionPolicy).Should(Equal(dpv1alpha1.BackupDeletionPolicyRetain))
			Expect(backup.Spec.ParentBackupName).Should(Equal(parentBackup.Name))

			ops.Spec.Backup.RetentionPeriod = "not-a-duration"
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
		})

		It("fails fast with fatal errors on deterministic invalid inputs", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			failedParentBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failed-parent-backup",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseFailed},
			}
			otherClusterParentBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-cluster-parent-backup",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": "other-cluster",
					},
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
			}
			otherClusterPolicy := policy.DeepCopy()
			otherClusterPolicy.Name = "other-cluster-policy"
			otherClusterPolicy.ResourceVersion = ""
			otherClusterPolicy.Labels["app.kubernetes.io/instance"] = "other-cluster"
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, failedParentBackup, otherClusterParentBackup, otherClusterPolicy).Build()
			ops := createBackupOpsObj(clusterName, "backup-fatal-"+randomStr)

			By("expect fatal error when backupPolicyName refers to a nonexistent backup policy")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: "missing"}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when backupMethod is not defined in the backup policy")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupMethod: "not-exist-method"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("backup method not-exist-method is not supported"))

			By("expect fatal error when retentionPeriod is invalid")
			ops.Spec.Backup = &opsv1alpha1.Backup{RetentionPeriod: "not-a-duration"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when parentBackupName refers to a nonexistent backup")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: "not-exist-parent-backup"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when parent backup is Failed")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: failedParentBackup.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when parent backup belongs to another cluster")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: otherClusterParentBackup.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when explicit backupPolicyName belongs to another cluster")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: otherClusterPolicy.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("is not belong to cluster"))

			By("expect retryable error when no default backup policy exists yet for the cluster")
			emptyClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			_, err = getDefaultBackupPolicy(reqCtx, emptyClient, opsRes.Cluster, "")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found any default backup policy"))
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("expect fatal error when multiple default backup policies exist for the cluster")
			policy2 := policy.DeepCopy()
			policy2.Name = "default-policy-2"
			policy2.ResourceVersion = ""
			multiPolicyClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy, policy2).Build()
			_, err = getDefaultBackupPolicy(reqCtx, multiPolicyClient, opsRes.Cluster, "")
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
		})

		It("keeps retrying while the backup policy is not yet Available", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			pendingPolicy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pending-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				// the policy exists but its status has not been reconciled to Available yet.
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(pendingPolicy).Build()
			ops := createBackupOpsObj(clusterName, "backup-pending-policy-"+randomStr)

			By("expect retryable error when the policy is referenced explicitly with a valid method")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: pendingPolicy.Name, BackupMethod: "snapshot"}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("is not available yet"))
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("expect retryable error when the default policy is resolved implicitly")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupMethod: "snapshot"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("is not available yet"))
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("expect fatal error only after the policy is Available and the method is still absent")
			availablePolicy := pendingPolicy.DeepCopy()
			availablePolicy.ResourceVersion = ""
			availablePolicy.Status.Phase = dpv1alpha1.AvailablePhase
			availableClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(availablePolicy).Build()
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: availablePolicy.Name, BackupMethod: "bogus-method"}
			_, err = buildBackup(reqCtx, availableClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("backup method bogus-method is not supported"))
		})

		It("keeps retrying while parent backup is still running", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			runningParentBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "running-parent-backup",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseRunning},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, runningParentBackup).Build()
			ops := createBackupOpsObj(clusterName, "backup-parent-running-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: runningParentBackup.Name}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("is not completed"))
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())
		})

		It("uses a stable generated backup name for OpsRequest re-entry", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy).Build()
			ops := createBackupOpsObj(clusterName, "backup-generated-name-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}

			firstBackup, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(firstBackup.Name).Should(Equal("backup-" + string(ops.UID)))

			freshOps := ops.DeepCopy()
			freshOps.Spec.Backup = &opsv1alpha1.Backup{}
			secondBackup, err := buildBackup(reqCtx, fakeClient, freshOps, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(secondBackup.Name).Should(Equal(firstBackup.Name))

			opsRes.OpsRequest = freshOps
			reentryClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, firstBackup).Build()
			Expect(BackupOpsHandler{}.Action(reqCtx, reentryClient, opsRes)).Should(Succeed())
		})

		It("handles a backup name collision with an existing backup", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/instance": opsRes.Cluster.Name,
					},
					Annotations: map[string]string{
						dptypes.DefaultBackupPolicyAnnotationKey: "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			ops := createBackupOpsObj(clusterName, "backup-collision-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupName: "colliding-backup"}
			opsRes.OpsRequest = ops
			desiredBackup, err := buildBackup(reqCtx,
				fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy).Build(),
				ops, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())

			By("expect fatal error when the existing backup is not created by this OpsRequest")
			foreignBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "colliding-backup",
					Namespace: testCtx.DefaultNamespace,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, foreignBackup).Build()
			err = BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("already exists and is not created by this OpsRequest"))

			By("expect fatal error when the existing backup only matches the reusable OpsRequest name")
			staleUIDBackup := desiredBackup.DeepCopy()
			staleUIDBackup.Annotations[constant.OpsRequestUIDAnnotationKey] = "stale-ops-uid"
			fakeClient = fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, staleUIDBackup).Build()
			err = BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect fatal error when the existing backup ownership matches but the spec is stale")
			staleSpecBackup := desiredBackup.DeepCopy()
			staleSpecBackup.Spec.BackupMethod = "stale-method"
			fakeClient = fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, staleSpecBackup).Build()
			err = BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())

			By("expect success when the existing backup was created by this OpsRequest")
			ownedBackup := desiredBackup.DeepCopy()
			fakeClient = fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, ownedBackup).Build()
			Expect(BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)).Should(Succeed())
		})

		It("reports backup policy and method validation errors", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			ops := createBackupOpsObj(clusterName, "backup-errors-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: "missing", BackupMethod: "snapshot"}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(`"missing" not found`))

			ops.Spec.Backup = nil
			_, err = getDefaultBackupPolicy(reqCtx, fakeClient, opsRes.Cluster, "")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found any default backup policy"))
		})
	})
})

func createBackupOpsObj(clusterName, backupOpsName string) *opsv1alpha1.OpsRequest {
	ops := testops.NewOpsRequestObj(backupOpsName, testCtx.DefaultNamespace,
		clusterName, opsv1alpha1.BackupType)
	return testops.CreateOpsRequest(ctx, testCtx, ops)
}
