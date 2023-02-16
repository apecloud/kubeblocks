/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dataprotection

import (
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
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Backup Policy Controller", func() {
	const clusterName = "wesql-cluster"
	const componentName = "replicasets-primary"
	const containerName = "mysql"
	const defaultPVCSize = "1Gi"
	const backupPolicyName = "test-backup-policy"
	const backupPolicyTplName = "test-backup-policy-template"
	const backupRemoteVolumeName = "backup-remote-volume"
	const backupRemotePVCName = "backup-remote-pvc"
	const defaultSchedule = "0 3 * * *"
	const defaultTTL = "168h0m0s"
	const backupNamePrefix = "test-backup-job-"

	viper.SetDefault("DP_BACKUP_SCHEDULE", "0 3 * * *")
	viper.SetDefault("DP_BACKUP_TTL", "168h0m0s")

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

		By("By mocking a statefulset")
		sts := testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, clusterName+"-"+componentName, clusterName, componentName).
			AddLabels(intctrlutil.AppInstanceLabelKey, clusterName).
			AddContainer(corev1.Container{Name: containerName, Image: testdbaas.ApeCloudMySQLImage}).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testdbaas.DataVolumeName},
				Spec:       testdbaas.NewPVC(defaultPVCSize),
			}).Create(&testCtx).GetObject()

		By("By mocking a pod belonging to the statefulset")
		_ = testdbaas.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddContainer(corev1.Container{Name: containerName, Image: testdbaas.ApeCloudMySQLImage}).
			Create(&testCtx)
	})

	AfterEach(cleanEnv)

	When("creating backup policy with default settings", func() {
		var backupToolName string
		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dpv1alpha1.BackupTool{}, testdbaas.RandomizedObjName())
			backupToolName = backupTool.Name

		})

		Context("creates a backup policy", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					SetBackupsHistoryLimit(1).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testdbaas.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
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
					intctrlutil.AppInstanceLabelKey:  backupPolicy.Labels[intctrlutil.AppInstanceLabelKey],
					dataProtectionLabelAutoBackupKey: "true",
				}

				backupExpired := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(backupExpired),
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupInProgress))
					})).Should(Succeed())

				backupStatus.Expiration = &metav1.Time{Time: now.Add(-time.Hour * 24)}
				backupStatus.StartTimestamp = backupStatus.Expiration
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupExpired))

				backupOutLimit1 := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(backupOutLimit1),
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupInProgress))
					})).Should(Succeed())

				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit1))

				backupOutLimit2 := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(backupOutLimit2),
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupInProgress))
					})).Should(Succeed())

				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour * 2)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit2))

				// trigger the backup policy controller through update cronjob
				patchCronJobStatus(backupPolicyKey)

				By("retain the latest backup")
				Eventually(testdbaas.GetListLen(&testCtx, intctrlutil.BackupSignature,
					client.MatchingLabels(backupPolicy.Spec.Target.LabelsSelector.MatchLabels),
					client.InNamespace(backupPolicy.Namespace))).Should(Equal(1))
			})
		})

		Context("creates a backup policy with empty schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testdbaas.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with invalid schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					SetSchedule("invalid schedule").
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should failed", func() {
				Eventually(testdbaas.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).NotTo(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with backup policy template", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				viper.SetDefault("DP_BACKUP_SCHEDULE", nil)
				viper.SetDefault("DP_BACKUP_TTL", nil)
				By("By creating a backupPolicyTemplate")
				template := testdbaas.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					SetCredentialKeyword("username", "password").
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					Create(&testCtx).GetObject()

				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testdbaas.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with nil pointer credentialKeyword in backupPolicyTemplate", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				viper.SetDefault("DP_BACKUP_SCHEDULE", nil)
				viper.SetDefault("DP_BACKUP_TTL", nil)
				By("By creating a backupPolicyTemplate")
				template := testdbaas.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					Create(&testCtx).GetObject()

				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testdbaas.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})
	})
})

func patchBackupStatus(status dpv1alpha1.BackupStatus, key types.NamespacedName) {
	Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, key, func(fetched *dpv1alpha1.Backup) {
		fetched.Status = status
	})).Should(Succeed())
}

func patchCronJobStatus(key types.NamespacedName) {
	now := metav1.Now()
	Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, key, func(fetched *batchv1.CronJob) {
		fetched.Status = batchv1.CronJobStatus{LastSuccessfulTime: &now, LastScheduleTime: &now}
	})).Should(Succeed())
}
