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
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclocks "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Restore Controller test", func() {
	const namespace2 = "test2"

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		cleanNamespaced := func(namespace string) {
			// delete rest mocked objects
			inNS := client.InNamespace(namespace)

			// namespaced
			testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS, ml)
			testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)
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
		}

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)

		cleanNamespaced(testCtx.DefaultNamespace)
		cleanNamespaced(namespace2)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	ensureNamespace := func(name string) {
		Eventually(func(g Gomega) {
			obj := &corev1.Namespace{}
			obj.Name = name
			err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(obj), &corev1.Namespace{})
			if err == nil {
				return
			}
			g.Expect(client.IgnoreNotFound(err)).Should(Succeed())
			err = testCtx.Cli.Create(testCtx.Ctx, obj)
			g.Expect(err).Should(Succeed())
		}).Should(Succeed())
	}

	BeforeEach(func() {
		cleanEnv()
		ensureNamespace(namespace2)
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("restore controller test", func() {
		var (
			repo        *dpv1alpha1.BackupRepo
			repoPVCName string
			actionSet   *dpv1alpha1.ActionSet
			nodeName    = "minikube"
		)

		BeforeEach(func() {
			By("creating an actionSet")
			actionSet = testdp.NewFakeActionSet(&testCtx, nil)

			By("creating storage provider")
			_ = testdp.NewFakeStorageProvider(&testCtx, nil)

			By("creating a backupRepo")
			repo, repoPVCName = testdp.NewFakeBackupRepo(&testCtx, nil)
		})

		initResourcesAndWaitRestore := func(
			mockBackupCompleted,
			useVolumeSnapshot,
			isSerialPolicy bool,
			backupType dpv1alpha1.BackupType,
			expectRestorePhase dpv1alpha1.RestorePhase,
			change func(f *testdp.MockRestoreFactory),
			changeBackupStatus func(b *dpv1alpha1.Backup),
			backupNames ...string,
		) *dpv1alpha1.Restore {
			By("create a completed backup")
			backup := mockBackupForRestore(actionSet.Name, repo.Name, repoPVCName, mockBackupCompleted,
				useVolumeSnapshot, backupType, backupNames...)
			if changeBackupStatus != nil {
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					changeBackupStatus(backup)
				})).Should(Succeed())
			}
			By("create restore ")
			schedulingSpec := dpv1alpha1.SchedulingSpec{
				NodeName: nodeName,
			}
			restoreFactory := testdp.NewRestoreFactory(testCtx.DefaultNamespace, testdp.RestoreName).
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
				client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
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

		checkJobSA := func(restore *dpv1alpha1.Restore, saName string) {
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, v := range jobList.Items {
				Expect(v.Spec.Template.Spec.ServiceAccountName).WithOffset(1).
					Should(Equal(saName))
			}
		}

		mockRestoreJobsCompleted := func(restore *dpv1alpha1.Restore) {
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, v := range jobList.Items {
				testdp.PatchK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&v), batchv1.JobComplete)
			}
		}

		mockAndCheckRestoreCompleted := func(restore *dpv1alpha1.Restore) {
			By("mock jobs are completed")
			mockRestoreJobsCompleted(restore)

			By("wait for restore is completed")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
				g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseCompleted))
			})).Should(Succeed())
		}

		testRestoreWithVolumeClaimsTemplate := func(replicas, startingIndex int) {
			restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
				func(f *testdp.MockRestoreFactory) {
					f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
						testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), nil)
					// Note: should ignore this policy when podSelectionStrategy is Any of the source target.
					f.SetPrepareDataRequiredPolicy(dpv1alpha1.OneToOneRestorePolicy, "")
				}, nil)

			By("expect restore jobs and pvcs are created")
			checkJobAndPVCSCount(restore, replicas, replicas, startingIndex)

			checkJobSA(restore, viper.GetString(dptypes.CfgKeyWorkerServiceAccountName))

			By("mock jobs are completed and wait for restore is completed")
			mockAndCheckRestoreCompleted(restore)
		}
		checkJobParametersEnv := func(restore *dpv1alpha1.Restore) {

			By("check parameters env in restore jobs")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			for _, job := range jobList.Items {
				Expect(len(job.Spec.Template.Spec.Containers)).ShouldNot(BeZero())
				for _, c := range job.Spec.Template.Spec.Containers {
					if c.Name != dprestore.Restore {
						continue
					}
					count := 0
					for _, env := range c.Env {
						for _, param := range testdp.TestParameters {
							if param.Name == env.Name && param.Value == env.Value {
								count++
							}
						}
					}
					Expect(count).To(Equal(len(testdp.TestParameters)))
				}
			}
		}
		Context("with restore fails", func() {
			It("test restore is Failed when backup is not completed", func() {
				By("expect for restore is Failed ")
				initResourcesAndWaitRestore(false, false, false, "", dpv1alpha1.RestorePhaseFailed,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(3), int32(0), nil)
					}, nil)
			})

			It("test restore is failed when check failed in new action", func() {
				By("expect for restore is Failed")
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseFailed,
					func(f *testdp.MockRestoreFactory) {
						f.Get().Spec.Backup.Name = "wrongBackup"
					}, nil)
				By("check status.conditions")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					val := meta.IsStatusConditionFalse(r.Status.Conditions, dprestore.ConditionTypeRestoreCheckBackupRepo)
					g.Expect(val).Should(BeTrue())
				})).Should(Succeed())
			})

			It("test restore is failed when validate failed in new action", func() {
				By("expect for restore is Failed")
				restore := initResourcesAndWaitRestore(false, false, false, "", dpv1alpha1.RestorePhaseFailed, func(f *testdp.MockRestoreFactory) {
					f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
						testdp.DataVolumeMountPath, "", int32(3), int32(0), nil)
				}, nil)
				By("check status.conditions")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					val := meta.IsStatusConditionFalse(r.Status.Conditions, dprestore.ConditionTypeRestoreValidationPassed)
					g.Expect(val).Should(BeTrue())
				})).Should(Succeed())
			})

			It("fails the restore when the parallel recovery Jobs fail", func() {
				By("expect for restore is Failed ")
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(3), int32(0), nil)
					}, nil)

				By("wait for all parallel Jobs and PVCs")
				checkJobAndPVCSCount(restore, 3, 3, 0)

				By("mock restore job is Failed")
				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
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
			It("test restore parameters", func() {
				By("set schema and parameters in actionSet")
				testdp.MockActionSetWithSchema(&testCtx, actionSet)
				replicas := 3
				startingIndex := 0
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), nil)
						// Note: should ignore this policy when podSelectionStrategy is Any of the source target.
						f.SetPrepareDataRequiredPolicy(dpv1alpha1.OneToOneRestorePolicy, "")
						f.SetParameters(testdp.TestParameters)
					}, nil)

				By("expect restore jobs and pvcs are created")
				checkJobAndPVCSCount(restore, replicas, replicas, startingIndex)
				By("expect parameters env in restore jobs")
				checkJobParametersEnv(restore)
			})
			It("rejects Serial before creating recovery Jobs or target PVCs", func() {
				restore := initResourcesAndWaitRestore(true, false, true, "", dpv1alpha1.RestorePhaseFailed,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", 2, 1, nil)
					}, nil)
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), restore)).Should(Succeed())
				condition := meta.FindStatusCondition(restore.Status.Conditions, dprestore.ConditionTypeRestoreValidationPassed)
				Expect(condition).ShouldNot(BeNil())
				Expect(condition.Message).Should(ContainSubstring("Serial is no longer supported"))
				checkJobAndPVCSCount(restore, 0, 0, 1)
			})

			It("test dataSourceRef", func() {
				initResourcesAndWaitRestore(true, true, false, "", dpv1alpha1.RestorePhaseAsDataSource,
					func(f *testdp.MockRestoreFactory) {
						f.SetDataSourceRef(testdp.DataVolumeName, testdp.DataVolumeMountPath)
					}, nil)
			})

			It("test when dataRestorePolicy is OneToOne", func() {
				startingIndex := 0
				restoredReplicas := 2
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(restoredReplicas), int32(startingIndex), nil)
						f.SetPrepareDataRequiredPolicy(dpv1alpha1.OneToOneRestorePolicy, "")
					}, func(b *dpv1alpha1.Backup) {
						b.Status.Target.PodSelector.Strategy = dpv1alpha1.PodSelectionStrategyAll
						b.Status.Target.SelectedTargetPods = []string{"pod-0", "pod-1"}
					})

				By("wait to create two jobs and pvcs")
				checkJobAndPVCSCount(restore, restoredReplicas, restoredReplicas, 0)

				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
				for _, v := range jobList.Items {
					var checkBackupBasePathPass bool
					index := v.Name[strings.LastIndex(v.Name, "-")+1:]
					// checks if the backupBasePath exits
					for _, env := range v.Spec.Template.Spec.Containers[0].Env {
						if env.Name == dptypes.DPBackupBasePath && strings.Contains(env.Value, fmt.Sprintf("pod-%s", index)) {
							checkBackupBasePathPass = true
							break
						}
					}
					Expect(checkBackupBasePathPass).Should(BeTrue())
				}

				By("mock jobs are completed and wait for restore is completed")
				mockAndCheckRestoreCompleted(restore)
			})

			It("test when dataRestorePolicy is OneToMany and sourceTargetPod is pod-0", func() {
				startingIndex := 0
				restoredReplicas := 2
				sourcePodName := "pod-0"
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(restoredReplicas), int32(startingIndex), nil)
						f.SetPrepareDataRequiredPolicy(dpv1alpha1.OneToManyRestorePolicy, sourcePodName)
					}, func(b *dpv1alpha1.Backup) {
						b.Status.Target.PodSelector.Strategy = dpv1alpha1.PodSelectionStrategyAll
						b.Status.Target.SelectedTargetPods = []string{sourcePodName, "pod-1"}
					})

				By("wait to create two jobs and pvcs")
				checkJobAndPVCSCount(restore, restoredReplicas, restoredReplicas, 0)
				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
				for _, v := range jobList.Items {
					var checkBackupBasePathPass bool
					// checks if the backupBasePath exits
					for _, env := range v.Spec.Template.Spec.Containers[0].Env {
						if env.Name == dptypes.DPBackupBasePath && strings.Contains(env.Value, sourcePodName) {
							checkBackupBasePathPass = true
							break
						}
					}
					Expect(checkBackupBasePathPass).Should(BeTrue())
				}

				By("mock jobs are completed and wait for restore is completed")
				mockAndCheckRestoreCompleted(restore)
			})

		})

		Context("test postReady stage", func() {
			var _ *testdp.BackupClusterInfo
			BeforeEach(func() {
				By("fake a new cluster")
				_ = testdp.NewFakeCluster(&testCtx)
				testapps.NewComponentFactory(testCtx.DefaultNamespace,
					constant.GenerateClusterComponentName(testdp.ClusterName, testdp.ComponentName), "test-cmpd").
					SetServiceVersion("8.0.30").
					Create(&testCtx)
			})

			It("test post ready actions", func() {
				By("remove the prepareData stage for testing post ready actions")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
					set.Spec.Restore.PrepareData = nil
				})).Should(Succeed())

				matchLabels := map[string]string{
					constant.AppInstanceLabelKey: testdp.ClusterName,
				}
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetConnectCredential(testdp.ClusterName).SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
					}, nil)

				By("wait for creating two exec jobs with the matchLabels")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(2))

				checkJobSA(restore, viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName))

				By("mock exec jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for creating a job of jobAction with the matchLabels, expect jobs count is 3(2+1)")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(3))

				By("mock jobs are completed and wait for restore is completed")
				mockAndCheckRestoreCompleted(restore)

				By("test deleting restore")
				Expect(k8sClient.Delete(ctx, restore)).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(restore), restore, false)).Should(Succeed())
			})

			It("should complete an existing postReady job when target pod is no longer ready", func() {
				By("remove the prepareData stage for testing post ready actions")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
					set.Spec.Restore.PrepareData = nil
				})).Should(Succeed())

				matchLabels := map[string]string{
					constant.AppInstanceLabelKey: testdp.ClusterName,
				}
				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetConnectCredential(testdp.ClusterName).SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
					}, nil)

				By("wait for creating two exec jobs with the matchLabels")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(2))

				By("mock exec jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for creating a job of jobAction with the matchLabels")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(3))

				By("make the target pods unavailable before the existing postReady job is observed as complete")
				podList := &corev1.PodList{}
				Expect(k8sClient.List(ctx, podList,
					client.MatchingLabels(matchLabels),
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
				Expect(podList.Items).ShouldNot(BeEmpty())
				for i := range podList.Items {
					pod := &podList.Items[i]
					Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:               corev1.PodReady,
							Status:             corev1.ConditionFalse,
							LastTransitionTime: metav1.Now(),
						}}
					})).Should(Succeed())
				}

				By("expect the existing postReady job is tracked before rebuilding from unavailable target pods")
				Consistently(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(3))

				By("mock all postReady jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for restore to complete from the existing job status")
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(restore), func(g Gomega, r *dpv1alpha1.Restore) {
					g.Expect(r.Status.Phase).Should(Equal(dpv1alpha1.RestorePhaseCompleted))
				})).Should(Succeed())
			})

			It("test jobAction env", func() {
				By("remove the prepareData stage for testing post ready actions")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
					set.Spec.Restore.PrepareData = nil
				})).Should(Succeed())

				matchLabels := map[string]string{
					constant.AppInstanceLabelKey: testdp.ClusterName,
				}

				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
					}, func(b *dpv1alpha1.Backup) {
						b.Status.Target.ConnectionCredential = nil
					})

				getJobKey := func(jobIndex int) client.ObjectKey {
					return client.ObjectKey{
						Name:      fmt.Sprintf("restore-post-ready-%s-%s-%d-%d", restore.UID[:8], restore.Spec.Backup.Name, 1, jobIndex),
						Namespace: restore.Namespace,
					}
				}

				getDPDBPortEnv := func(container *corev1.Container) corev1.EnvVar {
					for _, env := range container.Env {
						if env.Name == dptypes.DPDBPort {
							return env
						}
					}
					return corev1.EnvVar{}
				}
				By("wait for creating two exec jobs with the matchLabels")
				a := testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))
				Eventually(a).Should(HaveLen(2))

				checkJobSA(restore, viper.GetString(dptypes.CfgKeyExecWorkerServiceAccountName))

				By("mock exec jobs are completed")
				mockRestoreJobsCompleted(restore)

				By("wait for creating a job of jobAction with the matchLabels, expect jobs count is 3(2+1)")
				Eventually(testapps.List(&testCtx, generics.JobSignature,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(3))

				By("check backup job's port env")
				Eventually(testapps.CheckObj(&testCtx, getJobKey(0), func(g Gomega, fetched *batchv1.Job) {
					g.Expect(getDPDBPortEnv(&fetched.Spec.Template.Spec.Containers[0]).Value).Should(Equal(strconv.Itoa(testdp.PortNum)))
				})).Should(Succeed())

			})
			It("test parameters env", func() {
				By("set schema and parameters in actionSet")
				testdp.MockActionSetWithSchema(&testCtx, actionSet)
				By("remove the prepareData stage for testing post ready actions")
				Expect(testapps.ChangeObj(&testCtx, actionSet, func(set *dpv1alpha1.ActionSet) {
					set.Spec.Restore.PrepareData = nil
				})).Should(Succeed())

				matchLabels := map[string]string{
					constant.AppInstanceLabelKey: testdp.ClusterName,
				}

				restore := initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetJobActionConfig(matchLabels).SetExecActionConfig(matchLabels)
						f.SetParameters(testdp.TestParameters)
					}, func(b *dpv1alpha1.Backup) {
						b.Status.Target.ConnectionCredential = nil
					})
				By("expect parameters env in restore jobs")
				checkJobParametersEnv(restore)
			})
		})

		Context("test cross namespace", func() {
			It("should wait for preparation of backup repo", func() {
				By("creating a restore in a different namespace from backup")
				initResourcesAndWaitRestore(true, false, false, "", dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetNamespace(namespace2)
					}, nil)
			})
		})

		Context("test restore from incremental backup", func() {
			var (
				baseBackup       *dpv1alpha1.Backup
				parentBackupName string
				ancestorBackups  = []*dpv1alpha1.Backup{}
				cnt              = 0
				testClock        = testclocks.NewFakeClock(time.Now().Add(time.Hour))
			)

			genIncBackupName := func() string {
				cnt++
				return fmt.Sprintf("inc-backup-%d", cnt)
			}

			changeTimeRange := func(backup *dpv1alpha1.Backup) {
				backup.Status.TimeRange = &dpv1alpha1.BackupTimeRange{
					Start: &metav1.Time{},
					End:   &metav1.Time{},
				}
				// testClock is only used to mock timestamps in ascending order, Step() don't affect the real or logic time of test
				testClock.Step(time.Minute)
				backup.Status.TimeRange.Start.Time = testClock.Now()
				testClock.Step(time.Minute)
				backup.Status.TimeRange.End.Time = testClock.Now()
			}

			BeforeEach(func() {
				By("mock completed full backup and parent incremental backup")
				baseBackup = mockBackupForRestore(actionSet.Name, repo.Name, repoPVCName, true, false, dpv1alpha1.BackupTypeFull)
				Expect(testapps.ChangeObjStatus(&testCtx, baseBackup, func() { changeTimeRange(baseBackup) })).Should(Succeed())
				actionSet = testdp.NewFakeIncActionSet(&testCtx)
				parentBackupName = baseBackup.Name
				for i := 0; i < 3; i++ {
					backup := mockBackupForRestore(actionSet.Name, repo.Name, repoPVCName, true, false, dpv1alpha1.BackupTypeIncremental,
						genIncBackupName(), parentBackupName, baseBackup.Name)
					Expect(testapps.ChangeObjStatus(&testCtx, backup, func() { changeTimeRange(backup) })).Should(Succeed())
					ancestorBackups = append(ancestorBackups, backup)
					parentBackupName = backup.Name
				}
			})

			AfterEach(func() {
				ancestorBackups = []*dpv1alpha1.Backup{}
				cnt = 0
			})

			It("test restore from incremental backup", func() {
				replicas, startingIndex := 3, 0
				restore := initResourcesAndWaitRestore(true, false, false, dpv1alpha1.BackupTypeIncremental, dpv1alpha1.RestorePhaseRunning,
					func(f *testdp.MockRestoreFactory) {
						f.SetVolumeClaimsTemplate(testdp.MysqlTemplateName, testdp.DataVolumeName,
							testdp.DataVolumeMountPath, "", int32(replicas), int32(startingIndex), nil)
						f.SetPrepareDataRequiredPolicy(dpv1alpha1.OneToOneRestorePolicy, "")
					}, changeTimeRange, genIncBackupName(), parentBackupName, baseBackup.Name)

				By("wait for creating jobs and pvcs")
				checkJobAndPVCSCount(restore, replicas, replicas, 0)
				By("check job env")
				ancestorIncrementalBackupNames := []string{}
				for _, backup := range ancestorBackups {
					ancestorIncrementalBackupNames = append(ancestorIncrementalBackupNames, backup.Name)
				}
				expectedEnv := map[string]string{
					dptypes.DPAncestorIncrementalBackupNames: strings.Join(ancestorIncrementalBackupNames, ","),
					dptypes.DPBaseBackupName:                 baseBackup.Name,
				}
				jobList := &batchv1.JobList{}
				Expect(k8sClient.List(ctx, jobList,
					client.MatchingLabels{dprestore.DataProtectionRestoreLabelKey: restore.Name},
					client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
				for _, job := range jobList.Items {
					cnt := 0
					for _, env := range job.Spec.Template.Spec.Containers[0].Env {
						if value, ok := expectedEnv[env.Name]; ok {
							Expect(env.Value).Should(Equal(value))
							cnt++
						}
					}
					Expect(cnt).To(Equal(len(expectedEnv)))
				}
				By("mock jobs are completed and wait for restore is completed")
				mockAndCheckRestoreCompleted(restore)
			})
		})
	})
})

