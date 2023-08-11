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
	appsv1 "k8s.io/api/apps/v1"

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Backup Policy Controller", func() {
	const clusterName = "wesql-cluster"
	const componentName = "replicasets-primary"
	const containerName = "mysql"
	const defaultPVCSize = "1Gi"
	const backupPolicyName = "test-backup-policy"
	const backupRemotePVCName = "backup-remote-pvc"
	const defaultSchedule = "0 3 * * *"
	const defaultTTL = "7d"
	const backupNamePrefix = "test-backup-job-"
	const mgrNamespace = "kube-system"

	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		viper.SetDefault(constant.CfgKeyCtrlrMgrNS, mgrNamespace)
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)

		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.JobSignature, true, inNS)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.SecretSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS)
		// mgr namespaced
		inMgrNS := client.InNamespace(mgrNamespace)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inMgrNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

		By("By mocking a statefulset")
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, clusterName+"-"+componentName, clusterName, componentName).
			AddAppInstanceLabel(clusterName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC(defaultPVCSize),
			}).Create(&testCtx).GetObject()

		By("By mocking a pod belonging to the statefulset")
		pod := testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddAppInstanceLabel(clusterName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			Create(&testCtx).GetObject()

		By("By mocking a pvc belonging to the pod")
		_ = testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+pod.Name, clusterName, componentName, "data").
			SetStorage("1Gi").
			Create(&testCtx)
	})

	AfterEach(cleanEnv)

	When("creating backup policy with default settings", func() {
		var backupToolName string
		getCronjobKey := func(backupType dpv1alpha1.BackupType) types.NamespacedName {
			return types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s-%s", backupPolicyName, testCtx.DefaultNamespace, backupType),
				Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
			}
		}

		BeforeEach(func() {
			viper.Set(constant.CfgKeyCtrlrMgrNS, mgrNamespace)
			viper.Set(constant.CfgKeyCtrlrMgrAffinity,
				"{\"nodeAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"preference\":{\"matchExpressions\":[{\"key\":\"kb-controller\",\"operator\":\"In\",\"values\":[\"true\"]}]},\"weight\":100}]}}")
			viper.Set(constant.CfgKeyCtrlrMgrTolerations,
				"[{\"key\":\"key1\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\"}]")
			viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "{\"beta.kubernetes.io/arch\":\"amd64\"}")

			By("By creating a backupTool")
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dpv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			backupToolName = backupTool.Name
		})

		AfterEach(func() {
			viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
			viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
			viper.Set(constant.CfgKeyCtrlrMgrTolerations, "")
			viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "")
		})

		Context("creates a backup policy", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			var startingDeadlineMinutes int64 = 60
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddDataFilePolicy().
					SetBackupToolName(backupToolName).
					SetBackupsHistoryLimit(1).
					SetSchedule(defaultSchedule, true).
					SetScheduleStartingDeadlineMinutes(&startingDeadlineMinutes).
					SetTTL(defaultTTL).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetPVC(backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, getCronjobKey(dpv1alpha1.BackupTypeDataFile), func(g Gomega, fetched *batchv1.CronJob) {
					g.Expect(fetched.Spec.Schedule).To(Equal(defaultSchedule))
					g.Expect(fetched.Spec.JobTemplate.Spec.Template.Spec.Tolerations).ShouldNot(BeEmpty())
					g.Expect(fetched.Spec.JobTemplate.Spec.Template.Spec.NodeSelector).ShouldNot(BeEmpty())
					g.Expect(fetched.Spec.JobTemplate.Spec.Template.Spec.Affinity).ShouldNot(BeNil())
					g.Expect(fetched.Spec.JobTemplate.Spec.Template.Spec.Affinity.NodeAffinity).ShouldNot(BeNil())
					g.Expect(fetched.Spec.StartingDeadlineSeconds).ShouldNot(BeNil())
					g.Expect(*fetched.Spec.StartingDeadlineSeconds).Should(Equal(startingDeadlineMinutes * 60))
				})).Should(Succeed())
			})
			It("limit backups to 1", func() {
				now := metav1.Now()
				backupStatus := dpv1alpha1.BackupStatus{
					Phase:               dpv1alpha1.BackupCompleted,
					Expiration:          &now,
					StartTimestamp:      &now,
					CompletionTimestamp: &now,
				}

				autoBackupLabel := map[string]string{
					dataProtectionLabelAutoBackupKey:   "true",
					dataProtectionLabelBackupPolicyKey: backupPolicyName,
					dataProtectionLabelBackupTypeKey:   string(dpv1alpha1.BackupTypeDataFile),
				}

				By("create a expired backup")
				backupExpired := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeDataFile).
					Create(&testCtx).GetObject()
				By("create 1st limit backup")
				backupOutLimit1 := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeDataFile).
					Create(&testCtx).GetObject()
				By("create 2nd limit backup")
				backupOutLimit2 := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeDataFile).
					Create(&testCtx).GetObject()

				By("waiting expired backup completed")
				backupExpiredKey := client.ObjectKeyFromObject(backupExpired)
				patchK8sJobStatus(backupExpiredKey, batchv1.JobComplete)
				Eventually(testapps.CheckObj(&testCtx, backupExpiredKey,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
				By("mock update expired backup status to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(-time.Hour * 24)}
				backupStatus.StartTimestamp = backupStatus.Expiration
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupExpired))

				By("waiting 1st limit backup completed")
				backupOutLimit1Key := client.ObjectKeyFromObject(backupOutLimit1)
				patchK8sJobStatus(backupOutLimit1Key, batchv1.JobComplete)
				Eventually(testapps.CheckObj(&testCtx, backupOutLimit1Key,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
				By("mock update 1st limit backup NOT to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit1))

				By("waiting 2nd limit backup completed")
				backupOutLimit2Key := client.ObjectKeyFromObject(backupOutLimit2)
				patchK8sJobStatus(backupOutLimit2Key, batchv1.JobComplete)
				Eventually(testapps.CheckObj(&testCtx, backupOutLimit2Key,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
				By("mock update 2nd limit backup NOT to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour * 2)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit2))

				// trigger the backup policy controller through update cronjob
				patchCronJobStatus(getCronjobKey(dpv1alpha1.BackupTypeDataFile))

				By("retain the latest backup")
				Eventually(testapps.List(&testCtx, intctrlutil.BackupSignature,
					client.MatchingLabels(backupPolicy.Spec.Datafile.Target.LabelsSelector.MatchLabels),
					client.InNamespace(backupPolicy.Namespace))).Should(HaveLen(1))
			})
		})

		Context("creates a backup policy with empty schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetPVC(backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with invalid schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddSnapshotPolicy().
					SetBackupToolName(backupToolName).
					SetSchedule("invalid schedule", true).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetPVC(backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should failed", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).NotTo(Equal(dpv1alpha1.PolicyAvailable))
				})).Should(Succeed())
			})
		})

		Context("creating a backupPolicy with secret", func() {
			It("creating a backupPolicy with secret", func() {
				By("By creating a backupPolicy with empty secret")
				randomSecretName := testCtx.GetRandomStr()
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddDataFilePolicy().
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(randomSecretName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetPVC(backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
					g.Expect(fetched.Spec.Datafile.Target.Secret.Name).To(Equal(randomSecretName))
				})).Should(Succeed())
			})
		})

		Context("creating a backupPolicy with global backup config", func() {
			It("creating a backupPolicy with global backup config", func() {
				By("By creating a backupPolicy with empty secret")
				pvcName := "backup-data"
				pvcInitCapacity := "10Gi"
				viper.SetDefault(constant.CfgKeyBackupPVCName, pvcName)
				viper.SetDefault(constant.CfgKeyBackupPVCInitCapacity, pvcInitCapacity)
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddDataFilePolicy().
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
					g.Expect(fetched.Spec.Datafile.PersistentVolumeClaim.Name).ToNot(BeNil())
					g.Expect(*fetched.Spec.Datafile.PersistentVolumeClaim.Name).To(Equal(pvcName))
					g.Expect(fetched.Spec.Datafile.PersistentVolumeClaim.InitCapacity.String()).To(Equal(pvcInitCapacity))
				})).Should(Succeed())
			})
		})
		Context("reconcile a logfile backupPolicy", func() {
			It("with reconfigure config and job deployKind", func() {
				By("creating a backupPolicy")
				pvcName := "backup-data"
				pvcInitCapacity := "10Gi"
				viper.SetDefault(constant.CfgKeyBackupPVCName, pvcName)
				viper.SetDefault(constant.CfgKeyBackupPVCInitCapacity, pvcInitCapacity)
				reconfigureRef := `{
					"name": "postgresql-configuration",
					"key": "postgresql.conf",
					"enable": {
					  "logfile": [{"key":"archive_command","value":"''"}]
					},
					"disable": {
					  "logfile": [{"key": "archive_command","value":"'/bin/true'"}]
					}
				  }`
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddAnnotations(constant.ReconfigureRefAnnotationKey, reconfigureRef).
					AddLogfilePolicy().
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					AddSnapshotPolicy().
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
				})).Should(Succeed())
				By("enable schedule for reconfigure")
				Eventually(testapps.GetAndChangeObj(&testCtx, backupPolicyKey, func(fetched *dpv1alpha1.BackupPolicy) {
					fetched.Spec.Schedule.Logfile = &dpv1alpha1.SchedulePolicy{Enable: true, CronExpression: "* * * * *"}
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Annotations[constant.LastAppliedConfigAnnotationKey]).To(Equal(`[{"key":"archive_command","value":"''"}]`))
				})).Should(Succeed())

				By("disable schedule for reconfigure")
				Eventually(testapps.GetAndChangeObj(&testCtx, backupPolicyKey, func(fetched *dpv1alpha1.BackupPolicy) {
					fetched.Spec.Schedule.Logfile.Enable = false
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Annotations[constant.LastAppliedConfigAnnotationKey]).To(Equal(`[{"key":"archive_command","value":"'/bin/true'"}]`))
				})).Should(Succeed())
			})

			It("test logfile backup with a statefulSet deployKind", func() {

				// mock a backupTool
				backupTool := createStatefulKindBackupTool()

				testLogfileBackupWithStatefulSet := func() {
					By("init test resources")
					// mock a cluster
					cluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
						"test-cd", "test-cv").Create(&testCtx).GetObject()
					// mock a backupPolicy
					backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
						SetOwnerReferences("apps.kubeblocks.io/v1alpha1", "Cluster", cluster).
						AddLogfilePolicy().
						SetTTL("7d").
						SetSchedule("*/1 * * * *", false).
						SetBackupToolName(backupTool.Name).
						SetPVC(backupRemotePVCName).
						AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
						Create(&testCtx).GetObject()

					By("enable logfile schedule, expect for backup and statefulSet creation")
					Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.Schedule.Logfile.Enable = true
					})).Should(Succeed())
					backup := &dpv1alpha1.Backup{}
					sts := &appsv1.StatefulSet{}
					backupName := getCreatedCRNameByBackupPolicy(backupPolicy, dpv1alpha1.BackupTypeLogFile)
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
						backup = tmpBackup
						g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupRunning))
					})).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, sts, true)).Should(Succeed())

					By("check the container envs which is injected successfully.")
					expectedEnv := map[string]string{
						constant.DPArchiveInterval:  "60s",
						constant.DPTTL:              "7d",
						constant.DPLogfileTTL:       "192h",
						constant.DPLogfileTTLSecond: "691200",
					}
					checkGenerateENV := func(sts *appsv1.StatefulSet) {
						mainContainer := sts.Spec.Template.Spec.Containers[0]
						for k, v := range expectedEnv {
							for _, env := range mainContainer.Env {
								if env.Name != k {
									continue
								}
								Expect(env.Value).Should(Equal(v))
								break
							}
						}
					}
					checkGenerateENV(sts)

					By("update cronExpression, expect for noticing backup to reconcile")
					Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.Schedule.Logfile.CronExpression = "*/2 * * * *"
						ttl := "2h"
						backupPolicy.Spec.Retention.TTL = &ttl
					})).Should(Succeed())
					// waiting for sts has changed and expect sts env to change to the corresponding value
					expectedEnv = map[string]string{
						constant.DPArchiveInterval:  "120s",
						constant.DPTTL:              "2h",
						constant.DPLogfileTTL:       "26h",
						constant.DPLogfileTTLSecond: "93600",
					}
					oldStsGeneration := sts.Generation
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpSts *appsv1.StatefulSet) {
						g.Expect(tmpSts.Generation).Should(Equal(oldStsGeneration + 1))
						checkGenerateENV(tmpSts)
					})).Should(Succeed())

					By("expect to recreate the backup after delete the backup during enable logfile")
					Expect(testapps.ChangeObj(&testCtx, backup, func(policy *dpv1alpha1.Backup) {
						backup.Finalizers = []string{}
					})).Should(Succeed())
					testapps.DeleteObject(&testCtx, client.ObjectKeyFromObject(backup), backup)
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
						g.Expect(tmpBackup.Generation).Should(Equal(int64(1)))
						g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupRunning))
					})).Should(Succeed())

					By("disable logfile, expect the backup phase to Completed and sts is deleted")
					Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.Schedule.Logfile.Enable = false
					})).Should(Succeed())
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
						g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, sts, false)).Should(Succeed())

					By("enable logfile schedule, expect to re-create backup ")
					Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.Schedule.Logfile.Enable = true
					})).Should(Succeed())
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
						g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupRunning))
					})).Should(Succeed())

					By("delete cluster, expect the backup phase to Completed")
					testapps.DeleteObject(&testCtx, types.NamespacedName{
						Name:      clusterName,
						Namespace: testCtx.DefaultNamespace,
					}, &appsv1alpha1.Cluster{})
					Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{
						Name:      backupName,
						Namespace: testCtx.DefaultNamespace,
					}, func(g Gomega, tmpBackup *dpv1alpha1.Backup) {
						g.Expect(tmpBackup.Status.Phase).Should(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())

					// disabled logfile
					Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(policy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.Schedule.Logfile.Enable = false
					})).Should(Succeed())
				}

				testLogfileBackupWithStatefulSet()

				// clear backupPolicy
				testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, client.InNamespace(testCtx.DefaultNamespace),
					client.HasLabels{testCtx.TestObjLabelKey})

				// test again for create a cluster with same name
				testLogfileBackupWithStatefulSet()

			})
		})
	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}

func patchCronJobStatus(key types.NamespacedName) {
	now := metav1.Now()
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *batchv1.CronJob) {
		fetched.Status = batchv1.CronJobStatus{LastSuccessfulTime: &now, LastScheduleTime: &now}
	})).Should(Succeed())
}

func createStatefulKindBackupTool() *dpv1alpha1.BackupTool {
	By("By creating a backupTool")
	backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/pitr_backuptool.yaml",
		&dpv1alpha1.BackupTool{}, testapps.RandomizedObjName())
	Expect(testapps.ChangeObj(&testCtx, backupTool, func(bt *dpv1alpha1.BackupTool) {
		bt.Spec.DeployKind = dpv1alpha1.DeployKindStatefulSet
	})).Should(Succeed())
	return backupTool
}
