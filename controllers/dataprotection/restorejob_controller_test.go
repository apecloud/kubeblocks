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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("RestoreJob Controller", func() {
	const (
		clusterName = "mycluster"
		compName    = "cluster"
	)
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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
			SetBackupType(dataprotectionv1alpha1.BackupTypeDataFile).
			Create(&testCtx).GetObject()
	}

	assureBackupPolicyObj := func(backupTool string) *dataprotectionv1alpha1.BackupPolicy {
		By("By assure an backupPolicy obj")
		return testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, "backup-policy-").
			WithRandomName().
			AddFullPolicy().
			AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
			SetSchedule("0 3 * * *", true).
			SetTTL("7d").
			SetBackupToolName(backupTool).
			SetTargetSecretName("mycluster-cluster-secret").
			SetPVC("backup-host-path-pvc").
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
		return testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, clusterName, clusterName, compName).
			SetReplicas(3).
			AddAppInstanceLabel(clusterName).
			AddContainer(corev1.Container{Name: "mysql", Image: testapps.ApeCloudMySQLImage}).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC("1Gi"),
			}).Create(&testCtx).GetObject()
	}

	patchBackupStatus := func(phase dataprotectionv1alpha1.BackupPhase, key types.NamespacedName) {
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(backup *dataprotectionv1alpha1.Backup) {
			backup.Status.Phase = phase
		})).Should(Succeed())
	}

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(job *batchv1.Job) {
			found := false
			for _, cond := range job.Status.Conditions {
				if cond.Type == jobStatus {
					found = true
				}
			}
			if !found {
				jobCondition := batchv1.JobCondition{Type: jobStatus}
				job.Status.Conditions = append(job.Status.Conditions, jobCondition)
			}
		})).Should(Succeed())
	}

	testRestoreJob := func(withResources ...bool) {
		By("By creating a statefulset and pod")
		sts := assureStatefulSetObj()
		testapps.MockConsensusComponentPods(&testCtx, sts, clusterName, compName)

		By("By creating a backupTool")
		backupTool := assureBackupToolObj(withResources...)

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
		backupKey := types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}
		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
			g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupInProgress))
		})).Should(Succeed())

		patchBackupStatus(dataprotectionv1alpha1.BackupCompleted, backupKey)

		patchK8sJobStatus(batchv1.JobComplete, key)

		result := &dataprotectionv1alpha1.RestoreJob{}
		Eventually(func() bool {
			Expect(k8sClient.Get(ctx, key, result)).Should(Succeed())
			return result.Status.Phase == dataprotectionv1alpha1.RestoreJobCompleted ||
				result.Status.Phase == dataprotectionv1alpha1.RestoreJobFailed
		}).Should(BeTrue())
		Expect(result.Status.Phase).Should(Equal(dataprotectionv1alpha1.RestoreJobCompleted))
	}

	Context("When creating restoreJob", func() {
		It("Should success with no error", func() {
			testRestoreJob()
		})

		It("Without backupTool resources should success with no error", func() {
			testRestoreJob(true)
		})
	})

})