func mockBackupForRestore(
	actionSetName, repoName, backupPVCName string,
	mockBackupCompleted, useVolumeSnapshotBackup bool,
	backupType dpv1alpha1.BackupType,
	backupNames ...string,
) *dpv1alpha1.Backup {
	backup := testdp.NewFakeBackup(&testCtx, func(backup *dpv1alpha1.Backup) {
		if len(backupNames) > 0 {
			backup.Name = backupNames[0]
		}
		if backupType == dpv1alpha1.BackupTypeIncremental {
			if len(backupNames) > 1 {
				backup.Spec.ParentBackupName = backupNames[1]
			}
			backup.Spec.BackupMethod = testdp.IncBackupMethodName
		}
	})
	// wait for backup is failed by backup controller.
	// it will be failed if the backupPolicy is not created.
	Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
		g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupPhaseFailed))
	})).Should(Succeed())

	if mockBackupCompleted {
		// then mock backup to completed
		Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
			backupMethodName := testdp.BackupMethodName
			if useVolumeSnapshotBackup {
				backupMethodName = testdp.VSBackupMethodName
				testdp.MockBackupVSStatusActions(backup)
			}
			if backupType == dpv1alpha1.BackupTypeIncremental {
				backupMethodName = testdp.IncBackupMethodName
				if len(backupNames) > 2 {
					backup.Status.ParentBackupName = backupNames[1]
					backup.Status.BaseBackupName = backupNames[2]
				}
			}
			backup.Status.Path = "/backup-data" + "/" + backup.Name
			backup.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			backup.Status.BackupRepoName = repoName
			backup.Status.PersistentVolumeClaimName = backupPVCName
			testdp.MockBackupStatusTarget(backup, dpv1alpha1.PodSelectionStrategyAny)
			backup.Status.Target.ContainerPort = &dpv1alpha1.ContainerPort{
				ContainerName: testdp.ContainerName + "-1",
				PortName:      testdp.PortName,
			}
			testdp.MockBackupStatusMethod(backup, backupMethodName, testdp.DataVolumeName, actionSetName)
		})).Should(Succeed())
	}
	return backup
}

func policyTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	return scheme
}

func serialRestore() *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "default", UID: "restore-uid", Finalizers: []string{dptypes.DataProtectionFinalizerName}},
		Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}, PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
			VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
		}},
	}
}

func TestRestoreRejectsSerialBeforePreparingDependencies(t *testing.T) {
	for _, dataSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "dataSource"}[dataSource], func(t *testing.T) {
			restore := serialRestore()
			if dataSource {
				restore.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
			}
			scheme := policyTestScheme(t)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(restore).Build()
			r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
			_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
			require.NoError(t, err)
			require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), restore))
			require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
			condition := meta.FindStatusCondition(restore.Status.Conditions, dprestore.ConditionTypeRestoreValidationPassed)
			require.NotNil(t, condition)
			require.Contains(t, condition.Message, "Serial is no longer supported")
			jobs := &batchv1.JobList{}
			require.NoError(t, cli.List(context.Background(), jobs))
			require.Empty(t, jobs.Items)
			pvcs := &corev1.PersistentVolumeClaimList{}
			require.NoError(t, cli.List(context.Background(), pvcs))
			require.Empty(t, pvcs.Items)
		})
	}
}

func TestSerialRestoreDrainsUnrecordedJobAcrossReconciles(t *testing.T) {
	ctx := context.Background()
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: "existing-job", Namespace: restore.Namespace, UID: "job-uid",
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(restore, dpv1alpha1.GroupVersion.WithKind("Restore"))},
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "worker", Namespace: restore.Namespace,
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))},
	}, Status: corev1.PodStatus{Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
	}}
	pod.Labels["job-name"] = job.Name
	data := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-0", Namespace: restore.Namespace}}
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, job, pod).
		WithObjects(restore, job, pod, data).Build()
	reconcile := func() {
		// A fresh reconciler each time must recover entirely from persisted facts.
		r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
		require.NoError(t, err)
		require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	}
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	require.Contains(t, meta.FindStatusCondition(restore.Status.Conditions, "ValidationPassed").Message, "waiting for existing")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Empty(t, pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey], "do not stop a writing container")
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
	require.NoError(t, cli.Status().Update(ctx, pod))
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
	pod.Status.Phase = corev1.PodSucceeded
	require.NoError(t, cli.Status().Update(ctx, pod))
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	require.NoError(t, cli.Status().Update(ctx, job))
	reconcile()
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
	require.NotNil(t, restore.Status.CompletionTimestamp)
	jobs := &batchv1.JobList{}
	require.NoError(t, cli.List(ctx, jobs))
	require.Len(t, jobs.Items, 1, "never create the next Serial Job")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(data), data), "preserve recovered data")
}

