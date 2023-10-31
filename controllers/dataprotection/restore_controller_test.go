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
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
)

var _ = Describe("Restore Controller test", func() {
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.SecretSignature, inNS, ml)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("restore controller test", func() {
		var (
			repoPVCName string
			actionSet   *dpv1alpha1.ActionSet
			nodeName    = "minikube"
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet = testdp.NewFakeActionSet(&testCtx)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating a backupRepo")
			_, repoPVCName = testdp.NewFakeBackupRepo(&testCtx, nil)
		})

		initResourcesAndWaitRestore := func(
			mockBackupCompleted,
			useVolumeSnapshot,
			isSerialPolicy bool,
			expectRestorePhase dpv1alpha1.RestorePhase,
			change func(f *testdp.MockRestoreFactory)) *dpv1alpha1.Restore {
			By("create a completed backup")
			backup := mockBackupForRestore(actionSet.Name, repoPVCName, mockBackupCompleted, useVolumeSnapshot)

			By("create restore ")
			schedulingSpec := dpv1alpha1.SchedulingSpec{
				NodeName: nodeName,
			}
			restoreFactory := testdp.NewRestoreactory(testCtx.DefaultNamespace, testdp.RestoreName).
				SetBackup(backup.Name, testCtx.DefaultNamespace).
				SetSchedulingSpec(schedulingSpec)

			change(restoreFactory)

			if isSerialPolicy {
				restoreFactory.SetVolumeClaimRestorePolicy(dpv1alpha1.VolumeClaimRestorePolicySerial)
			}
			restore := restoreFactory.Create(&testCtx).GetObject()

			By(fmt.Sprintf("wait for restore is %s", expectRestorePhase))
			restoreKey := client.ObjectKeyFromObject(restore)
			Eventually(testapps.CheckObj(&testCtx, restoreKey, func(g Gomega, r *dpv1alpha1.Restore) {
				g.Expect(r.Status.Phase).Should(Equal(expectRestorePhase))
			})).Should(Succeed())
			return restore
		}

		checkJobAndPVCSCount := func(restore *dpv1alpha1.Restore, jobReplicas, pvcReplicas, startingIndex int) {
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(jobReplicas))

			pvcMatchingLabels := client.MatchingLabels{constant.AppManagedByLabelKey: "restore"}
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, pvcMatchingLabels,
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(pvcReplicas))

			By(fmt.Sprintf("pvc index should greater than or equal to %d", startingIndex))
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, pvcList, pvcMatchingLabels,
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, v := range pvcList.Items {
				indexStr := string(v.Name[len(v.Name)-1])
				index, _ := strconv.Atoi(indexStr)
				Expect(index >= startingIndex).Should(BeTrue())
			}
		}

		mockRestoreJobsCompleted := func(restore *dpv1alpha1.Restore) {
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, v := range jobList.Items {
				testdp.PatchK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&v), batchv1.JobComplete)
			}
		}

		testRestoreWithVolumeClaimsTemplate := func(replicas, startingIndex int) {
			restore := initResourcesAndWaitRestore(true, false, false, dpv1alpha1.RestorePhaseRunning,
				func(f *testdp.MockRestoreFactory) {
					f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
						testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex))
				})

			By("expect restore jobs and pvcs are created")
			checkJobAndPVCSCount(restore, replicas, replicas, startingIndex)

			By("mock jobs are completed")
			mockRestoreJobsCompleted(restore)

			By("wait for restore is completed")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
				g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseCompleted))
			})).Should(Succeed())
		}

		Context("with restore fails", func() {
			It("test restore is Failed when backup is not completed", func() {
				By("expect for restore is Failed ")
				restore := initResourcesAndWaitRestore(false, false, true, dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(3), int32(0))
					})
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseFailed))
				})).Should(Succeed())
			})

			It("test restore is Failed when restore job is not Failed", func() {
				By("expect for restore is Failed ")
				restore := initResourcesAndWaitRestore(true, false, true, dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(3), int32(0))
					})

				By("wait for creating first job and pvc")
				checkJobAndPVCSCount(restore, 1, 1, 0)

				By("mock restore job is Failed")
				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())

				for _, v := range jobList.Items {
					testdp.PatchK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&v), batchv1.JobFailed)
				}

				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("test prepareData stage", func() {
			It("test volumeClaimsTemplate when startingIndex is 0", func() {
				testRestoreWithVolumeClaimsTemplate(3, 0)
			})

			It("test volumeClaimsTemplate when startingIndex is 1", func() {
				testRestoreWithVolumeClaimsTemplate(2, 1)
			})

			It("test volumeClaimsTemplate when volumeClaimRestorePolicy is Serial", func() {
				replicas := 2
				startingIndex := 1
				restore := initResourcesAndWaitRestore(true, false, true, dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex))
					})

				By("wait for creating first job and pvc")
				checkJobAndPVCSCount(restore, 1, 1, startingIndex)

				By("mock jobs are completed")
				mockRestoreJobsCompleted(restore)

				var firstJobName string
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Actions.PrepareData).ShouldNot(BeEmpty())
					g.Expect(r.Status.Actions.PrepareData[0].Status).Should(Equal(dpv1alpha1.RestoreActionCompleted))
					firstJobName = strings.ReplaceAll(r.Status.Actions.PrepareData[0].ObjectKey, "Job/", "")
				})).Should(Succeed())

				By("wait for deleted first job")
				Eventually(testapps.CheckObjExists(&testCtx,
					types.NamespacedName{Name: firstJobName, Namespace: testCtx.DefaultNamespace}, &batchv1.Job{}, false)).Should(Succeed())

				By("after the first job is completed, next job will be created")
				checkJobAndPVCSCount(restore, 1, replicas, startingIndex)

				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())

				for _, v := range jobList.Items {
					finished, _, _ := dputils.IsJobFinished(&v)
					Expect(finished).Should(BeFalse())
				}

				By("mock jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for restore is completed")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseCompleted))
				})).Should(Succeed())
			})

			It("test dataSourceRef", func() {
				initResourcesAndWaitRestore(true, true, false, dpv1alpha1.RestorePhaseAsDataSource,
					func(f *testdp.MockRestoreFactory) {
						f.SetDataSourceRef(testdp.DataVolumeName, testdp.DataVolumeMountPath)
					})
			})

		})

		Context("test postReady stage", func() {
			var _ *testdp.BackupClusterInfo
			BeforeEach(func() {
				By("fake a new cluster")
				_ = testdp.NewFakeCluster(&testCtx)
			})

			It("test post ready actions", func() {
				By("remove the prepareData stage for testing post ready actions")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
					set.Spec.Restore.PrepareData = nil
				})).Should(Succeed())

				matchLabels := map[string]string{
					constant.AppInstanceLabelKey: testdp.ClusterName,
				}
				restore := initResourcesAndWaitRestore(true, false, false, dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetConnectCredential(testdp.ClusterName).SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
					})

				By("wait for creating two exec jobs with the matchLabels")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(2))

				By("mock exec jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for creating a job of jobAction with the matchLabels, expect jobs count is 3(2+1)")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionLabelRestoreKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(3))

				By("mock jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for restore is completed")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseCompleted))
				})).Should(Succeed())

				By("test deleting restore")
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(restore), restore, false)).Should(Succeed())
			})
		})
	})
})

func mockBackupForRestore(actionSetName, backupPVCName string, mockBackupCompleted, useVolumeSnapshotBackup bool) *dpv1alpha1.Backup {
	backup := testdp.NewFakeBackup(&testCtx, nil)
	// wait for backup is failed by backup controller.
	// it will be failed if the backupPolicy is not created.
	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
		g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseFailed))
	})).Should(Succeed())

	if mockBackupCompleted {
		// then mock backup to completed
		backupMethodName := testdp.BackupMethodName
		if useVolumeSnapshotBackup {
			backupMethodName = testdp.VSBackupMethodName
		}
		Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
			backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			backup.Status.PersistentVolumeClaimName = backupPVCName
			testdp.MockBackupStatusMethod(backup, backupMethodName, testdp.DataVolumeName, actionSetName)
		})).Should(Succeed())
	}
	return backup
}
