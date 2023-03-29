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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
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

	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)

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
		testapps.ClearResources(&testCtx, intctrlutil.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS, ml)
		//
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.BackupToolSignature, ml)
	}
	var nodeName string
	var pvcName string

	BeforeEach(func() {
		cleanEnv()
		viper.Set(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
		By("mock a cluster")
		testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			"test-cd", "test-cv").Create(&testCtx)

		By("By mocking a statefulset")
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, clusterName+"-"+componentName, clusterName, componentName).
			AddAppInstanceLabel(clusterName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: testapps.DataVolumeName},
				Spec:       testapps.NewPVC(defaultPVCSize),
			}).Create(&testCtx).GetObject()

		By("By mocking a pvc belonging to the pod")
		pvc := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+sts.Name+"-0", clusterName, componentName, "data").
			SetStorage("1Gi").
			Create(&testCtx).GetObject()
		pvcName = pvc.Name

		By("By mocking a pvc belonging to the pod2")
		pvc2 := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+sts.Name+"-1", clusterName, componentName, "data").
			SetStorage("1Gi").
			Create(&testCtx).GetObject()

		By("By mocking a pod belonging to the statefulset")
		volume := corev1.Volume{Name: pvc.Name, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}
		pod := testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
			AddAppInstanceLabel(clusterName).
			AddRoleLabel("leader").
			AddAppComponentLabel(componentName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			AddVolume(volume).
			Create(&testCtx).GetObject()
		nodeName = pod.Spec.NodeName

		By("By mocking a pod 2 belonging to the statefulset")
		volume2 := corev1.Volume{Name: pvc2.Name, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc2.Name}}}
		_ = testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-1").
			AddAppInstanceLabel(clusterName).
			AddAppComponentLabel(componentName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			AddVolume(volume2).
			Create(&testCtx).GetObject()
	})

	AfterEach(func() {
		cleanEnv()
		viper.Set(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
	})

	When("with default settings", func() {
		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			_ = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
				SetBackupToolName(backupTool.Name).
				SetSchedule(defaultSchedule).
				SetTTL(defaultTTL).
				AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
				AddMatchLabels(constant.RoleLabelKey, "leader").
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
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				By("Check backup job's nodeName equals pod's nodeName")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *batchv1.Job) {
					g.Expect(fetched.Spec.Template.Spec.NodeName).To(Equal(nodeName))
				})).Should(Succeed())

				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
					g.Expect(fetched.Labels[constant.AppInstanceLabelKey]).Should(Equal(clusterName))
					g.Expect(fetched.Labels[constant.KBAppComponentLabelKey]).Should(Equal(componentName))
					g.Expect(fetched.Annotations[constant.ClusterSnapshotAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				By("Check backup job is deleted after completed")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey, &batchv1.Job{}, false))
			})

			It("should fail after job fails", func() {
				patchK8sJobStatus(backupKey, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a snapshot backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				viper.Set("VOLUMESNAPSHOT", "true")
				viper.Set(constant.CfgKeyCtrlrMgrNS, "default")
				viper.Set(constant.CfgKeyCtrlrMgrAffinity,
					"{\"nodeAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"preference\":{\"matchExpressions\":[{\"key\":\"kb-controller\",\"operator\":\"In\",\"values\":[\"true\"]}]},\"weight\":100}]}}")
				viper.Set(constant.CfgKeyCtrlrMgrTolerations,
					"[{\"key\":\"key1\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\"}]")
				viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "{\"beta.kubernetes.io/arch\":\"amd64\"}")

				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			AfterEach(func() {
				viper.Set("VOLUMESNAPSHOT", "false")
				viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
				viper.Set(constant.CfgKeyCtrlrMgrTolerations, "")
				viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "")
			})

			It("should success after all jobs complete", func() {
				preJobKey := types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}
				postJobKey := types.NamespacedName{Name: backupKey.Name + "-post", Namespace: backupKey.Namespace}
				patchK8sJobStatus(preJobKey, batchv1.JobComplete)
				By("Check job tolerations")
				Eventually(testapps.CheckObj(&testCtx, preJobKey, func(g Gomega, fetched *batchv1.Job) {
					g.Expect(fetched.Spec.Template.Spec.Tolerations).ShouldNot(BeEmpty())
					g.Expect(fetched.Spec.Template.Spec.NodeSelector).ShouldNot(BeEmpty())
					g.Expect(fetched.Spec.Template.Spec.Affinity).ShouldNot(BeNil())
					g.Expect(fetched.Spec.Template.Spec.Affinity.NodeAffinity).ShouldNot(BeNil())
				})).Should(Succeed())

				patchVolumeSnapshotStatus(backupKey, true)
				patchK8sJobStatus(postJobKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())

				By("Check pre job cleaned")
				Eventually(testapps.CheckObjExists(&testCtx, preJobKey, &batchv1.Job{}, false)).Should(Succeed())
				By("Check post job cleaned")
				Eventually(testapps.CheckObjExists(&testCtx, postJobKey, &batchv1.Job{}, false)).Should(Succeed())
				By("Check if the target pod name is correct")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *snapshotv1.VolumeSnapshot) {
					g.Expect(*fetched.Spec.Source.PersistentVolumeClaimName).To(Equal(pvcName))
				})).Should(Succeed())
			})

			It("should fail after pre-job fails", func() {
				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})

			It("should fail if volumesnapshot reports error", func() {

				By("patching job status to pass check")
				preJobKey := types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}
				patchK8sJobStatus(preJobKey, batchv1.JobComplete)

				By("patching volumesnapshot status with error")
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(tmpVS *snapshotv1.VolumeSnapshot) {
					msg := "test-error"
					vsError := snapshotv1.VolumeSnapshotError{
						Message: &msg,
					}
					snapStatus := snapshotv1.VolumeSnapshotStatus{Error: &vsError}
					tmpVS.Status = &snapStatus
				})).Should(Succeed())

				By("checking backup failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})

		Context("creates a snapshot backup on error", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				viper.Set("VOLUMESNAPSHOT", "true")
				By("By remove persistent pvc")
				// delete rest mocked objects
				inNS := client.InNamespace(testCtx.DefaultNamespace)
				ml := client.HasLabels{testCtx.TestObjLabelKey}
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS, ml)
			})

			It("should fail when disable volumesnapshot", func() {
				viper.Set("VOLUMESNAPSHOT", "false")

				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})

			It("should fail without pvc", func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)

				patchK8sJobStatus(types.NamespacedName{Name: backupKey.Name + "-pre", Namespace: backupKey.Namespace}, batchv1.JobComplete)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
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
				backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
					&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName(),
					func(backupTool *dataprotectionv1alpha1.BackupTool) {
						backupTool.Spec.Resources = nil
					})

				By("By creating a backupPolicy from backupTool: " + backupTool.Name)
				_ = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					SetBackupToolName(backupTool.Name).
					SetSchedule(defaultSchedule).
					SetTTL(defaultTTL).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					AddHookPreCommand("touch /data/mysql/.restore;sync").
					AddHookPostCommand("rm -f /data/mysql/.restore;sync").
					SetRemoteVolumePVC(backupRemoteVolumeName, backupRemotePVCName).
					Create(&testCtx).GetObject()

				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
				})).Should(Succeed())
			})
		})
	})
	When("with exceptional settings", func() {
		Context("creates a backup with non existent backup policy", func() {
			var backupKey types.NamespacedName
			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetTTL(defaultTTL).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})
			It("Should fail", func() {
				By("Check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})
	})
})

func patchK8sJobStatus(key types.NamespacedName, jobStatus batchv1.JobConditionType) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *batchv1.Job) {
		jobCondition := batchv1.JobCondition{Type: jobStatus}
		fetched.Status.Conditions = append(fetched.Status.Conditions, jobCondition)
	})).Should(Succeed())
}

func patchVolumeSnapshotStatus(key types.NamespacedName, readyToUse bool) {
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(fetched *snapshotv1.VolumeSnapshot) {
		snapStatus := snapshotv1.VolumeSnapshotStatus{ReadyToUse: &readyToUse}
		fetched.Status = &snapStatus
	})).Should(Succeed())
}