func TestPostReadyRejectsInheritedSerialBeforeStartingJobs(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default", UID: "cluster-uid"},
		Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{Parameters: map[string]string{dptypes.VolumeRestorePolicyParameterKey: "Serial"}}}}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "cluster-mysql", Namespace: cluster.Namespace, UID: "component-uid"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Namespace: cluster.Namespace, Labels: map[string]string{
		constant.AppInstanceLabelKey: cluster.Name, constant.KBAppComponentLabelKey: "mysql", dptypes.ClusterUIDLabelKey: string(cluster.UID),
	}}}
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Name: postReadyRestoreName(comp.UID), Namespace: cluster.Namespace, Labels: postReadyRestoreLabels(pvc, comp),
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind, Name: comp.Name, UID: comp.UID}},
		Finalizers:      []string{dptypes.DataProtectionFinalizerName},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(cluster, comp, restore).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
	jobs := &batchv1.JobList{}
	require.NoError(t, cli.List(ctx, jobs))
	require.Empty(t, jobs.Items)
}

func TestCompletedSerialRestoreKeepsHistoricalResult(t *testing.T) {
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore).WithObjects(restore).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseCompleted, restore.Status.Phase)
	require.Nil(t, meta.FindStatusCondition(restore.Status.Conditions, "ValidationPassed"))
}

