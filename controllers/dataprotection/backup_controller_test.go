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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Backup for a StatefulSet", func() {
	const clusterName = "wesql-cluster"
	const componentName = "replicasets-primary"
	const containerName = "mysql"
	const defaultPVCSize = "1Gi"
	const backupPolicyName = "test-backup-policy"
	const backupRemoteVolumeName = "backup-remote-volume"
	const backupRemotePVCName = "backup-remote-pvc"
	const defaultSchedule = "0 3 * * *"
	const defaultTTL = "168h0m0s"
	const backupName = "test-backup-job"

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

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testdbaas.RandomizedObjName())

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			_ = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
				SetBackupToolName(backupTool.Name).
				SetSchedule(defaultSchedule).
				SetTTL(defaultTTL).
				AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
				SetTargetSecretName(clusterName).
				AddHookPreCommand("touch /data/mysql/.restore;sync").
				AddHookPostCommand("rm -f /data/mysql/.restore;sync").
				SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
				Create(&testCtx).GetObject()
		})

		Context("creates a full backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})

			It("should fail after job fails", func() {
				patchK8sJobStatus(backupKey, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a snapshot backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				viper.Set("VOLUMESNAPSHOT", "true")

				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			AfterEach(func() {
				viper.Set("VOLUMESNAPSHOT", "false")
			})

			It("should success after all jobs complete", func() {
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobComplete)
				patchVolumeSnapshotStatus(backupKey, true)
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-post", Namespace: backupKey.Namespace}, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})

			It("should fail after pre-job fails", func() {
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})
	})

	When("without backupTool resources", func() {
		Context("creates a full backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("By creating a backupTool")
				backupTool := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
					&dataprotectionv1alpha1.BackupTool{}, testdbaas.RandomizedObjName(),
					func(backupTool *dataprotectionv1alpha1.BackupTool) {
						backupTool.Spec.Resources = nil
					})

				By("By creating a backupPolicy from backupTool: " + backupTool.Name)
				_ = testdbaas.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupTool.Name).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					AddMatchLabels(intctrlutil.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()

				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testdbaas.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})
		})
	})
})

func patchK8sJobStatus(key types.NamespacedName, jobStatus batchv1.JobConditionType) {
	Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, key, func(fetched *batchv1.Job) {
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		fetched.Status.Conditions = append(fetched.Status.Conditions, jobCondition)
	})).Should(Succeed())
}

func patchVolumeSnapshotStatus(key types.NamespacedName, readyToUse bool) {
	Eventually(testdbaas.GetAndChangeObjStatus(&testCtx, key, func(fetched *snapshotv1.VolumeSnapshot) {
		snapStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		fetched.Status = &snapStatus
	})).Should(Succeed())
}
