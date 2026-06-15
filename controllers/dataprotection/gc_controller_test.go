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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Data Protection Garbage Collection Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		_ = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(cleanEnv)

	Context("garbage collection", func() {
		It("uses configured GC frequency only when positive", func() {
			oldFrequency := viper.GetInt(dptypes.CfgKeyGCFrequencySeconds)
			defer viper.Set(dptypes.CfgKeyGCFrequencySeconds, oldFrequency)

			viper.Set(dptypes.CfgKeyGCFrequencySeconds, 7)
			Expect(getGCFrequency()).To(Equal(7 * time.Second))

			viper.Set(dptypes.CfgKeyGCFrequencySeconds, 0)
			Expect(getGCFrequency()).To(Equal(time.Duration(dptypes.DefaultGCFrequencySeconds)))
		})

		It("evaluates deterministic backup deletion decisions", func() {
			scheme := runtime.NewScheme()
			Expect(dpv1alpha1.AddToScheme(scheme)).Should(Succeed())

			completedAt := metav1.Time{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
			laterCompletedAt := metav1.Time{Time: completedAt.Add(time.Hour)}
			backupPolicy := &dpv1alpha1.BackupPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "policy",
				},
				Spec: dpv1alpha1.BackupPolicySpec{
					RetentionPolicy: dpv1alpha1.BackupPolicyRetentionPolicyRetainLatestBackup,
				},
			}
			olderBackup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "older",
					Labels: map[string]string{
						dptypes.ClusterUIDLabelKey:   "cluster-uid",
						dptypes.BackupPolicyLabelKey: backupPolicy.Name,
					},
				},
				Spec: dpv1alpha1.BackupSpec{
					BackupPolicyName: backupPolicy.Name,
					BackupMethod:     "full",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase:               dpv1alpha1.BackupPhaseCompleted,
					CompletionTimestamp: &completedAt,
				},
			}
			latestBackup := olderBackup.DeepCopy()
			latestBackup.Name = "latest"
			latestBackup.Status.CompletionTimestamp = &laterCompletedAt
			incrementalBackup := olderBackup.DeepCopy()
			incrementalBackup.Name = "incremental"
			incrementalBackup.Labels[dptypes.BackupTypeLabelKey] = string(dpv1alpha1.BackupTypeIncremental)
			runningBackup := olderBackup.DeepCopy()
			runningBackup.Name = "running"
			runningBackup.Status.Phase = dpv1alpha1.BackupPhaseRunning
			failedBackup := olderBackup.DeepCopy()
			failedBackup.Name = "failed"
			failedBackup.Status.Phase = dpv1alpha1.BackupPhaseFailed

			reconciler := &GCReconciler{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(backupPolicy, olderBackup, latestBackup, incrementalBackup, runningBackup, failedBackup).
					Build(),
			}
			reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

			deletable, err := reconciler.isBackupDeletable(reqCtx, runningBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(deletable).To(BeFalse())

			deletable, err = reconciler.isBackupDeletable(reqCtx, failedBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(deletable).To(BeTrue())

			deletable, err = reconciler.isBackupDeletable(reqCtx, incrementalBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(deletable).To(BeFalse())

			deletable, err = reconciler.isBackupDeletable(reqCtx, latestBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(deletable).To(BeFalse())

			deletable, err = reconciler.isBackupDeletable(reqCtx, olderBackup)
			Expect(err).NotTo(HaveOccurred())
			Expect(deletable).To(BeTrue())

			isLatest, err := reconciler.isLatestCompletedBackup(context.Background(), &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseRunning},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(isLatest).To(BeFalse())

			related, err := reconciler.getRelatedBackups(context.Background(), &dpv1alpha1.Backup{})
			Expect(err).NotTo(HaveOccurred())
			Expect(related).To(BeNil())
		})

		var (
			backupNamePrefix = "schedule-test-backup-"
			backupPolicy     *dpv1alpha1.BackupPolicy
			now              = metav1.Now()
			backupStatus     = dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupPhaseCompleted,
				Expiration:          &now,
				StartTimestamp:      &now,
				CompletionTimestamp: &now,
				Target: &dpv1alpha1.BackupStatusTarget{
					BackupTarget: dpv1alpha1.BackupTarget{
						PodSelector: &dpv1alpha1.PodSelector{
							Strategy: dpv1alpha1.PodSelectionStrategyAny,
						},
					},
				},
			}
			autoBackupLabel = map[string]string{
				dptypes.AutoBackupLabelKey:     "true",
				dptypes.BackupScheduleLabelKey: testdp.BackupPolicyName,
				dptypes.BackupMethodLabelKey:   testdp.BackupMethodName,
			}
		)

		getJobKey := func(backup *dpv1alpha1.Backup) client.ObjectKey {
			return client.ObjectKey{
				Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix+"-0"),
				Namespace: backup.Namespace,
			}
		}

		checkBackupCompleted := func(key client.ObjectKey) {
			Eventually(testapps.CheckObj(&testCtx, key,
				func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
				})).Should(Succeed())
		}

		createBackup := func(name, methodName string) *dpv1alpha1.Backup {
			return testdp.NewBackupFactory(testCtx.DefaultNamespace, name).
				WithRandomName().AddLabelsInMap(autoBackupLabel).
				SetBackupPolicyName(testdp.BackupPolicyName).
				SetBackupMethod(methodName).
				Create(&testCtx).GetObject()
		}

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet := testdp.NewFakeActionSet(&testCtx, nil)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating backup repo")
			_, _ = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("By creating a backupPolicy from actionSet " + actionSet.Name)
			backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)
		})

		It("delete expired backups", func() {
			setBackupUnexpired := func(backup *dpv1alpha1.Backup) {
				backup.Status.Expiration = &metav1.Time{Time: fakeClock.Now().Add(time.Hour * 24)}
				backup.Status.StartTimestamp = &metav1.Time{Time: fakeClock.Now().Add(time.Hour)}
				testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backup), backup.Status)
			}

			By("create an expired backup")
			backupExpired := createBackup(backupNamePrefix+"expired", testdp.BackupMethodName)

			By("create an unexpired backup")
			backup1 := createBackup(backupNamePrefix+"unexpired", testdp.BackupMethodName)

			By("waiting expired backup completed")
			expiredKey := client.ObjectKeyFromObject(backupExpired)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(backupExpired), batchv1.JobComplete)
			checkBackupCompleted(expiredKey)

			By("mock backup status to expire")
			backupStatus.Expiration = &metav1.Time{Time: fakeClock.Now().Add(-time.Hour * 24)}
			backupStatus.StartTimestamp = backupStatus.Expiration
			testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backupExpired), backupStatus)

			By("waiting backup completed")
			backup1Key := client.ObjectKeyFromObject(backup1)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(backup1), batchv1.JobComplete)
			checkBackupCompleted(backup1Key)

			By("mock backup not to expire")
			setBackupUnexpired(backup1)

			By("retain the unexpired backup")
			Eventually(testapps.List(&testCtx, generics.BackupSignature,
				client.MatchingLabels(autoBackupLabel),
				client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(1))
			Eventually(testapps.CheckObjExists(&testCtx, backup1Key, &dpv1alpha1.Backup{}, true)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, expiredKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
		})

		It("should not delete the latest backup", func() {
			shouldNotDelete := func(key client.ObjectKey) {
				Eventually(testapps.CheckObjExists(&testCtx, key, &dpv1alpha1.Backup{}, true)).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, key,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
						g.Expect(fetched.DeletionTimestamp).To(BeNil())
					})).Should(Succeed())
			}

			By("setting the backup policy retention policy to retain latest backup")
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
				policy.Spec.RetentionPolicy = dpv1alpha1.BackupPolicyRetentionPolicyRetainLatestBackup
			})).Should(Succeed())

			By("creating an older full backup")
			olderBackup := createBackup("older-full-backup", testdp.BackupMethodName)
			olderKey := client.ObjectKeyFromObject(olderBackup)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(olderBackup), batchv1.JobComplete)
			checkBackupCompleted(olderKey)

			By("setting the older full backup as expired")
			expiredTime := metav1.Time{Time: fakeClock.Now().Add(-time.Hour * 24)}
			olderBackup.Status.Expiration = &expiredTime
			olderBackup.Status.StartTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour * 3)}
			olderBackup.Status.CompletionTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour * 3)}
			olderBackup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			olderBackup.Status.BackupRepoName = testdp.BackupRepoName
			olderBackup.Status.Target = &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						Strategy: dpv1alpha1.PodSelectionStrategyAny,
					},
				},
			}
			testdp.PatchBackupStatus(&testCtx, olderKey, olderBackup.Status)

			By("the older full backup should be not deleted, it is the latest backup for now")
			time.Sleep(2 * gcFrequency)
			Eventually(testapps.List(&testCtx, generics.BackupSignature,
				client.MatchingLabels(autoBackupLabel),
				client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(1))
			shouldNotDelete(olderKey)

			By("creating an incremental backup whose parent is the older full backup")
			_ = testdp.NewFakeIncActionSet(&testCtx)
			incrementalBackup := createBackup("incremental-backup", testdp.IncBackupMethodName)
			incrementalKey := client.ObjectKeyFromObject(incrementalBackup)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(incrementalBackup), batchv1.JobComplete)
			checkBackupCompleted(incrementalKey)

			By("setting the incremental backup as expired")
			incrementalBackup.Status.Expiration = &expiredTime
			incrementalBackup.Status.ParentBackupName = olderKey.Name
			incrementalBackup.Status.StartTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour * 2)}
			incrementalBackup.Status.CompletionTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour * 2)}
			incrementalBackup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			incrementalBackup.Status.Target = &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						Strategy: dpv1alpha1.PodSelectionStrategyAny,
					},
				},
			}
			testdp.PatchBackupStatus(&testCtx, incrementalKey, incrementalBackup.Status)

			By("the incremental backup should be not deleted, its parent is the latest backup for now")
			time.Sleep(2 * gcFrequency)
			Eventually(testapps.List(&testCtx, generics.BackupSignature,
				client.MatchingLabels(autoBackupLabel),
				client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(2))
			shouldNotDelete(incrementalKey)
			By("the older full backup should be not deleted, it is the parent of the incremental backup")
			shouldNotDelete(olderKey)

			By("creating the latest full backup")
			latestBackup := createBackup("latest-full-backup", testdp.BackupMethodName)
			latestKey := client.ObjectKeyFromObject(latestBackup)
			testdp.PatchK8sJobStatus(&testCtx, getJobKey(latestBackup), batchv1.JobComplete)
			checkBackupCompleted(latestKey)

			By("setting the latest full backup as expired")
			latestBackup.Status.Expiration = &expiredTime
			latestBackup.Status.StartTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour)}
			latestBackup.Status.CompletionTimestamp = &metav1.Time{Time: expiredTime.Time.Add(-time.Hour)}
			latestBackup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			latestBackup.Status.Target = &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{
					PodSelector: &dpv1alpha1.PodSelector{
						Strategy: dpv1alpha1.PodSelectionStrategyAny,
					},
				},
			}
			testdp.PatchBackupStatus(&testCtx, latestKey, latestBackup.Status)

			By("verify the latest full backup is retained while older is deleted")
			Eventually(testapps.List(&testCtx, generics.BackupSignature,
				client.MatchingLabels(autoBackupLabel),
				client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(1))
			shouldNotDelete(latestKey)
			Eventually(testapps.CheckObjExists(&testCtx, olderKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, incrementalKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())

			By("reset the backup policy retention policy")
			Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
				policy.Spec.RetentionPolicy = ""
			})).Should(Succeed())

			By("verify all backups are deleted")
			Eventually(testapps.List(&testCtx, generics.BackupSignature,
				client.MatchingLabels(autoBackupLabel),
				client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(0))
			Eventually(testapps.CheckObjExists(&testCtx, latestKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, olderKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, incrementalKey, &dpv1alpha1.Backup{}, false)).Should(Succeed())
		})
	})
})