func TestSerialRestoreWaitsForPodAfterJobDeletion(t *testing.T) {
	ctx := context.Background()
	restore := serialRestore()
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	jobName := "restore-preparedata-" + string(restore.UID[:8]) + "-backup-0"
	controller := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "orphan-worker", Namespace: restore.Namespace,
		Labels:          dprestore.BuildRestoreLabels(restore.Name),
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: jobName, UID: "deleted-job-uid", Controller: &controller}},
	}, Status: corev1.PodStatus{Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}},
	}}
	scheme := policyTestScheme(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, pod).WithObjects(restore, pod).Build()
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)}
	result, err := r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.Positive(t, result.RequeueAfter)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseRunning, restore.Status.Phase)
	pod.Status.Phase = corev1.PodSucceeded
	require.NoError(t, cli.Status().Update(ctx, pod))
	_, err = r.Reconcile(ctx, request)
	require.NoError(t, err)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), restore))
	require.Equal(t, dpv1alpha1.RestorePhaseFailed, restore.Status.Phase)
}

func TestSerialRestoreDrainsActualCrossNamespaceExecJob(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	oldNamespace := viper.Get(constant.CfgKeyCtrlrMgrNS)
	viper.Set(constant.CfgKeyCtrlrMgrNS, "operator")
	t.Cleanup(func() { viper.Set(constant.CfgKeyCtrlrMgrNS, oldNamespace) })
	restore := serialRestore()
	restore.Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{ExecAction: &dpv1alpha1.ExecAction{Target: dpv1alpha1.ExecActionTarget{
		PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"target": "true"}},
	}}}
	restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
	repo := &dpv1alpha1.BackupRepo{ObjectMeta: metav1.ObjectMeta{Name: "repo"}}
	target := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: restore.Namespace, Labels: map[string]string{"target": "true"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "database"}}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(restore, &batchv1.Job{}, &corev1.Pod{}).
		WithObjects(restore, repo, target).Build()
	backupSet := dprestore.BackupActionSet{
		Backup: &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup"}, Status: dpv1alpha1.BackupStatus{BackupRepoName: repo.Name}},
		ActionSet: &dpv1alpha1.ActionSet{Spec: dpv1alpha1.ActionSetSpec{Restore: &dpv1alpha1.RestoreActionSpec{PostReady: []dpv1alpha1.ActionSpec{{
			Exec: &dpv1alpha1.ExecActionSpec{Command: []string{"true"}},
		}}}}},
	}
	mgr := dprestore.NewRestoreManager(restore, record.NewFakeRecorder(20), scheme, cli)
	jobs, err := mgr.BuildPostReadyActionJobs(intctrlutil.RequestCtx{Ctx: ctx, Req: ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)}}, cli, backupSet, nil, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	job := jobs[0]
	job.UID = "exec-job-uid"
	require.Equal(t, "operator", job.Namespace)
	require.NoError(t, cli.Create(ctx, job))
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "exec-worker", Namespace: job.Namespace,
		Labels: job.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "restore", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}}}
	pod.Labels["job-name"] = job.Name
	require.NoError(t, cli.Create(ctx, pod))
	r := &RestoreReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(restore)})
	require.NoError(t, err)
	require.Positive(t, result.RequeueAfter, "the Exec Job has not finished")
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	require.Equal(t, "true", pod.Annotations[dprestore.DataProtectionStopRestoreManagerAnnotationKey])
}

func TestExecutionPolicyCheckAllowsMissingSourceRestore(t *testing.T) {
	ctx := context.Background()
	scheme := policyTestScheme(t)
	source := serialRestore()
	source.Spec.PrepareDataConfig.VolumeClaimRestorePolicy = dpv1alpha1.VolumeClaimRestorePolicyParallel
	source.Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{VolumeSource: "data"}
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "default", UID: "target-uid"},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{APIGroup: &apiGroup, Kind: dptypes.RestoreKind, Name: source.Name}}}
	helper := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: getPopulatePVCName(pvc.UID), Namespace: pvc.Namespace}}
	child := buildExecutionRestore(pvc, helper, source)
	child.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: pvc.Name, UID: pvc.UID}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	require.NoError(t, executionRestorePolicyError(ctx, cli, child), "a created execution Restore must not depend on the source Restore still existing")
}
