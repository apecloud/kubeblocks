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

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		viper.SetDefault(constant.CfgKeyCtrlrMgrNS, mgrNamespace)
		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
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
			By("By creating a backupTool")
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dpv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			backupToolName = backupTool.Name
		})

		AfterEach(func() {
			viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
		})

		Context("creates a backup policy", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddFullPolicy().
					SetBackupToolName(backupToolName).
					SetBackupsHistoryLimit(1).
					SetSchedule(defaultSchedule, true).
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
				Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
					client.MatchingLabels(backupPolicy.Spec.Datafile.Target.LabelsSelector.MatchLabels),
					client.InNamespace(backupPolicy.Namespace))).Should(Equal(1))
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
					AddFullPolicy().
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
					AddFullPolicy().
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.PolicyAvailable))
					g.Expect(fetched.Spec.Datafile.PersistentVolumeClaim.Name).To(Equal(pvcName))
					g.Expect(fetched.Spec.Datafile.PersistentVolumeClaim.InitCapacity.String()).To(Equal(pvcInitCapacity))
				})).Should(Succeed())
			})
		})
		Context("creating a logfile backupPolicy", func() {
			It("with reconfigure config", func() {
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
					SetBackupToolName(backupToolName).
					AddIncrementalPolicy().
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
					g.Expect(fetched.Annotations[constant.LastAppliedConfigAnnotation]).To(Equal(`[{"key":"archive_command","value":"''"}]`))
				})).Should(Succeed())

				By("disable schedule for reconfigure")
				Eventually(testapps.GetAndChangeObj(&testCtx, backupPolicyKey, func(fetched *dpv1alpha1.BackupPolicy) {
					fetched.Spec.Schedule.Logfile.Enable = false
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Annotations[constant.LastAppliedConfigAnnotation]).To(Equal(`[{"key":"archive_command","value":"'/bin/true'"}]`))
				})).Should(Succeed())
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
