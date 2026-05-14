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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

// These tests pin the failure-gate contract for restore-type OpsRequest's
// ReconcileAction.
//
// Test (d) is the race contract: when the cluster is transiently Failed
// while a Restore CR is still in progress, ReconcileAction must NOT
// terminally fail the OpsRequest. (d) was RED on the single-snapshot
// cluster.Status.Phase implementation; it turns GREEN once the
// restore-aware failure gate is in place.
//
// Tests (a) (b) (c) (e) are protection tests covering existing failure /
// success contracts:
//
//   - (a) a Restore CR in terminal Failed phase must fail the OpsRequest.
//   - (b) a cluster being deleted must fail the OpsRequest.
//   - (c) all Restore CRs Completed but cluster still Failed must fail the
//     OpsRequest.
//   - (e) cluster reaching Running must succeed the OpsRequest, even if a
//     Restore CR is still in progress (cluster Running is authoritative
//     for restore success).
//
// (f) tests the error path for the restore-aware failure gate. (f.1) is
// active: an empty Restore CR list during a Failed cluster yields a
// non-terminal explicit requeue (not terminal Failed, not silent Running).
// (f.2) covers Restore CR list API error; it remains PIt because injecting
// a list error reliably requires a fake/intercepting client, which is out
// of scope for the current envtest fixture.
var _ = Describe("Restore OpsRequest ReconcileAction failure-gate race", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-race-compdef-" + randomStr
		clusterName = "test-race-cluster-" + randomStr
	)

	cleanEnv := func() {
		// Mirror the cleanup pattern used by the existing restore_test.go
		// Describe so resources from previous It don't leak into the next.
		By("clean resources")
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
	}

	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	// Helper: create a Restore CR tied to the test cluster with a given
	// phase. The label `app.kubernetes.io/instance=<cluster-name>` is the
	// **candidate** lookup convention D's implementation may use; this is
	// not yet a frozen contract. Alternatives include `GetCompLabels`
	// (already used by `plan.RestoreManager.GetRestoreObjectMeta`),
	// OwnerReferences, or name prefix; each has trade-offs. Whatever D
	// picks, this helper must mirror that exact choice so post-D the
	// fixture remains discoverable. If the helper and D diverge, the post-D
	// regression will fail to find the Restore CRs and the tests will
	// misbehave.
	createRestoreCRForCluster := func(clusterName, restoreName string, phase dpv1alpha1.RestorePhase) *dpv1alpha1.Restore {
		r := testdp.NewRestoreFactory(testCtx.DefaultNamespace, restoreName).
			SetLabels(map[string]string{
				constant.AppInstanceLabelKey: clusterName,
			}).
			SetBackup("any-backup-"+randomStr, testCtx.DefaultNamespace).
			Create(&testCtx).
			GetObject()
		Expect(testapps.ChangeObjStatus(&testCtx, r, func() {
			r.Status.Phase = phase
		})).Should(Succeed())
		return r
	}

	// (d) RACE TEST — RED on pre-fix code, GREEN after D.
	//
	// When cluster.Status.Phase is briefly Failed during a restore that is
	// still in progress (Restore CR not yet terminal), the single-snapshot
	// check in restore.go:129 fires and terminally marks the OpsRequest
	// Failed even though the underlying restore later completes.
	//
	// Pre-fix: ReconcileAction returns OpsFailedPhase + "restore failed".
	// Post-D: ReconcileAction returns OpsRunningPhase because the Restore CR
	// is Running and the cluster Failed is treated as transient during
	// in-progress restore.
	//
	// The assertion is intentionally strict: phase must remain Running and
	// err must be nil, not merely "not Failed".
	It("(d) race: cluster brief Failed during restore + Restore CR Running → OpsRequest Running", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-d-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		// Restore CR is still Running (preparedata or postready in progress).
		_ = createRestoreCRForCluster(clusterName, clusterName+"-restore-d", dpv1alpha1.RestorePhaseRunning)

		// Cluster transiently Failed during the in-progress restore.
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
		})).Should(Succeed())

		restoreHandler := RestoreOpsHandler{}
		phase, _, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		// Pre-fix: phase==OpsFailedPhase, err.Error()=="restore failed" → RED.
		// Post-D: phase==OpsRunningPhase, err==nil → GREEN.
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase),
			"race test: in-progress Restore CR + cluster transient Failed must NOT terminally fail OpsRequest; current line-129 violates this")
		Expect(err).ShouldNot(HaveOccurred(),
			"race test: no fatal error during transient cluster Failed window")
	})

	// (a) PROTECTION — Restore CR terminal Failed → OpsRequest Failed.
	// Pre-fix: cluster Failed (propagated from Restore Failed) triggers
	// line 129 → OpsFailedPhase + "restore failed". GREEN.
	// Post-D: D's failure gate explicitly recognizes Restore CR terminal
	// Failed as a real failure → OpsFailedPhase. GREEN.
	It("(a) protection: Restore CR terminal Failed → OpsRequest Failed", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-a-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		_ = createRestoreCRForCluster(clusterName, clusterName+"-restore-a", dpv1alpha1.RestorePhaseFailed)

		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
		})).Should(Succeed())

		restoreHandler := RestoreOpsHandler{}
		phase, _, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase),
			"protection (a): a Restore CR in terminal Failed phase must keep OpsRequest in Failed phase")
		Expect(err).Should(HaveOccurred(),
			"protection (a): a failed restore must surface an error")
	})

	// (b) PROTECTION — cluster deleting → OpsRequest Failed. The fixture
	// uses the finalizer dance: add a test finalizer to the cluster, Delete
	// it, and the finalizer holds the object alive so DeletionTimestamp is
	// observable. Pre-fix line 128 honors `cluster.IsDeleting()`; post-D
	// must preserve this behavior. Rewriting the Failed branch can easily
	// drop the deleting check; this test guards against that regression.
	It("(b) protection: Cluster deleting → OpsRequest Failed", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-b-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		// Move cluster phase off of Running so the Running short-circuit
		// at line 126 of restore.go doesn't fire before the IsDeleting()
		// check at line 128. In a real deletion scenario the cluster phase
		// transitions to Deleting (or away from Running) before the object
		// is fully removed.
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.DeletingClusterPhase
		})).Should(Succeed())

		// Step 1: add a test finalizer so Delete won't immediately remove
		// the object — DeletionTimestamp will be observable.
		Expect(testapps.ChangeObj(&testCtx, opsRes.Cluster, func(c *appsv1.Cluster) {
			c.Finalizers = append(c.Finalizers, "test-race-finalizer")
		})).Should(Succeed())

		// Step 2: Delete; the finalizer holds the object alive with
		// DeletionTimestamp set.
		Expect(k8sClient.Delete(testCtx.Ctx, opsRes.Cluster)).Should(Succeed())

		// Confirm DeletionTimestamp is observable to ReconcileAction.
		fetched := &appsv1.Cluster{}
		Expect(k8sClient.Get(testCtx.Ctx, client.ObjectKeyFromObject(opsRes.Cluster), fetched)).Should(Succeed())
		Expect(fetched.DeletionTimestamp.IsZero()).Should(BeFalse(),
			"protection (b): cluster must have a non-zero DeletionTimestamp before ReconcileAction is called")

		restoreHandler := RestoreOpsHandler{}
		phase, _, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase),
			"protection (b): cluster.IsDeleting() must keep OpsRequest in Failed phase pre-fix AND post-D")
		Expect(err).Should(HaveOccurred(),
			"protection (b): a deleting cluster must surface an error")

		// Cleanup: remove the test finalizer so cleanEnv can fully clear.
		Expect(testapps.ChangeObj(&testCtx, fetched, func(c *appsv1.Cluster) {
			c.Finalizers = nil
		})).Should(Succeed())
	})

	// (c) PROTECTION — all Restore CRs Completed + cluster still Failed →
	// OpsRequest Failed. Both pre-fix and post-D GREEN. Pre-fix: line 129
	// fires on cluster Failed regardless of Restore CR state. Post-D: D's
	// failure gate explicitly handles "Restore CRs done + cluster still
	// Failed" as a real terminal failure.
	It("(c) protection: all Restore CRs Completed + cluster still Failed → OpsRequest Failed", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-c-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		_ = createRestoreCRForCluster(clusterName, clusterName+"-restore-c-preparedata", dpv1alpha1.RestorePhaseCompleted)
		_ = createRestoreCRForCluster(clusterName, clusterName+"-restore-c-postready", dpv1alpha1.RestorePhaseCompleted)

		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
		})).Should(Succeed())

		restoreHandler := RestoreOpsHandler{}
		phase, _, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		Expect(phase).Should(Equal(opsv1alpha1.OpsFailedPhase),
			"protection (c): all Restore CRs Completed but cluster still Failed must still terminally fail OpsRequest")
		Expect(err).Should(HaveOccurred(),
			"protection (c): persistent cluster Failed after restore completion must surface an error")
	})

	// (e) PROTECTION — success contract preservation. Cluster Running +
	// Restore CR postready still Running → OpsRequest Succeed. Both pre-fix
	// (line 127 cluster Running short-circuit) and post-D must keep this.
	// This protects against D being misread as "OpsRequest can only succeed
	// after all Restore CRs are Completed" — which would change the contract.
	It("(e) success preservation: cluster Running + Restore CR postready still Running → OpsRequest Succeed", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-e-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		// postready Restore CR still Running, cluster already Running.
		_ = createRestoreCRForCluster(clusterName, clusterName+"-restore-e-postready", dpv1alpha1.RestorePhaseRunning)

		// initOperationsResources defaults cluster to Running; explicit
		// reaffirm here for clarity.
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.RunningClusterPhase
		})).Should(Succeed())

		restoreHandler := RestoreOpsHandler{}
		phase, _, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		Expect(phase).Should(Equal(opsv1alpha1.OpsSucceedPhase),
			"success preservation (e): cluster Running must keep OpsRequest succeed contract, even if a Restore CR is still Running")
		Expect(err).ShouldNot(HaveOccurred(),
			"success preservation (e): cluster Running success path must not return an error")
	})

	// (f) ERROR-PATH tests for the restore-aware failure gate. The semantics
	// split into two cases:
	//
	//   (f.1) Empty Restore CR list while cluster.Status.Phase == Failed.
	//         This can be a normal short window before Restore CRs are
	//         created. The failure gate returns (OpsRunningPhase, 30s
	//         requeueAfter, nil err) + log.Info — a bounded, non-terminal,
	//         explicit retry that is neither silent Running nor terminal
	//         Failed.
	//
	//   (f.2) Restore CR list API error from the K8s client. The failure
	//         gate returns (OpsRunningPhase, 0, err) so controller-runtime
	//         re-queues loudly. Injecting a list error reliably needs a
	//         fake/intercepting client which is out of scope for the
	//         envtest fixture here, so this remains a documented gap.
	It("(f.1) empty Restore CR list + cluster Failed → Running + 30s requeueAfter + nil err (post-D)", func() {
		opsRes, _, _ := initOperationsResources(compDefName, clusterName)
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

		opsRes.OpsRequest = createRestoreOpsObj(clusterName, "restore-ops-f1-"+randomStr, "any-backup-name")
		opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsRunningPhase

		// No Restore CR is created — simulate the pre-create window in which
		// cluster.Status.Phase has reached Failed but the Restore CR(s) have
		// not yet been created by the upstream restore code path.
		Expect(testapps.ChangeObjStatus(&testCtx, opsRes.Cluster, func() {
			opsRes.Cluster.Status.Phase = appsv1.FailedClusterPhase
		})).Should(Succeed())

		restoreHandler := RestoreOpsHandler{}
		phase, requeueAfter, err := restoreHandler.ReconcileAction(reqCtx, k8sClient, opsRes)

		// (f.1) post-D contract: non-terminal phase + bounded requeue + no
		// error (quiet retry with log). NOT terminal Failed, NOT silent
		// Running.
		Expect(phase).Should(Equal(opsv1alpha1.OpsRunningPhase),
			"(f.1): empty Restore CR list during cluster Failed must keep OpsRequest non-terminal")
		Expect(requeueAfter).Should(Equal(30*time.Second),
			"(f.1): empty Restore CR list must trigger bounded requeue (30s)")
		Expect(err).ShouldNot(HaveOccurred(),
			"(f.1): empty Restore CR list is not an error — it is a pre-create window")
	})
	PIt("(f.2) Restore CR list API error + cluster Failed → Running + err (post-D)", func() {
		// TODO post-D: inject List error via fake client wrapper;
		//       assert phase==OpsRunningPhase, requeueAfter==0, err!=nil
	})
})
