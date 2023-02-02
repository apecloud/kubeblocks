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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Backup for a StatefulSet", func() {

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

		By("By creating a statefulset")
		_ = testdbaas.CreateCustomizedObj(&testCtx, "backup/statefulset.yaml", &appsv1.StatefulSet{},
			testCtx.UseDefaultNamespace())
		_ = testdbaas.CreateCustomizedObj(&testCtx, "backup/statefulset_pod.yaml", &corev1.Pod{},
			testCtx.UseDefaultNamespace())
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("with default settings", func() {
		var backupPolicyName string

		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testdbaas.RandomizedObjName())

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
				&dataprotectionv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
				func(backupPolicy *dataprotectionv1alpha1.BackupPolicy) {
					backupPolicy.Spec.BackupToolName = backupTool.Name
				})
			backupPolicyName = backupPolicy.Name
		})

		Context("creates a full backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup.yaml",
					&dataprotectionv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dataprotectionv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicyName
					})
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})

			It("should fail after job fails", func() {
				patchK8sJobStatus(backupKey, batchv1.JobFailed)

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
				backup := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup_snapshot.yaml",
					&dataprotectionv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dataprotectionv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicyName
					})
				backupKey = client.ObjectKeyFromObject(backup)
			})

			AfterEach(func() {
				viper.Set("VOLUMESNAPSHOT", "false")
			})

			It("should success after all jobs complete", func() {
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobComplete)
				patchVolumeSnapshotStatus(backupKey, true)
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-post", Namespace: backupKey.Namespace}, batchv1.JobComplete)

				Eventually(testdbaas.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})

			It("should fail after pre-job fails", func() {
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobFailed)

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
				backupPolicy := testdbaas.CreateCustomizedObj(&testCtx, "backup/backuppolicy.yaml",
					&dataprotectionv1alpha1.BackupPolicy{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backupPolicy *dataprotectionv1alpha1.BackupPolicy) {
						backupPolicy.Spec.BackupToolName = backupTool.Name
					})

				By("By creating a backup from backupPolicy: " + backupPolicy.Name)
				backup := testdbaas.CreateCustomizedObj(&testCtx, "backup/backup.yaml",
					&dataprotectionv1alpha1.Backup{}, testdbaas.RandomizedObjName(), testCtx.UseDefaultNamespace(),
					func(backup *dataprotectionv1alpha1.Backup) {
						backup.Spec.BackupPolicyName = backupPolicy.Name
					})
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				patchK8sJobStatus(backupKey, batchv1.JobComplete)

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
