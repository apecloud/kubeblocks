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

package restore

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Backup Deleter Test", func() {

	cleanEnv := func() {
		By("clean resources")
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// namespaced
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("with restore manager functions", func() {
		var (
			actionSet    *dpv1alpha1.ActionSet
			nodeName     = "minikube"
			replicas     = 2
			instanceName = "test"
		)

		BeforeEach(func() {

			By("create actionSet")
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.WithName(testdp.ActionSetName))

		})

		mockBackupForRestore := func(testCtx *testutil.TestContext, actionSetName, backupPVCName string, mockBackupCompleted, useVolumeSnapshotBackup bool) *dpv1alpha1.Backup {
			backup := testdp.NewFakeBackup(testCtx, nil)
			if mockBackupCompleted {
				// then mock backup to completed
				backupMethodName := testdp.BackupMethodName
				if useVolumeSnapshotBackup {
					backupMethodName = testdp.VSBackupMethodName
				}
				Expect(testapps.ChangeObjStatus(testCtx, backup, func() {
					backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
					backup.Status.PersistentVolumeClaimName = backupPVCName
					testdp.MockBackupStatusMethod(backup, backupMethodName, testdp.DataVolumeName, actionSetName)
				})).Should(Succeed())
			}
			return backup
		}

		initResources := func(reqCtx intctrlutil.RequestCtx, startingIndex int, useVolumeSnapshotBackup bool, change func(f *testdp.MockRestoreFactory)) (*RestoreManager, *BackupActionSet) {
			By("create a completed backup")
			backup := mockBackupForRestore(&testCtx, actionSet.Name, testdp.BackupPVCName, true, false)

			schedulingSpec := dpv1alpha1.SchedulingSpec{
				NodeName: nodeName,
			}

			By("create restore")
			restoreFactory := testdp.NewRestoreFactory(testCtx.DefaultNamespace, testdp.RestoreName).
				SetBackup(backup.Name, testCtx.DefaultNamespace).
				SetSchedulingSpec(schedulingSpec)

			change(restoreFactory)

			restore := restoreFactory.Create(&testCtx).Get()

			By("create restore manager")
			restoreMGR := NewRestoreManager(restore, recorder, k8sClient.Scheme())
			backupSet, err := restoreMGR.GetBackupActionSetByNamespaced(reqCtx, k8sClient, backup.Name, testCtx.DefaultNamespace)
			Expect(err).ShouldNot(HaveOccurred())
			return restoreMGR, backupSet
		}

		checkPVC := func(startingIndex int, useVolumeSnapshot bool) {
			By("expect for pvcs are created")
			pvcMatchingLabels := client.MatchingLabels{constant.AppManagedByLabelKey: "restore"}
			Eventually(testapps.List(&testCtx, generics.PersistentVolumeClaimSignature, pvcMatchingLabels,
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(replicas))

			By(fmt.Sprintf("pvc index should greater than or equal to %d and dataSource can not be nil", startingIndex))
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, pvcList, pvcMatchingLabels,
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, v := range pvcList.Items {
				indexStr := string(v.Name[len(v.Name)-1])
				index, _ := strconv.Atoi(indexStr)
				Expect(index >= startingIndex).Should(BeTrue())
				if useVolumeSnapshot {
					Expect(v.Spec.DataSource).ShouldNot(BeNil())
				}
			}
		}

		getReqCtx := func() intctrlutil.RequestCtx {
			return intctrlutil.RequestCtx{
				Ctx: ctx,
				Req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: testCtx.DefaultNamespace,
					},
				},
			}
		}

		checkVolumes := func(job *batchv1.Job, volumeName string, exist bool) {
			var volumeExist bool
			for _, v := range job.Spec.Template.Spec.Volumes {
				if v.Name == volumeName {
					volumeExist = true
					break
				}
			}
			Expect(volumeExist).Should(Equal(exist))
		}

		It("test with RestorePVCFromSnapshot function", func() {
			reqCtx := getReqCtx()
			startingIndex := 0
			useVolumeSnapshot := true
			restoreMGR, backupSet := initResources(reqCtx, startingIndex, useVolumeSnapshot, func(f *testdp.MockRestoreFactory) {
				f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
					testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), nil)
			})

			By("test RestorePVCFromSnapshot function")
			Expect(restoreMGR.RestorePVCFromSnapshot(reqCtx, k8sClient, *backupSet)).Should(Succeed())

			checkPVC(startingIndex, useVolumeSnapshot)
		})

		It("test with BuildPrepareDataJobs function and Parallel volumeRestorePolicy", func() {
			reqCtx := getReqCtx()
			startingIndex := 1
			restoreMGR, backupSet := initResources(reqCtx, startingIndex, false, func(f *testdp.MockRestoreFactory) {
				f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
					testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), map[string]string{
						constant.AppInstanceLabelKey: instanceName,
					})
			})

			By(fmt.Sprintf("test BuildPrepareDataJobs function, expect for %d jobs", replicas))
			actionSetName := "preparedata-0"
			jobs, err := restoreMGR.BuildPrepareDataJobs(reqCtx, k8sClient, *backupSet, actionSetName)
			Expect(err).ShouldNot(HaveOccurred())
			// job contains the pvc's label
			Expect(jobs[0].Spec.Template.Labels[constant.AppInstanceLabelKey]).Should(Equal(instanceName))
			Expect(len(jobs)).Should(Equal(replicas))
			// image should be expanded by env
			Expect(jobs[0].Spec.Template.Spec.Containers[0].Image).Should(ContainSubstring(testdp.ImageTag))

			checkPVC(startingIndex, false)
		})

		It("test with BuildPrepareDataJobs function and Serial volumeRestorePolicy", func() {
			reqCtx := getReqCtx()
			startingIndex := 1
			restoreMGR, backupSet := initResources(reqCtx, startingIndex, false, func(f *testdp.MockRestoreFactory) {
				f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
					testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), nil).
					SetVolumeClaimRestorePolicy(dpv1alpha1.VolumeClaimRestorePolicySerial)
			})

			actionSetName := "preparedata-0"
			testSerialCreateJob := func(expectRestoreFinished bool) {
				By("test BuildPrepareDataJobs function, expect for 1 job")
				jobs, err := restoreMGR.BuildPrepareDataJobs(reqCtx, k8sClient, *backupSet, actionSetName)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(jobs)).Should(Equal(1))

				By("test CreateJobsIfNotExist function")
				jobs, err = restoreMGR.CreateJobsIfNotExist(reqCtx, k8sClient, restoreMGR.Restore, jobs)
				Expect(err).ShouldNot(HaveOccurred())

				By("test CheckJobsDone function and jobs is running")
				allJobsFinished, existFailedJob := restoreMGR.CheckJobsDone(dpv1alpha1.PrepareData, actionSetName, *backupSet, jobs)
				Expect(allJobsFinished).Should(BeFalse())

				By("mock jobs are completed")
				jobCondition := batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}
				for i := range jobs {
					jobs[i].Status.Conditions = append(jobs[i].Status.Conditions, jobCondition)
				}

				By("test CheckJobsDone function and jobs are finished")
				allJobsFinished, existFailedJob = restoreMGR.CheckJobsDone(dpv1alpha1.PrepareData, actionSetName, *backupSet, jobs)
				Expect(allJobsFinished).Should(BeTrue())

				By("test Recalculation function, allJobFinished should be false because it only restored one pvc.")
				restoreMGR.Recalculation(backupSet.Backup.Name, actionSetName, &allJobsFinished, &existFailedJob)
				if expectRestoreFinished {
					Expect(allJobsFinished).Should(BeTrue())
				} else {
					Expect(allJobsFinished).Should(BeFalse())
				}
			}

			// expect for creating and finishing the first restore job but restore is continuing.
			testSerialCreateJob(false)

			// expect for creating and finishing the last one restore job and restore should be competed.
			testSerialCreateJob(true)

			By("test AnalysisRestoreActionsWithBackup function")
			allActionsFinished, _ := restoreMGR.AnalysisRestoreActionsWithBackup(dpv1alpha1.PrepareData, testdp.BackupName, actionSetName)
			Expect(allActionsFinished).Should(BeTrue())

		})

		It("test with BuildVolumePopulateJob function", func() {
			reqCtx := getReqCtx()
			restoreMGR, backupSet := initResources(reqCtx, 0, true, func(f *testdp.MockRestoreFactory) {
				f.SetDataSourceRef(testdp.DataVolumeName, testdp.DataVolumeMountPath)
			})

			By("test BuildVolumePopulateJob function, expect for 1 job")
			populatePVC := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-populate-pvc",
				},
			}
			job, err := restoreMGR.BuildVolumePopulateJob(reqCtx, k8sClient, *backupSet, populatePVC, 0)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(job).ShouldNot(BeNil())
		})

		testPostReady := func(existVolume bool) {
			kbNamespace := "kb-system"
			kbServiceAccountName := "kubeblocks"
			viper.Set(constant.CfgKeyCtrlrMgrNS, kbNamespace)
			viper.Set(constant.KBServiceAccountName, kbServiceAccountName)
			reqCtx := getReqCtx()
			matchLabels := map[string]string{
				constant.AppInstanceLabelKey: testdp.ClusterName,
			}
			restoreMGR, backupSet := initResources(reqCtx, 0, false, func(f *testdp.MockRestoreFactory) {
				f.SetConnectCredential(testdp.ClusterName).SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
			})

			By("create cluster to restore")
			testdp.NewFakeCluster(&testCtx)

			By("test with execAction and expect for creating 2 exec job")
			// step 0 is the execAction in actionSet
			jobs, err := restoreMGR.BuildPostReadyActionJobs(reqCtx, k8sClient, *backupSet, 0)
			Expect(err).ShouldNot(HaveOccurred())
			// the count of exec jobs should equal to the pods count of cluster
			Expect(len(jobs)).Should(Equal(2))
			Expect(jobs[0].Namespace).Should(Equal(kbNamespace))
			Expect(jobs[0].Spec.Template.Spec.ServiceAccountName).Should(Equal(kbServiceAccountName))

			By("test with jobAction and expect for creating 1 job")
			// step 0 is the execAction in actionSet
			jobs, err = restoreMGR.BuildPostReadyActionJobs(reqCtx, k8sClient, *backupSet, 1)
			Expect(err).ShouldNot(HaveOccurred())
			// count of job should equal to 1
			Expect(len(jobs)).Should(Equal(1))

			checkVolumes(jobs[0], testdp.DataVolumeName, existVolume)
		}

		It("test with BuildPostReadyActionJobs function and run target pod node", func() {
			testPostReady(true)
		})

		It("test with BuildPostReadyActionJobs function and no need to run target pod node", func() {
			Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
				runTargetPodNode := false
				actionSet.Spec.Restore.PostReady[1].Job.RunOnTargetPodNode = &runTargetPodNode
			})).Should(Succeed())
			testPostReady(false)
		})
	})

})
