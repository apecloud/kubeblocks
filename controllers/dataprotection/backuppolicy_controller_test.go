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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
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
		_ = testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			Create(&testCtx)
	})

	AfterEach(cleanEnv)

	When("creating backup policy with default settings", func() {
		var backupToolName string
		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dpv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			backupToolName = backupTool.Name

		})

		Context("creates a backup policy", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					SetBackupsHistoryLimit(1).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
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
					constant.AppInstanceLabelKey:     backupPolicy.Labels[constant.AppInstanceLabelKey],
					dataProtectionLabelAutoBackupKey: "true",
				}

				By("create a expired backup")
				backupExpired := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				By("create 1st limit backup")
				backupOutLimit1 := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				By("create 2nd limit backup")
				backupOutLimit2 := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupNamePrefix).
					WithRandomName().AddLabelsInMap(autoBackupLabel).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dpv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()

				By("mock jobs completed")
				backupExpiredKey := client.ObjectKeyFromObject(backupExpired)
				patchK8sJobStatus(backupExpiredKey, batchv1.JobComplete)
				backupOutLimit1Key := client.ObjectKeyFromObject(backupOutLimit1)
				patchK8sJobStatus(backupOutLimit1Key, batchv1.JobComplete)
				backupOutLimit2Key := client.ObjectKeyFromObject(backupOutLimit2)
				patchK8sJobStatus(backupOutLimit2Key, batchv1.JobComplete)

				By("waiting expired backup completed")
				Eventually(testapps.CheckObj(&testCtx, backupExpiredKey,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
				By("waiting 1st limit backup completed")
				Eventually(testapps.CheckObj(&testCtx, backupOutLimit1Key,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())
				By("waiting 2nd limit backup completed")
				Eventually(testapps.CheckObj(&testCtx, backupOutLimit2Key,
					func(g Gomega, fetched *dpv1alpha1.Backup) {
						g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupCompleted))
					})).Should(Succeed())

				By("mock update expired backup status to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(-time.Hour * 24)}
				backupStatus.StartTimestamp = backupStatus.Expiration
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupExpired))
				By("mock update 1st limit backup NOT to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit1))
				By("mock update 2nd limit backup NOT to expire")
				backupStatus.Expiration = &metav1.Time{Time: now.Add(time.Hour * 24)}
				backupStatus.StartTimestamp = &metav1.Time{Time: now.Add(time.Hour * 2)}
				patchBackupStatus(backupStatus, client.ObjectKeyFromObject(backupOutLimit2))

				// trigger the backup policy controller through update cronjob
				patchCronJobStatus(backupPolicyKey)

				By("retain the latest backup")
				Eventually(testapps.GetListLen(&testCtx, intctrlutil.BackupSignature,
					client.MatchingLabels(backupPolicy.Spec.Target.LabelsSelector.MatchLabels),
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
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
			It("should success with empty viper config", func() {
				viper.SetDefault("DP_BACKUP_SCHEDULE", "")
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with invalid schedule", func() {
			var backupPolicyKey types.NamespacedName
			var backupPolicy *dpv1alpha1.BackupPolicy
			BeforeEach(func() {
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					SetSchedule("invalid schedule").
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should failed", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
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
				template := testapps.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					SetCredentialKeyword("username", "password").
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					Create(&testCtx).GetObject()

				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
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
				template := testapps.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					Create(&testCtx).GetObject()

				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey = client.ObjectKeyFromObject(backupPolicy)
			})
			It("should success", func() {
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})
		})

		Context("creates a backup policy with empty secret", func() {
			var (
				backupSecretName = "backup-secret"
				rootSecretName   = "root-secret"
				secretsMap       map[string]*corev1.Secret
			)

			// delete secrets before test starts
			cleanSecrets := func() {
				// delete rest mocked objects
				inNS := client.InNamespace(testCtx.DefaultNamespace)
				ml := client.HasLabels{testCtx.TestObjLabelKey}
				// delete secret created for backup policy
				testapps.ClearResources(&testCtx, intctrlutil.SecretSignature, inNS, ml)
				testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
				testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, inNS, ml)
			}

			fakeSecret := func(name string, labels map[string]string) *corev1.Secret {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: testCtx.DefaultNamespace,
						Labels:    labels,
					},
				}
			}

			BeforeEach(func() {
				secretsMap = make(map[string]*corev1.Secret)
				// mock two secrets for backup policy, one for backup account, one for root conn
				secretsMap[backupSecretName] = fakeSecret(backupSecretName, map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.ClusterAccountLabelKey: (string)(appsv1alpha1.DataprotectionAccount),
				})
				secretsMap[rootSecretName] = fakeSecret(rootSecretName, map[string]string{
					constant.AppInstanceLabelKey:  clusterName,
					constant.AppManagedByLabelKey: constant.AppName,
				})

				cleanSecrets()
			})

			AfterEach(cleanSecrets)

			It("creating a backupPolicy with secret", func() {
				// create two secrets
				for _, v := range secretsMap {
					Expect(testCtx.CreateObj(context.Background(), v)).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(v), &corev1.Secret{}, true)).Should(Succeed())
				}

				By("By creating a backupPolicy with empty secret")
				randomSecretName := testCtx.GetRandomStr()
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(randomSecretName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
					g.Expect(fetched.Spec.Target.Secret.Name).To(Equal(randomSecretName))
				})).Should(Succeed())
			})

			It("creating a backupPolicy with secrets missing", func() {
				By("By creating a backupPolicy with empty secret")
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				By("Secrets missing, the backup policy should never be `ConfigAvailable`")
				Consistently(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).NotTo(Equal(dpv1alpha1.ConfigAvailable))
				})).Should(Succeed())
			})

			It("creating a backupPolicy uses default secret", func() {
				// create two secrets
				for _, v := range secretsMap {
					Expect(testCtx.CreateObj(context.Background(), v)).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(v), &corev1.Secret{}, true)).Should(Succeed())
				}

				By("By creating a backupPolicy with empty secret")
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupToolName).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
					g.Expect(fetched.Spec.Target.Secret.Name).To(Equal(backupSecretName))
				})).Should(Succeed())
			})

			It("create backup policy with tempate and specify credential keyword", func() {
				for _, v := range secretsMap {
					Expect(testCtx.CreateObj(context.Background(), v)).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(v), &corev1.Secret{}, true)).Should(Succeed())
				}
				// create template
				template := testapps.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					SetCredentialKeyword("username", "password").
					Create(&testCtx).GetObject()

				// create backup policy
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
					g.Expect(fetched.Spec.Target.Secret.Name).To(Equal(rootSecretName))
				})).Should(Succeed())
			})

			It("create backup policy with tempate but without default credential keyword", func() {
				// create two secrets
				for _, v := range secretsMap {
					Expect(testCtx.CreateObj(context.Background(), v)).Should(Succeed())
					Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(v), &corev1.Secret{}, true)).Should(Succeed())
				}
				// create template
				template := testapps.NewBackupPolicyTemplateFactory(backupPolicyTplName).
					SetBackupToolName(backupToolName).
					Create(&testCtx).GetObject()

				// create backup policy
				By("By creating a backupPolicy from backupTool: " + backupToolName)
				backupPolicy := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupPolicyTplName(template.Name).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()
				backupPolicyKey := client.ObjectKeyFromObject(backupPolicy)
				Eventually(testapps.CheckObj(&testCtx, backupPolicyKey, func(g Gomega, fetched *dpv1alpha1.BackupPolicy) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.ConfigAvailable))
					g.Expect(fetched.Spec.Target.Secret.Name).To(Equal(backupSecretName))
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
