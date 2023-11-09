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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dpbackup "github.com/apecloud/kubeblocks/pkg/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
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

		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupPolicySignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		_ = testdp.NewFakeCluster(&testCtx)
	})

	AfterEach(cleanEnv)

	Context("garbage collection", func() {
		var (
			backupNamePrefix = "schedule-test-backup-"
			backupPolicy     *dpv1alpha1.BackupPolicy
		)

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

		It("delete expired backups", func() {
			now := metav1.Now()
			backupStatus := dpv1alpha1.BackupStatus{
				Phase:               dpv1alpha1.BackupPhaseCompleted,
				Expiration:          &now,
				StartTimestamp:      &now,
				CompletionTimestamp: &now,
			}

			autoBackupLabel := map[string]string{
				dptypes.AutoBackupLabelKey:     "true",
				dptypes.BackupScheduleLabelKey: testdp.BackupPolicyName,
				dptypes.BackupMethodLabelKey:   testdp.BackupMethodName,
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

			setBackupUnexpired := func(backup *dpv1alpha1.Backup) {
				backup.Status.Expiration = &metav1.Time{Time: fakeClock.Now().Add(time.Hour * 24)}
				backup.Status.StartTimestamp = &metav1.Time{Time: fakeClock.Now().Add(time.Hour)}
				testdp.PatchBackupStatus(&testCtx, client.ObjectKeyFromObject(backup), backup.Status)
			}

			By("create an expired backup")
			backupExpired := createBackup(backupNamePrefix + "expired")

			By("create an unexpired backup")
			backup1 := createBackup(backupNamePrefix + "unexpired")

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
	})
})
