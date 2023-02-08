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
	appsv1 "k8s.io/api/apps/v1"
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
	const timeout = time.Second * 20
	const interval = time.Second

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

		By("By creating a statefulset")
		_ = testdbaas.CreateCustomizedObj(&testCtx, "backup/statefulset.yaml", &appsv1.StatefulSet{},
			testCtx.UseDefaultNamespace())
		_ = testdbaas.CreateCustomizedObj(&testCtx, "backup/statefulset_pod.yaml", &corev1.Pod{},
			testCtx.UseDefaultNamespace())
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
				backupPolicy = testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
					&dpv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backupPolicy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.BackupToolName = backupToolName
						backupPolicy.Spec.BackupsHistoryLimit = 1
					})
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

				backupExpired := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup.yaml",
					&dpv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dpv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicyKey.Name
						backup.SetLabels(autoBackupLabel)
					})
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupExpired))

				backupOutLimit1 := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup.yaml",
					&dpv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dpv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicyKey.Name
						backup.SetLabels(autoBackupLabel)
					})
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit1))

				time.Sleep(time.Second)

				backupOutLimit2 := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup.yaml",
					&dpv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dpv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicyKey.Name
						backup.SetLabels(autoBackupLabel)
					})
				backupStatus.StartTimestamp = &metav1.Time{Time: backupOutLimit2.CreationTimestamp.Time}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit2))

				// trigger the backup policy controller through update cronjob
				patchCronJobStatus(backupPolicyKey)

				By("retain the latest backup")
				Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(backupExpired),
					&dpv1alpha1.Backup{}, false), timeout, interval).Should(Succeed())
			})
		})

		Context("creates a backup policy with empty schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
					&dpv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backupPolicy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.BackupToolName = backupToolName
						backupPolicy.Spec.TTL = nil
						backupPolicy.Spec.Schedule = ""
					})
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
				backupPolicy = testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
					&dpv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backupPolicy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.BackupToolName = backupToolName
						backupPolicy.Spec.Schedule = "invalid schedule"
					})
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
				template := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicytemplate.yaml",
					&dpv1alpha1.BackupPolicyTemplate{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(t *dpv1alpha1.BackupPolicyTemplate) {
						t.Spec.BackupToolName = backupToolName
					})

				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
					&dpv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backupPolicy *dpv1alpha1.BackupPolicy) {
						backupPolicy.Spec.BackupPolicyTemplateName = template.Name
						backupPolicy.Spec.Schedule = ""
						backupPolicy.Spec.TTL = nil
						backupPolicy.Spec.OnFailAttempted = 0
						backupPolicy.Spec.Hooks = nil
						backupPolicy.Spec.BackupToolName = ""
					})
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
