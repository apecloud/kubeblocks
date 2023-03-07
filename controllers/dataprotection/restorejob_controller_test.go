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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("RestoreJob Controller", func() {

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
		testapps.ClearResources(&testCtx, intctrlutil.RestoreJobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	assureRestoreJobObj := func(backup string) *dataprotectionv1alpha1.RestoreJob {
		By("By assure an restoreJob obj")
		return testapps.NewRestoreJobFactory(testCtx.DefaultNamespace, "restore-job-").
			WithRandomName().SetBackupJobName(backup).
			SetTargetSecretName("mycluster-cluster-secret").
			AddTargetVolumePVC("mysql-restore-storage", "datadir-mycluster-0").
			AddTargetVolumeMount(corev1.VolumeMount{Name: "mysql-restore-storage", MountPath: "/var/lib/mysql"}).
			Create(&testCtx).GetObject()
	}

	assureBackupObj := func(backupPolicy string) *dataprotectionv1alpha1.Backup {
		By("By assure an backup obj")
		return testapps.NewBackupFactory(testCtx.DefaultNamespace, "backup-job-").
			WithRandomName().SetBackupPolicyName(backupPolicy).
			SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
			SetTTL("168h0m0s").
			Create(&testCtx).GetObject()
	}

	assureBackupPolicyObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		return testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, "backup-policy-").
			WithRandomName().
			SetSchedule("0 3 * * *").
			SetTTL("168h0m0s").
			SetBackupToolName(backupTool).
			SetBackupPolicyTplName("backup-config-mysql").
			SetTargetSecretName("mycluster-cluster-secret").
			SetRemoteVolumePVC("backup-remote-volume", "backup-host-path-pvc").
			Create(&testCtx).GetObject()
	}

	assureBackupToolObj := func(withoutResources ...bool) *dataprotectionv1alpha1.BackupTool {
		By("By assure an backupTool obj")
		return testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
			&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName(),
			func(bt *dataprotectionv1alpha1.BackupTool) {
				nilResources := false
				// optional arguments, only use the first one.
				if len(withoutResources) > 0 {
					nilResources = withoutResources[0]
				}
				if nilResources {
					bt.Spec.Resources = nil
				}
			})
	}

	assureStatefulSetObj := func() *appsv1.StatefulSet {
		By("By assure an stateful obj")
		return testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, "mycluster", "mycluster", "replicasets").
			AddAppInstanceLabel("mycluster").
			AddContainer(corev1.Container{Name: "mysql", Image: testapps.ApeCloudMySQLImage}).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC("1Gi"),
			}).Create(&testCtx).GetObject()
	}

	patchBackupStatus := func(phase dataprotectionv1alpha1.BackupPhase, key types.NamespacedName) {
		backup := dataprotectionv1alpha1.Backup{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &backup)
		}).Should(Succeed())
		Expect(k8sClient.Get(ctx, key, &backup)).Should(Succeed())

		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.Phase = phase
		Expect(k8sClient.Status().Patch(ctx, &backup, patch)).Should(Succeed())
	}

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		k8sJob := batchv1.Job{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &k8sJob)
		}).Should(Succeed())
		Expect(k8sClient.Get(ctx, key, &k8sJob)).Should(Succeed())

		patch := client.MergeFrom(k8sJob.DeepCopy())
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		k8sJob.Status.Conditions = append(k8sJob.Status.Conditions, jobCondition)
		Expect(k8sClient.Status().Patch(ctx, &k8sJob, patch)).Should(Succeed())
	}

	Context("When creating restoreJob", func() {
		It("Should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj()

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backup from backupPolicy: " + backupPolicy.Name)
			backup := assureBackupObj(backupPolicy.Name)

			By("By creating a restoreJob from backup: " + backup.Name)
			toCreate := assureRestoreJobObj(backup.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchBackupStatus(dataprotectionv1alpha1.BackupCompleted, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace})

			patchK8sJobStatus(batchv1.JobComplete, types.NamespacedName{Name: toCreate.Name, Namespace: toCreate.Namespace})

			result := &dataprotectionv1alpha1.RestoreJob{}
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.RestoreJobCompleted ||
					result.Status.Phase == dataprotectionv1alpha1.RestoreJobFailed
			}).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.RestoreJobCompleted))
		})

		It("Without backupTool resources should success with no error", func() {

			By("By creating a statefulset")
			_ = assureStatefulSetObj()

			By("By creating a backupTool")
			backupTool := assureBackupToolObj(true)

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			backupPolicy := assureBackupPolicyObj(backupTool.Name)

			By("By creating a backup from backupPolicy: " + backupPolicy.Name)
			backup := assureBackupObj(backupPolicy.Name)

			By("By creating a restoreJob from backup: " + backup.Name)
			toCreate := assureRestoreJobObj(backup.Name)
			key := types.NamespacedName{
				Name:      toCreate.Name,
				Namespace: toCreate.Namespace,
			}

			patchBackupStatus(dataprotectionv1alpha1.BackupCompleted, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace})

			patchK8sJobStatus(batchv1.JobComplete, types.NamespacedName{Name: toCreate.Name, Namespace: toCreate.Namespace})

			result := &dataprotectionv1alpha1.RestoreJob{}
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
				return result.Status.Phase == dataprotectionv1alpha1.RestoreJobCompleted ||
					result.Status.Phase == dataprotectionv1alpha1.RestoreJobFailed
			}).Should(BeTrue())
			Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.RestoreJobCompleted))
		})
	})

})
