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

package dataprotection

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// These envtest specs guard the persistence-layer slice of
// handleStorageClassNotFound that the fake.Client unit tests cannot
// validate: real Status subresource Patch via API server, metav1.Time
// serialization round-trip for LastTransitionTime, and event delivery
// through a real EventRecorder. The matrix of reason/message values is
// already covered by the fake.Client tests in
// clusterrestore_controller_test.go; specs here intentionally only
// exercise the bounded-window escalation outcome (Phase=Failed terminal)
// so the BeforeSuite-registered ClusterRestoreReconciler stays quiet
// (early return on terminal phase) and does not race with the spec.
var _ = Describe("ClusterRestore StorageClass missing envtest", func() {
	const (
		envtestStorageClassName  = "envtest-missing-sc"
		envtestTargetClusterName = "envtest-target-cluster"
		envtestTargetClusterUID  = types.UID("envtest-target-cluster-uid")
		envtestSourceBackupName  = "envtest-backup"
		envtestSourceBackupNS    = "envtest-backup-ns"
		envtestPVCName           = "data-envtest-target-cluster-mysql-0"
		envtestEventDrainTimeout = 2 * time.Second
	)

	var (
		nsName     string
		recorder   *record.FakeRecorder
		reconciler *ClusterRestoreReconciler
	)

	newClusterRestore := func(name string) *dpv1alpha1.ClusterRestore {
		return &dpv1alpha1.ClusterRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: nsName,
			},
			Spec: dpv1alpha1.ClusterRestoreSpec{
				TargetClusterName: envtestTargetClusterName,
				BackupRef: dpv1alpha1.ClusterRestoreBackupRef{
					Name:      envtestSourceBackupName,
					Namespace: envtestSourceBackupNS,
				},
			},
		}
	}

	newPVC := func() *corev1.PersistentVolumeClaim {
		sc := envtestStorageClassName
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      envtestPVCName,
				Namespace: nsName,
				Annotations: map[string]string{
					dptypes.VolumeSourceAnnotationKey: "data",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &sc,
			},
		}
	}

	targetRef := func() *dpv1alpha1.ClusterRestoreTargetClusterRef {
		return &dpv1alpha1.ClusterRestoreTargetClusterRef{
			Name:      envtestTargetClusterName,
			Namespace: nsName,
			UID:       envtestTargetClusterUID,
		}
	}

	// seedClusterRestore creates the ClusterRestore, then waits for the
	// BeforeSuite-registered ClusterRestoreReconciler to settle on
	// terminal Phase=Failed (the Backup the spec references does not
	// exist, so getAndValidateBackup returns NewFatalError and the
	// manager's reconciler patches Phase=Failed exactly once). After
	// that the manager early-returns on every subsequent enqueue
	// (terminal phase guard at the top of Reconcile), and the spec is
	// safe to overwrite Status.Conditions / TargetClusterRef without a
	// race. Phase stays Failed through the spec's call to
	// handleStorageClassNotFound, which patches Phase=Failed again
	// (idempotent).
	seedClusterRestore := func(name string, conditions []metav1.Condition) *dpv1alpha1.ClusterRestore {
		cr := newClusterRestore(name)
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		// Wait for manager's terminal settle. This is bounded
		// by getAndValidateBackup -> NewFatalError -> Phase=Failed.
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), cr)).To(Succeed())
			g.Expect(cr.Status.Phase).To(Equal(dpv1alpha1.ClusterRestorePhaseFailed))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed(), "manager-side reconciler did not settle on terminal Phase=Failed")
		// Now overwrite the seeded Status.Conditions for the spec under
		// test. Phase stays Failed; manager continues to early-return.
		patch := client.MergeFrom(cr.DeepCopy())
		cr.Status.Conditions = conditions
		cr.Status.TargetClusterRef = targetRef()
		Expect(k8sClient.Status().Patch(ctx, cr, patch)).To(Succeed())
		// Re-fetch so the spec sees the post-patch resourceVersion and
		// the persisted condition list.
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), cr)).To(Succeed())
		return cr
	}

	BeforeEach(func() {
		nsName = fmt.Sprintf("dp-cr-sc-envtest-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: nsName},
		})).To(Succeed())
		recorder = record.NewFakeRecorder(8)
		reconciler = &ClusterRestoreReconciler{
			Client:   k8sClient,
			Scheme:   k8sManager.GetScheme(),
			Recorder: recorder,
		}
	})

	AfterEach(func() {
		// Best-effort cleanup; envtest namespaces stay around but the
		// resources inside them do not leak across specs because spec
		// fixtures are name-prefixed by ns.
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
		_ = k8sClient.Delete(ctx, ns)
	})

	It("escalates to Failed/StorageClassMissing via real Status subresource and emits Warning event after the bounded wait window elapses", func() {
		crName := "envtest-escalate-after-window"
		// Pre-seed an existing WaitingForStorageClass condition whose
		// LastTransitionTime is past the 5-minute timeout. The
		// function must read this LTT through real API server
		// serialization (not in-memory) and use it to compute elapsed.
		// If LTT serialization is broken, elapsed would default to 0
		// and the function would NOT escalate, so the Failed/
		// StorageClassMissing outcome is itself proof that LTT
		// round-trip works.
		waitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - time.Second).Truncate(time.Second))
		cr := seedClusterRestore(crName, []metav1.Condition{
			{
				Type:               dpv1alpha1.ClusterRestoreReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             reasonWaitingForStorageClass,
				Message:            storageClassWaitingMessage(newClusterRestore(crName), envtestStorageClassName, newPVC()),
				ObservedGeneration: 1,
				LastTransitionTime: waitStart,
			},
		})

		result, err := reconciler.handleStorageClassNotFound(
			intctrlutil.RequestCtx{Ctx: ctx},
			cr,
			&dprestore.StorageClassNotFoundError{Name: envtestStorageClassName},
			targetRef(),
			newPVC(),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(BeZero(), "escalated branch should reconcile to terminal, not requeue")

		// Read back through the API server (real Status subresource path)
		// to confirm what was actually persisted, not the in-memory
		// reconciler-local copy.
		latest := &dpv1alpha1.ClusterRestore{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), latest)).To(Succeed())
		Expect(latest.Status.Phase).To(Equal(dpv1alpha1.ClusterRestorePhaseFailed))
		Expect(latest.Status.TargetClusterRef).NotTo(BeNil())
		Expect(latest.Status.TargetClusterRef.Name).To(Equal(envtestTargetClusterName))

		cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(reasonStorageClassMissing))
		Expect(cond.Message).To(ContainSubstring(envtestStorageClassName))
		Expect(cond.Message).To(ContainSubstring(fmt.Sprintf("after %s", storageClassWaitTimeout)))
		Expect(cond.Message).To(ContainSubstring(fmt.Sprintf("re-apply ClusterRestore %s/%s", nsName, crName)))

		// Bounded drain so async event delivery does not flake the
		// assertion. Single Warning event is the contract.
		evt, ok := drainSingleEvent(recorder, envtestEventDrainTimeout)
		Expect(ok).To(BeTrue(), "expected a single Warning event for StorageClassMissing escalation")
		Expect(evt).To(ContainSubstring(corev1.EventTypeWarning))
		Expect(evt).To(ContainSubstring(dprestore.ReasonRestoreFailed))
		Expect(evt).To(ContainSubstring(envtestStorageClassName))
	})

	It("preserves unrelated Status conditions through the real API server when escalating to StorageClassMissing", func() {
		crName := "envtest-preserve-unrelated"
		waitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - 5*time.Second).Truncate(time.Second))
		unrelatedLTT := metav1.NewTime(time.Now().Add(-time.Minute).Truncate(time.Second))
		cr := seedClusterRestore(crName, []metav1.Condition{
			{
				Type:               dpv1alpha1.ClusterRestoreReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             reasonWaitingForStorageClass,
				Message:            storageClassWaitingMessage(newClusterRestore(crName), envtestStorageClassName, newPVC()),
				ObservedGeneration: 1,
				LastTransitionTime: waitStart,
			},
			{
				Type:               "OtherControllerCondition",
				Status:             metav1.ConditionTrue,
				Reason:             "KeepMe",
				Message:            "owned by another status writer",
				ObservedGeneration: 1,
				LastTransitionTime: unrelatedLTT,
			},
		})

		_, err := reconciler.handleStorageClassNotFound(
			intctrlutil.RequestCtx{Ctx: ctx},
			cr,
			&dprestore.StorageClassNotFoundError{Name: envtestStorageClassName},
			targetRef(),
			newPVC(),
		)
		Expect(err).NotTo(HaveOccurred())

		latest := &dpv1alpha1.ClusterRestore{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), latest)).To(Succeed())

		ready := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(reasonStorageClassMissing))

		other := meta.FindStatusCondition(latest.Status.Conditions, "OtherControllerCondition")
		Expect(other).NotTo(BeNil(), "unrelated condition must survive Status patch through real API server")
		Expect(other.Reason).To(Equal("KeepMe"))
		Expect(other.Status).To(Equal(metav1.ConditionTrue))
		// metav1.Time has second precision, so an exact wall-clock
		// equality check survives the API server round-trip.
		Expect(other.LastTransitionTime.Time.Equal(unrelatedLTT.Time)).To(BeTrue(),
			"unrelated condition LastTransitionTime must round-trip through API server unchanged")

		// Drain to keep the recorder buffer clean for downstream specs.
		_, _ = drainSingleEvent(recorder, envtestEventDrainTimeout)
	})

	It("propagates the missing StorageClass identifier and ClusterRestore identity through real API server serialization", func() {
		crName := "envtest-message-identity"
		waitStart := metav1.NewTime(time.Now().Add(-storageClassWaitTimeout - 10*time.Second).Truncate(time.Second))
		cr := seedClusterRestore(crName, []metav1.Condition{
			{
				Type:               dpv1alpha1.ClusterRestoreReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             reasonWaitingForStorageClass,
				Message:            storageClassWaitingMessage(newClusterRestore(crName), envtestStorageClassName, newPVC()),
				ObservedGeneration: 1,
				LastTransitionTime: waitStart,
			},
		})

		_, err := reconciler.handleStorageClassNotFound(
			intctrlutil.RequestCtx{Ctx: ctx},
			cr,
			&dprestore.StorageClassNotFoundError{Name: envtestStorageClassName},
			targetRef(),
			newPVC(),
		)
		Expect(err).NotTo(HaveOccurred())

		latest := &dpv1alpha1.ClusterRestore{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cr), latest)).To(Succeed())
		cond := meta.FindStatusCondition(latest.Status.Conditions, dpv1alpha1.ClusterRestoreReadyCondition)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal(reasonStorageClassMissing))
		// Contract substrings: caller can grep the failure message
		// to identify (a) which StorageClass is missing and (b) which
		// ClusterRestore needs to be re-applied. Message rendering goes
		// through API server serialization in this assertion path.
		Expect(cond.Message).To(ContainSubstring(fmt.Sprintf(`StorageClass %q not found`, envtestStorageClassName)))
		Expect(cond.Message).To(ContainSubstring(fmt.Sprintf("ClusterRestore %s/%s", nsName, crName)))

		evt, ok := drainSingleEvent(recorder, envtestEventDrainTimeout)
		Expect(ok).To(BeTrue())
		Expect(evt).To(ContainSubstring(envtestStorageClassName))
		Expect(evt).To(ContainSubstring(corev1.EventTypeWarning))
	})
})

// drainSingleEvent reads exactly one event from a FakeRecorder with a
// bounded timeout, so the assertion does not hang on async event
// delivery and does not flake on under-counted buffers. Returns the
// event string and ok=true if one event was received within timeout.
func drainSingleEvent(recorder *record.FakeRecorder, timeout time.Duration) (string, bool) {
	select {
	case evt := <-recorder.Events:
		return strings.TrimSpace(evt), true
	case <-time.After(timeout):
		return "", false
	}
}
