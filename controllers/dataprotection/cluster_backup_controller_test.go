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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Cluster Backup Controller", func() {
	const customBackupFinalizer = "test.kubeblocks.io/backup-hold"

	cleanEnv := func() {
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS, ml)
	}

	newCluster := func(name string, terminationPolicy appsv1.TerminationPolicyType) *appsv1.Cluster {
		cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, name, "").
			SetTerminationPolicy(terminationPolicy).
			AddLabels(testCtx.TestObjLabelKey, "true").
			Create(&testCtx).
			GetObject()
		Eventually(func(g Gomega) {
			current := &appsv1.Cluster{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), current)).Should(Succeed())
			g.Expect(controllerutil.ContainsFinalizer(current, dptypes.DataProtectionFinalizerName)).Should(BeTrue())
		}).Should(Succeed())
		return cluster
	}

	newBackup := func(cluster *appsv1.Cluster, name string, phase dpv1alpha1.BackupPhase, backupType string,
		deletionPolicy dpv1alpha1.BackupDeletionPolicy, addClusterUID bool, extraFinalizers ...string) *dpv1alpha1.Backup {
		labels := map[string]string{
			constant.AppInstanceLabelKey: cluster.Name,
		}
		labels[testCtx.TestObjLabelKey] = "true"
		if backupType != "" {
			labels[dptypes.BackupTypeLabelKey] = backupType
		}
		if addClusterUID {
			labels[dptypes.ClusterUIDLabelKey] = string(cluster.UID)
		}
		backup := testdp.NewBackupFactory(testCtx.DefaultNamespace, name).
			SetBackupPolicyName("test-backup-policy").
			SetBackupMethod("test-backup-method").
			SetLabels(labels).
			Apply(func(b *dpv1alpha1.Backup) {
				b.Spec.DeletionPolicy = deletionPolicy
				b.Annotations = map[string]string{dptypes.SkipReconciliationAnnotationKey: "true"}
				if len(extraFinalizers) > 0 {
					b.Finalizers = append([]string{}, extraFinalizers...)
				}
			}).
			Create(&testCtx).
			GetObject()
		Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
			backup.Status.Phase = phase
		})).Should(Succeed())
		return backup
	}

	clusterEventuallyDeleted := func(clusterKey client.ObjectKey) {
		Eventually(func() bool {
			cluster := &appsv1.Cluster{}
			err := k8sClient.Get(ctx, clusterKey, cluster)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	}

	backupEventuallyDeleted := func(backupKey client.ObjectKey) {
		Eventually(func() bool {
			backup := &dpv1alpha1.Backup{}
			err := k8sClient.Get(ctx, backupKey, backup)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	}

	backupConsistentlyExists := func(backupKey client.ObjectKey) {
		Consistently(func() error {
			return k8sClient.Get(ctx, backupKey, &dpv1alpha1.Backup{})
		}, 2*time.Second, 200*time.Millisecond).Should(Succeed())
	}

	BeforeEach(cleanEnv)
	AfterEach(cleanEnv)

	It("adds dataprotection finalizer to non-deleting clusters", func() {
		cluster := newCluster("cluster-finalizer", appsv1.Delete)
		Consistently(func() error {
			current := &appsv1.Cluster{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), current); err != nil {
				return err
			}
			if !controllerutil.ContainsFinalizer(current, dptypes.DataProtectionFinalizerName) {
				return apierrors.NewBadRequest("dataprotection finalizer missing")
			}
			return nil
		}).Should(Succeed())
	})

	It("deletes all non-retain backups during wipeout", func() {
		cluster := newCluster("cluster-wipeout", appsv1.WipeOut)
		completed := newBackup(cluster, "backup-completed", dpv1alpha1.BackupPhaseCompleted, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, true)
		failed := newBackup(cluster, "backup-failed", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, true)
		continuous := newBackup(cluster, "backup-continuous", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeContinuous), dpv1alpha1.BackupDeletionPolicyDelete, true)
		retained := newBackup(cluster, "backup-retain", dpv1alpha1.BackupPhaseCompleted, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyRetain, true)

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

		backupEventuallyDeleted(client.ObjectKeyFromObject(completed))
		backupEventuallyDeleted(client.ObjectKeyFromObject(failed))
		backupEventuallyDeleted(client.ObjectKeyFromObject(continuous))
		backupConsistentlyExists(client.ObjectKeyFromObject(retained))
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})

	It("deletes only failed non-continuous backups for non-wipeout policies", func() {
		cluster := newCluster("cluster-delete", appsv1.Delete)
		failed := newBackup(cluster, "backup-failed-delete", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, true)
		completed := newBackup(cluster, "backup-completed-keep", dpv1alpha1.BackupPhaseCompleted, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, true)
		continuous := newBackup(cluster, "backup-continuous-keep", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeContinuous), dpv1alpha1.BackupDeletionPolicyDelete, true)
		retained := newBackup(cluster, "backup-retained-keep", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyRetain, true)

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

		backupEventuallyDeleted(client.ObjectKeyFromObject(failed))
		backupConsistentlyExists(client.ObjectKeyFromObject(completed))
		backupConsistentlyExists(client.ObjectKeyFromObject(continuous))
		backupConsistentlyExists(client.ObjectKeyFromObject(retained))
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})

	It("ignores backups from another cluster uid", func() {
		cluster := newCluster("cluster-uid-guard", appsv1.WipeOut)
		otherUIDBackup := newBackup(cluster, "backup-other-uid", dpv1alpha1.BackupPhaseCompleted, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, false)
		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(otherUIDBackup), func(backup *dpv1alpha1.Backup) {
			backup.Labels[dptypes.ClusterUIDLabelKey] = "another-cluster-uid"
		})()).Should(Succeed())

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

		backupConsistentlyExists(client.ObjectKeyFromObject(otherUIDBackup))
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})

	It("keeps cluster finalizer until deleting candidate backups disappear", func() {
		cluster := newCluster("cluster-pending-cleanup", appsv1.Delete)
		backup := newBackup(cluster, "backup-pending", dpv1alpha1.BackupPhaseFailed, string(dpv1alpha1.BackupTypeFull),
			dpv1alpha1.BackupDeletionPolicyDelete, true, customBackupFinalizer)

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

		Eventually(func(g Gomega) {
			currentCluster := &appsv1.Cluster{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), currentCluster)).Should(Succeed())
			g.Expect(currentCluster.GetDeletionTimestamp()).ShouldNot(BeNil())
			g.Expect(controllerutil.ContainsFinalizer(currentCluster, dptypes.DataProtectionFinalizerName)).Should(BeTrue())

			currentBackup := &dpv1alpha1.Backup{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), currentBackup)).Should(Succeed())
			g.Expect(currentBackup.GetDeletionTimestamp()).ShouldNot(BeNil())
		}).Should(Succeed())

		Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(backup), func(currentBackup *dpv1alpha1.Backup) {
			currentBackup.Finalizers = nil
		})()).Should(Succeed())

		backupEventuallyDeleted(client.ObjectKeyFromObject(backup))
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})

	It("removes dataprotection finalizer when no candidate backups remain", func() {
		cluster := newCluster("cluster-no-backups", appsv1.Delete)

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})

	It("does not depend on cluster phase when deleting backups", func() {
		cluster := newCluster("cluster-phase-agnostic", appsv1.WipeOut)
		Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cluster), func(currentCluster *appsv1.Cluster) {
			currentCluster.Status.Phase = appsv1.FailedClusterPhase
		})()).Should(Succeed())
		backup := newBackup(cluster, "backup-phase-agnostic", dpv1alpha1.BackupPhaseCompleted, string(dpv1alpha1.BackupTypeFull), dpv1alpha1.BackupDeletionPolicyDelete, true)

		Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())

		backupEventuallyDeleted(client.ObjectKeyFromObject(backup))
		clusterEventuallyDeleted(client.ObjectKeyFromObject(cluster))
	})
})
