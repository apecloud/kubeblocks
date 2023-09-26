/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dpbackup "github.com/apecloud/kubeblocks/internal/dataprotection/backup"
	"github.com/apecloud/kubeblocks/internal/dataprotection/utils/boolptr"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/internal/testutil/dataprotection"
)

var _ = Describe("Backup Schedule Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupScheduleSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.CronJobSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		_ = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(cleanEnv)

	When("creating backup schedule with default settings", func() {
		var (
			backupPolicy *dpv1alpha1.BackupPolicy
		)

		getCronjobKey := func(backupSchedule *dpv1alpha1.BackupSchedule,
			method string) client.ObjectKey {
			return client.ObjectKey{
				Name:      dpbackup.GenerateCRNameByBackupSchedule(backupSchedule, method),
				Namespace: backupPolicy.Namespace,
			}
		}

		getJobKey := func(backup *dpv1alpha1.Backup) client.ObjectKey {
			return client.ObjectKey{
				Name:      dpbackup.GenerateBackupJobName(backup, dpbackup.BackupDataJobNamePrefix),
				Namespace: backup.Namespace,
			}
		}

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet := testdp.NewFakeActionSet(&testCtx)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating backup repo")
			_, _ = testdp.NewFakeBackupRepo(&testCtx, nil)

			By("By creating a backupPolicy from actionSet " + actionSet.Name)
			backupPolicy = testdp.NewFakeBackupPolicy(&testCtx, nil)
		})

		AfterEach(func() {
		})

		Context("creates a backup schedule", func() {
			var (
				backupNamePrefix  = "schedule-test-backup-"
				backupSchedule    *dpv1alpha1.BackupSchedule
				backupScheduleKey client.ObjectKey
			)
			BeforeEach(func() {
				By("creating a backupSchedule")
				backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, nil)
				backupScheduleKey = client.ObjectKeyFromObject(backupSchedule)
			})

			It("should success", func() {
				By("checking backupSchedule status, should be available")
				Eventually(testapps.CheckObj(&testCtx, backupScheduleKey, func(g Gomega, fetched *dpv1alpha1.BackupSchedule) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupSchedulePhaseAvailable))
				})).Should(Succeed())

				By("checking cronjob, should not exist because all schedule policies of methods are disabled")
				Eventually(testapps.CheckObjExists(&testCtx, getCronjobKey(backupSchedule, testdp.BackupMethodName),
					&batchv1.CronJob{}, false)).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, getCronjobKey(backupSchedule, testdp.VSBackupMethodName),
					&batchv1.CronJob{}, false)).Should(Succeed())

				By(fmt.Sprintf("enabling %s method schedule", testdp.BackupMethodName))
				testdp.EnableBackupSchedule(&testCtx, backupSchedule, testdp.BackupMethodName)

				By("checking cronjob, should exist one cronjob to create backup")
				Eventually(testapps.CheckObj(&testCtx, getCronjobKey(backupSchedule, testdp.BackupMethodName), func(g Gomega, fetched *batchv1.CronJob) {
					schedulePolicy := dpbackup.GetSchedulePolicyByMethod(backupSchedule, testdp.BackupMethodName)
					g.Expect(boolptr.IsSetToTrue(schedulePolicy.Enabled)).To(BeTrue())
					g.Expect(fetched.Spec.Schedule).To(Equal(schedulePolicy.CronExpression))
					g.Expect(fetched.Spec.StartingDeadlineSeconds).ShouldNot(BeNil())
					g.Expect(*fetched.Spec.StartingDeadlineSeconds).To(Equal(getStartingDeadlineSeconds(backupSchedule)))
				})).Should(Succeed())
			})

			It("delete expired backups", func() {
				now := metav1.Now()
				backupStatus := dpv1alpha1.BackupStatus{
					Phase:               dpv1alpha1.BackupPhaseCompleted,
					Expiration:          &now,
					StartTimestamp:      &now,
					CompletionTimestamp: &now,
				}

				autoBackupLabel := map[string]string{
					dataProtectionLabelAutoBackupKey:   "true",
					dataProtectionLabelBackupPolicyKey: testdp.BackupPolicyName,
					dataProtectionLabelBackupMethodKey: testdp.BackupMethodName,
				}

				createBackup := func(name string) *dpv1alpha1.Backup {
					return testdp.NewBackupFactory(testCtx.DefaultNamespace, name).
						WithRandomName().AddLabelsInMap(autoBackupLabel).
						SetBackupPolicyName(testdp.BackupPolicyName).
						SetBackupMethod(testdp.BackupMethodName).
						Create(&testCtx).GetObject()
				}

				checkBackupCompleted := func(key client.ObjectKey) {
					Eventually(testapps.CheckObj(&testCtx, key,
						func(g Gomega, fetched *dpv1alpha1.Backup) {
							g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
						})).Should(Succeed())
				}

				By("create an expired backup")
				backupExpired := createBackup(backupNamePrefix + "expired")

				By("create 1st backup")
				backupOutLimit1 := createBackup(backupNamePrefix + "1")

				By("create 2nd backup")
				backupOutLimit2 := createBackup(backupNamePrefix + "2")

				By("waiting expired backup completed")
				expiredKey := client.ObjectKeyFromObject(backupExpired)
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(backupExpired), batchv1.JobComplete)
				checkBackupCompleted(expiredKey)

				By("mock update expired backup status to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(-time.Hour * 24)}
				backupStatus.StartTimestamp = backupStatus.Expiration
				testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backupExpired), backupStatus)

				By("waiting 1st backup completed")
				outLimit1Key := client.ObjectKeyFromObject(backupOutLimit1)
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(backupOutLimit1), batchv1.JobComplete)
				checkBackupCompleted(outLimit1Key)

				By("mock 1st backup not to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour)}
				testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backupOutLimit1), backupStatus)

				By("waiting 2nd backup completed")
				outLimit2Key := client.ObjectKeyFromObject(backupOutLimit2)
				testdp.PatchK8sJobStatus(&testCtx, getJobKey(backupOutLimit2), batchv1.JobComplete)
				checkBackupCompleted(outLimit2Key)

				By("mock 2nd backup not to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour * 2)}
				testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backupOutLimit2), backupStatus)

				By("patch backup schedule to trigger the controller to delete expired backup")
				Eventually(testapps.GetAndChangeObj(&testCtx, backupScheduleKey, func(fetched *dpv1alpha1.BackupSchedule) {
					fetched.Spec.Schedules[0].RetentionPeriod = "1d"
				})).Should(Succeed())

				By("retain the latest backup")
				Eventually(testapps.List(&testCtx, generics.BackupSignature,
					client.MatchingLabels(autoBackupLabel),
					client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(2))
			})
		})

		Context("creates a backup schedule with empty schedule", func() {
			It("should fail when create a backupSchedule without nil schedule policy", func() {
				backupScheduleObj := testdp.NewBackupScheduleFactory(testCtx.DefaultNamespace, testdp.BackupScheduleName).
					SetBackupPolicyName(testdp.BackupPolicyName).
					SetSchedules(nil).
					GetObject()
				Expect(testCtx.CheckedCreateObj(testCtx.Ctx, backupScheduleObj)).Should(HaveOccurred())
			})

			It("should fail when create a backupSchedule without empty schedule policy", func() {
				backupScheduleObj := testdp.NewBackupScheduleFactory(testCtx.DefaultNamespace, testdp.BackupScheduleName).
					SetBackupPolicyName(testdp.BackupPolicyName).
					GetObject()
				Expect(testCtx.CheckedCreateObj(testCtx.Ctx, backupScheduleObj)).Should(HaveOccurred())
			})
		})

		Context("creates a backup schedule with invalid field", func() {
			var (
				backupScheduleKey client.ObjectKey
				backupSchedule    *dpv1alpha1.BackupSchedule
			)

			BeforeEach(func() {
				By("creating a backupSchedule")
				backupSchedule = testdp.NewFakeBackupSchedule(&testCtx, func(schedule *dpv1alpha1.BackupSchedule) {
					schedule.Spec.Schedules[0].CronExpression = "invalid"
				})
				backupScheduleKey = client.ObjectKeyFromObject(backupSchedule)
			})

			It("should fail", func() {
				Eventually(testapps.CheckObj(&testCtx, backupScheduleKey, func(g Gomega, fetched *dpv1alpha1.BackupSchedule) {
					g.Expect(fetched.Status.Phase).NotTo(Equal(dpv1alpha1.BackupSchedulePhaseAvailable))
				})).Should(Succeed())
			})
		})
	})
})

func getStartingDeadlineSeconds(backupSchedule *dpv1alpha1.BackupSchedule) int64 {
	if backupSchedule.Spec.StartingDeadlineMinutes == nil {
		return 0
	}
	return *backupSchedule.Spec.StartingDeadlineMinutes * 60
}
