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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

type backupCacheLagClient struct {
	client.Client
	key        client.ObjectKey
	missesLeft int
}

type backupReadErrorClient struct {
	client.Reader
	err error
}

type backupReplaceAfterListReader struct {
	client.Reader
	observed    *dpv1alpha1.Backup
	replacement *dpv1alpha1.Backup
}

func (c *backupReadErrorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*dpv1alpha1.Backup); ok {
		return c.err
	}
	return c.Reader.Get(ctx, key, obj, opts...)
}

func (c *backupCacheLagClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*dpv1alpha1.Backup); ok && key == c.key && c.missesLeft > 0 {
		c.missesLeft--
		return apierrors.NewNotFound(schema.GroupResource{Group: dpv1alpha1.GroupVersion.Group, Resource: "backups"}, key.Name)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (r *backupReplaceAfterListReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if backups, ok := list.(*dpv1alpha1.BackupList); ok {
		backups.Items = []dpv1alpha1.Backup{*r.observed.DeepCopy()}
		return nil
	}
	return r.Reader.List(ctx, list, opts...)
}

func (r *backupReplaceAfterListReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if backup, ok := obj.(*dpv1alpha1.Backup); ok && key == client.ObjectKeyFromObject(r.replacement) {
		r.replacement.DeepCopyInto(backup)
		return nil
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

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
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
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
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "backup-policy-",
					Namespace:    testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
						testCtx.TestObjLabelKey:      "true",
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					Target: &dpv1alpha1.BackupTarget{
						PodSelector: &dpv1alpha1.PodSelector{
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
								constant.AppInstanceLabelKey: opsRes.Cluster.Name,
							}},
						},
					},
					BackupMethods: []dpv1alpha1.BackupMethod{{
						Name:            "snapshot",
						SnapshotVolumes: func() *bool { v := true; return &v }(),
					}},
				},
			}
			Expect(k8sClient.Create(reqCtx.Ctx, policy)).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, policy, func() {
				policy.Status.Phase = dpv1alpha1.AvailablePhase
			})).Should(Succeed())
			opsRes.OpsRequest.Spec.Backup = &opsv1alpha1.Backup{
				BackupPolicyName: policy.Name,
				BackupMethod:     "snapshot",
			}
			Expect(k8sClient.Update(reqCtx.Ctx, opsRes.OpsRequest)).Should(Succeed())
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock backup OpsRequest is Running")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("test backup action and reconcile function")
			bHandler := BackupOpsHandler{}
			Expect(bHandler.Action(reqCtx, k8sClient, opsRes)).Should(Succeed())

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
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
						dptypes.ClusterUIDLabelKey:   string(opsRes.Cluster.UID),
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

		It("keeps DataProtection input validation errors outside the Ops fatal state machine", func() {
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

			By("keep a missing backup policy retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: "missing"}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep an unknown backup method retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupMethod: "not-exist-method"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())
			Expect(err.Error()).Should(ContainSubstring("backup method not-exist-method is not supported"))

			By("keep an invalid retention period retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{RetentionPeriod: "not-a-duration"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep a missing parent backup retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: "not-exist-parent-backup"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep a failed parent backup retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: failedParentBackup.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep a cross-cluster parent backup retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: otherClusterParentBackup.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep a cross-cluster explicit backup policy retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: otherClusterPolicy.Name}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("expect retryable error when no default backup policy exists yet for the cluster")
			emptyClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			_, err = getDefaultBackupPolicy(reqCtx, emptyClient, opsRes.Cluster, "")
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found any default backup policy"))
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep multiple default backup policies retryable")
			policy2 := policy.DeepCopy()
			policy2.Name = "default-policy-2"
			policy2.ResourceVersion = ""
			multiPolicyClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy, policy2).Build()
			_, err = getDefaultBackupPolicy(reqCtx, multiPolicyClient, opsRes.Cluster, "")
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())
		})

		It("does not turn backup policy status into an Ops terminal decision", func() {
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

			By("keep an explicitly referenced unavailable policy retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: pendingPolicy.Name, BackupMethod: "snapshot"}
			_, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("keep an implicitly resolved unavailable policy retryable")
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupMethod: "snapshot"}
			_, err = buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())

			By("leave an unknown method retryable regardless of policy status")
			availablePolicy := pendingPolicy.DeepCopy()
			availablePolicy.ResourceVersion = ""
			availablePolicy.Status.Phase = dpv1alpha1.AvailablePhase
			availableClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(availablePolicy).Build()
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupPolicyName: availablePolicy.Name, BackupMethod: "bogus-method"}
			_, err = buildBackup(reqCtx, availableClient, ops, opsRes.Cluster)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeFalse())
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

		It("rejects an incremental parent from a recreated same-name cluster", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
			staleParent := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "stale-parent",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
						dptypes.ClusterUIDLabelKey:   "deleted-cluster-uid",
					},
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy, staleParent).Build()
			ops := createBackupOpsObj(clusterName, "backup-stale-parent-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{ParentBackupName: staleParent.Name}

			backup, err := buildBackup(reqCtx, fakeClient, ops, opsRes.Cluster)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("cluster UID"))
			Expect(backup).Should(BeNil())
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

		It("adopts an implicit legacy backup during Action re-entry", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
			}
			ops := createBackupOpsObj(clusterName, "backup-legacy-action-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			legacyBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716060000",
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID("legacy-action-backup-uid"),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Spec: dpv1alpha1.BackupSpec{
					BackupPolicyName: policy.Name,
					BackupMethod:     "snapshot",
					DeletionPolicy:   dpv1alpha1.BackupDeletionPolicyDelete,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, legacyBackup).Build()
			opsRes.APIReader = fakeClient

			Expect(BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)).Should(Succeed())
			backups := &dpv1alpha1.BackupList{}
			Expect(fakeClient.List(reqCtx.Ctx, backups)).Should(Succeed())
			Expect(backups.Items).Should(HaveLen(1))
			Expect(backups.Items[0].UID).Should(Equal(legacyBackup.UID))
		})

		It("does not create a modern backup when legacy adoption cannot be confirmed", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
			}
			ops := createBackupOpsObj(clusterName, "backup-legacy-unconfirmed-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			legacyBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716060030",
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID("legacy-unconfirmed-backup-uid"),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Spec: dpv1alpha1.BackupSpec{
					BackupPolicyName: policy.Name,
					BackupMethod:     "snapshot",
					DeletionPolicy:   dpv1alpha1.BackupDeletionPolicyDelete,
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, legacyBackup).Build()
			opsRes.APIReader = &backupReadErrorClient{
				Reader: fakeClient,
				err: apierrors.NewNotFound(
					schema.GroupResource{Group: dpv1alpha1.GroupVersion.Group, Resource: "backups"},
					legacyBackup.Name),
			}

			err := BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			backups := &dpv1alpha1.BackupList{}
			Expect(fakeClient.List(reqCtx.Ctx, backups)).Should(Succeed())
			Expect(backups.Items).Should(HaveLen(1))
			Expect(backups.Items[0].UID).Should(Equal(legacyBackup.UID))
		})

		It("reconciles an implicit legacy backup created before controller upgrade", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-legacy-reconcile-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			legacyBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716060100",
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID("legacy-reconcile-backup-uid"),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(legacyBackup).Build()
			opsRes.APIReader = fakeClient

			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, fakeClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("adopts an explicitly named legacy backup", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-explicit-legacy-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupName: "explicit-legacy-backup"}
			opsRes.OpsRequest = ops
			legacyBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:              ops.Spec.Backup.BackupName,
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID("explicit-legacy-backup-uid"),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(legacyBackup).Build()
			opsRes.APIReader = fakeClient

			Expect(BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)).Should(Succeed())
			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, fakeClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("fails closed when multiple current legacy backups match", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-legacy-multiple-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			legacyBackup := func(name, uid string) *dpv1alpha1.Backup {
				return &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
					Name:              name,
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID(uid),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				}}
			}
			first := legacyBackup("backup-"+ops.Namespace+"-"+opsRes.Cluster.Name+"-20260716060200", "legacy-first")
			second := legacyBackup("backup-"+ops.Namespace+"-"+opsRes.Cluster.Name+"-20260716060300", "legacy-second")
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(first, second).Build()
			opsRes.APIReader = fakeClient

			err := BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("multiple legacy backups"))
		})

		It("fails closed on a legacy candidate with a partial ownership protocol", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-legacy-partial-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			legacyBackup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
				Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716060400",
				Namespace:         opsRes.Cluster.Namespace,
				UID:               types.UID("legacy-partial-backup-uid"),
				CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
				Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				Annotations: map[string]string{
					constant.OpsRequestUIDAnnotationKey: string(ops.UID),
				},
			}}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(legacyBackup).Build()
			opsRes.APIReader = fakeClient

			err := BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("ownership protocol"))
		})

		It("fails closed on a current candidate with a non-legacy generated name", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-legacy-bad-name-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			badNameBackup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
				Name:              "backup-not-a-legacy-timestamp",
				Namespace:         opsRes.Cluster.Namespace,
				UID:               types.UID("legacy-bad-name-backup-uid"),
				CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
				Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
			}}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(badNameBackup).Build()
			opsRes.APIReader = fakeClient

			err := BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("legacy generated name shape"))
		})

		It("ignores a legacy backup that predates the current same-name OpsRequest", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "explicit-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
					},
				},
				Spec: dpv1alpha1.BackupPolicySpec{BackupMethods: []dpv1alpha1.BackupMethod{{
					Name:            "snapshot",
					SnapshotVolumes: func() *bool { v := true; return &v }(),
				}}},
				Status: dpv1alpha1.BackupPolicyStatus{Phase: dpv1alpha1.AvailablePhase},
			}
			ops := createBackupOpsObj(clusterName, "backup-legacy-stale-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{
				BackupPolicyName: policy.Name,
				BackupMethod:     "snapshot",
			}
			opsRes.OpsRequest = ops
			staleBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716050000",
					Namespace:         opsRes.Cluster.Namespace,
					UID:               types.UID("stale-legacy-backup-uid"),
					CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(-time.Second)),
					Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Spec: dpv1alpha1.BackupSpec{
					BackupPolicyName: policy.Name,
					BackupMethod:     "snapshot",
				},
			}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy, staleBackup).Build()
			opsRes.APIReader = fakeClient

			Expect(BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)).Should(Succeed())
			modernIntent, err := normalizedBackupIntent(ops)
			Expect(err).ShouldNot(HaveOccurred())
			modernBackup := &dpv1alpha1.Backup{}
			Expect(fakeClient.Get(reqCtx.Ctx, client.ObjectKey{
				Namespace: opsRes.Cluster.Namespace,
				Name:      modernIntent.BackupName,
			}, modernBackup)).Should(Succeed())
		})

		It("retries when a legacy backup changes UID between List and direct Get", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-legacy-replaced-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			observed := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
				Name:              "backup-" + ops.Namespace + "-" + opsRes.Cluster.Name + "-20260716060500",
				Namespace:         opsRes.Cluster.Namespace,
				UID:               types.UID("legacy-observed-backup-uid"),
				CreationTimestamp: metav1.NewTime(ops.CreationTimestamp.Add(time.Second)),
				Labels:            getBackupLabels(opsRes.Cluster.Name, ops.Name),
			}}
			replacement := observed.DeepCopy()
			replacement.UID = types.UID("legacy-replacement-backup-uid")
			baseReader := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			reader := &backupReplaceAfterListReader{
				Reader:      baseReader,
				observed:    observed,
				replacement: replacement,
			}
			opsRes.APIReader = reader

			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, baseReader, opsRes)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("changed identity"))
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
		})

		It("reconciles only the backup owned by the current OpsRequest UID", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
			ops := createBackupOpsObj(clusterName, "backup-reconcile-uid-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{BackupName: "zz-current-backup"}
			opsRes.OpsRequest = ops

			currentBackup, err := buildBackup(reqCtx,
				fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy).Build(),
				ops, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())
			currentBackup.Status.Phase = dpv1alpha1.BackupPhaseRunning

			staleBackup := currentBackup.DeepCopy()
			staleBackup.Name = "aa-stale-backup"
			staleBackup.Annotations[constant.OpsRequestUIDAnnotationKey] = "stale-ops-uid"
			staleBackup.Status.Phase = dpv1alpha1.BackupPhaseCompleted

			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(policy, staleBackup, currentBackup).Build()
			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, fakeClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
		})

		It("reads through a cache miss while the created Backup is visible from the API server", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-cache-lag-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			intent, err := normalizedBackupIntent(ops)
			Expect(err).ShouldNot(HaveOccurred())
			backup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      intent.BackupName,
					Namespace: opsRes.Cluster.Namespace,
					Labels:    getBackupLabels(opsRes.Cluster.Name, ops.Name),
				},
				Spec:   dpv1alpha1.BackupSpec{DeletionPolicy: dpv1alpha1.BackupDeletionPolicyDelete},
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseRunning},
			}
			backup.Annotations, err = getBackupAnnotations(ops, backup.Spec)
			Expect(err).ShouldNot(HaveOccurred())
			baseClient := fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(backup).Build()
			laggedClient := &backupCacheLagClient{
				Client:     baseClient,
				key:        client.ObjectKeyFromObject(backup),
				missesLeft: 1,
			}
			opsRes.APIReader = baseClient

			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, laggedClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))

			phase, _, err = BackupOpsHandler{}.ReconcileAction(reqCtx, laggedClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
		})

		It("fails when both the cache and API server confirm the Backup is absent", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-missing-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			missingClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			opsRes.APIReader = missingClient

			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, missingClient, opsRes)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("keeps the OpsRequest running when the APIReader cannot confirm a cache miss", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			ops := createBackupOpsObj(clusterName, "backup-reader-error-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops
			missingClient := fake.NewClientBuilder().WithScheme(fakeScheme).Build()
			readerErr := apierrors.NewServiceUnavailable("APIReader unavailable")
			opsRes.APIReader = &backupReadErrorClient{Reader: missingClient, err: readerErr}

			phase, _, err := BackupOpsHandler{}.ReconcileAction(reqCtx, missingClient, opsRes)
			Expect(err).Should(MatchError(readerErr))
			Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
		})

		It("accepts re-entry after API defaulting and dynamic default method changes", func() {
			fakeScheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(fakeScheme)).Should(Succeed())
			policy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-policy",
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						constant.AppInstanceLabelKey: opsRes.Cluster.Name,
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
			ops := createBackupOpsObj(clusterName, "backup-defaulted-reentry-"+randomStr)
			ops.Spec.Backup = &opsv1alpha1.Backup{}
			opsRes.OpsRequest = ops

			createdBackup, err := buildBackup(reqCtx,
				fake.NewClientBuilder().WithScheme(fakeScheme).WithObjects(policy).Build(),
				ops, opsRes.Cluster)
			Expect(err).ShouldNot(HaveOccurred())
			createdBackup.Spec.DeletionPolicy = dpv1alpha1.BackupDeletionPolicyDelete

			changedPolicy := policy.DeepCopy()
			changedPolicy.Spec.BackupMethods = []dpv1alpha1.BackupMethod{{
				Name:            "new-snapshot",
				SnapshotVolumes: func() *bool { v := true; return &v }(),
			}}
			fakeClient := fake.NewClientBuilder().WithScheme(fakeScheme).
				WithObjects(changedPolicy, createdBackup).Build()

			Expect(BackupOpsHandler{}.Action(reqCtx, fakeClient, opsRes)).Should(Succeed())
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
			Expect(err.Error()).Should(ContainSubstring("backup method snapshot is not supported"))

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
