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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	dpbackup "github.com/apecloud/kubeblocks/internal/dataprotection/backup"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
)

var _ = Describe("Backup Controller test", func() {
	const (
		clusterName                    = "wesql-cluster"
		componentName                  = "replicasets-primary"
		containerName                  = "mysql"
		backupPolicyName               = "test-backup-policy"
		backupRemotePVCName            = "backup-remote-pvc"
		defaultSchedule                = "0 3 * * *"
		defaultTTL                     = "7d"
		backupName                     = "test-backup-job"
		storageClassName               = "test-storage-class"
		backupRepoName                 = "test-backup-repo"
		backupMethodName               = "xtrabackup"
		volumeMountPath                = "/var/lib/mysql"
		pathPrefix                     = "/backup"
		volumeSnapshotBackupMethodName = "volume-snapshot"
	)

	var (
		nodeName   string
		volumeName string
		cluster    *appsv1alpha1.Cluster
	)

	viper.SetDefault(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		// namespaced
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResources(&testCtx, generics.CronJobSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ActionSetSignature, ml)
		testapps.ClearResources(&testCtx, generics.StorageClassSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupRepoSignature, true, ml)
		testapps.ClearResources(&testCtx, generics.StorageProviderSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()
		viper.Set(constant.CfgKeyCtrlrMgrNS, testCtx.DefaultNamespace)
		By("mock a cluster")
		cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			"test-cd", "test-cv").Create(&testCtx).GetObject()
		podGenerateName := clusterName + "-" + componentName
		By("By mocking a storage class")
		_ = testapps.CreateStorageClass(&testCtx, storageClassName, true)

		By("By mocking a pvc belonging to the pod")
		pvc := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+podGenerateName+"-0", clusterName, componentName, "data").
			SetStorage("1Gi").
			SetStorageClass(storageClassName).
			Create(&testCtx).GetObject()

		By("By mocking a pvc belonging to the pod2")
		pvc2 := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+podGenerateName+"-1", clusterName, componentName, "data").
			SetStorage("1Gi").
			SetStorageClass(storageClassName).
			Create(&testCtx).GetObject()

		By("By mocking a pod belonging to the statefulset")
		volume := corev1.Volume{Name: pvc.Name, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name}}}
		pod := testapps.NewPodFactory(testCtx.DefaultNamespace, podGenerateName+"-0").
			AddAppInstanceLabel(clusterName).
			AddRoleLabel("leader").
			AddAppComponentLabel(componentName).
			AddContainer(corev1.Container{Name: containerName, Image: testapps.ApeCloudMySQLImage}).
			AddVolume(volume).
			Create(&testCtx).GetObject()
		nodeName = pod.Spec.NodeName
		volumeName = volume.Name

		By("By mocking a pod 2 belonging to the statefulset")
		volume2 := corev1.Volume{Name: pvc2.Name, VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc2.Name}}}
		_ = testapps.NewPodFactory(testCtx.DefaultNamespace, podGenerateName+"-1").
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
		var (
			actionSet    *dpv1alpha1.ActionSet
			backupPolicy *dpv1alpha1.BackupPolicy
		)
		BeforeEach(func() {
			By("By creating a backupTool")
			actionSet = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
				&dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())

			By("By creating a backupPolicy from actionSet: " + actionSet.Name)
			backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
				SetTarget(constant.AppInstanceLabelKey, clusterName, constant.RoleLabelKey, "leader").
				SetBackupRepoName(backupRepoName).
				SetPathPrefix(pathPrefix).
				SetTargetConnectionCredential(clusterName).
				AddBackupMethod(backupMethodName, false, actionSet.Name).
				SetBackupMethodVolumeMounts(volumeName, volumeMountPath).
				AddBackupMethod(volumeSnapshotBackupMethodName, true, actionSet.Name).
				SetBackupMethodVolumes([]string{volumeName}).
				Create(&testCtx).GetObject()
		})

		Context("creates a backup", func() {
			var backupKey types.NamespacedName
			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetBackupPolicyName(backupPolicyName).
					SetBackupMethod(backupMethodName).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})

			It("should succeed after job completes", func() {
				By("Check backup status")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.PersistentVolumeClaimName).Should(Equal(backupRemotePVCName))
					g.Expect(fetched.Status.Path).Should(Equal(getBackupPath(fetched, backupPolicy.Spec.PathPrefix)))
				})).Should(Succeed())

				By("Check backup job's nodeName equals pod's nodeName")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *batchv1.Job) {
					g.Expect(fetched.Spec.Template.Spec.NodeSelector[hostNameLabelKey]).To(Equal(nodeName))
				})).Should(Succeed())

				patchK8sJobStatus(backupKey, batchv1.JobComplete)

				By("Check backup job completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
					g.Expect(fetched.Labels[dptypes.DataProtectionLabelClusterUIDKey]).Should(Equal(string(cluster.UID)))
					g.Expect(fetched.Labels[constant.AppInstanceLabelKey]).Should(Equal(clusterName))
					g.Expect(fetched.Labels[constant.KBAppComponentLabelKey]).Should(Equal(componentName))
					g.Expect(fetched.Annotations[constant.ClusterSnapshotAnnotationKey]).ShouldNot(BeEmpty())
				})).Should(Succeed())

				By("Check backup job is deleted after completed")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey, &batchv1.Job{}, false)).Should(Succeed())
			})

			It("should fail after job fails", func() {
				patchK8sJobStatus(backupKey, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})

		Context("deletes a backup", func() {
			var backupKey types.NamespacedName
			var backup *dpv1alpha1.Backup
			BeforeEach(func() {
				By("creating a backup from backupPolicy: " + backupPolicyName)
				backup = testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetBackupPolicyName(backupPolicyName).
					SetBackupMethod(backupMethodName).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)

				By("waiting for finalizers to be added")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dpv1alpha1.Backup) {
					g.Expect(backup.GetFinalizers()).ToNot(BeEmpty())
				})).Should(Succeed())

				By("setting backup file path")
				Eventually(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
				})).Should(Succeed())
			})

			It("should create a Job for deleting backup files", func() {
				By("deleting a Backup object")
				testapps.DeleteObject(&testCtx, backupKey, &dpv1alpha1.Backup{})

				By("checking new created Job")
				jobKey := dpbackup.BuildDeleteBackupFilesJobKey(backup)
				job := &batchv1.Job{}
				Eventually(testapps.CheckObjExists(&testCtx, jobKey,
					job, true)).Should(Succeed())
				volumeName := "backup-" + backupRemotePVCName
				Eventually(testapps.CheckObj(&testCtx, jobKey, func(g Gomega, job *batchv1.Job) {
					Expect(job.Spec.Template.Spec.Volumes).
						Should(ContainElement(corev1.Volume{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: backupRemotePVCName,
								},
							},
						}))
					Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).
						Should(ContainElement(corev1.VolumeMount{
							Name:      volumeName,
							MountPath: backupPathBase,
						}))
				})).Should(Succeed())

				By("checking Backup object, it should not be deleted")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("mock job for deletion to Failed, backup should not be deleted")
				Expect(testapps.ChangeObjStatus(&testCtx, job, func() {
					job.Status.Conditions = []batchv1.JobCondition{
						{
							Type: batchv1.JobFailed,
						},
					}
				})).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, true)).Should(Succeed())

				By("mock job for deletion to completed, backup should be deleted")
				Expect(testapps.ChangeObjStatus(&testCtx, job, func() {
					job.Status.Conditions = []batchv1.JobCondition{
						{
							Type: batchv1.JobComplete,
						},
					}
				})).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dpv1alpha1.Backup{}, false)).Should(Succeed())

				// TODO: add delete backup test case with the pvc not exists
			})
		})

		//Context("creates a snapshot backup", func() {
		//	var backupKey types.NamespacedName
		//	var backup *dpv1alpha1.Backup
		//
		//	BeforeEach(func() {
		//		viper.Set("VOLUMESNAPSHOT", "true")
		//		viper.Set(constant.CfgKeyCtrlrMgrNS, "default")
		//		viper.Set(constant.CfgKeyCtrlrMgrAffinity,
		//			"{\"nodeAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"preference\":{\"matchExpressions\":[{\"key\":\"kb-controller\",\"operator\":\"In\",\"values\":[\"true\"]}]},\"weight\":100}]}}")
		//		viper.Set(constant.CfgKeyCtrlrMgrTolerations,
		//			"[{\"key\":\"key1\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\"}]")
		//		viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "{\"beta.kubernetes.io/arch\":\"amd64\"}")
		//		snapshotBackupName := "backup-default-postgres-cluster-20230628104804"
		//		By("By creating a backup from backupPolicy: " + backupPolicyName)
		//		backup = testapps.NewBackupFactory(testCtx.DefaultNamespace, snapshotBackupName).
		//			SetBackupPolicyName(backupPolicyName).
		//			SetBackupMethod(volumeSnapshotBackupMethodName).
		//			Create(&testCtx).GetObject()
		//		backupKey = client.ObjectKeyFromObject(backup)
		//	})
		//
		//	AfterEach(func() {
		//		viper.Set("VOLUMESNAPSHOT", "false")
		//		viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
		//		viper.Set(constant.CfgKeyCtrlrMgrTolerations, "")
		//		viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, "")
		//	})
		//
		//	It("should success after all jobs complete", func() {
		//		backupPolicyKey := types.NamespacedName{Name: backupPolicyName, Namespace: backupKey.Namespace}
		//		patchBackupPolicySpecBackupStatusUpdates(backupPolicyKey)
		//
		//		preJobKey := types.NamespacedName{Name: generateUniqueJobName(backup, "hook-pre"), Namespace: backupKey.Namespace}
		//		postJobKey := types.NamespacedName{Name: generateUniqueJobName(backup, "hook-post"), Namespace: backupKey.Namespace}
		//		patchK8sJobStatus(preJobKey, batchv1.JobComplete)
		//		By("Check job tolerations")
		//		Eventually(testapps.CheckObj(&testCtx, preJobKey, func(g Gomega, fetched *batchv1.Job) {
		//			g.Expect(fetched.Spec.Template.Spec.Tolerations).ShouldNot(BeEmpty())
		//			g.Expect(fetched.Spec.Template.Spec.NodeSelector).ShouldNot(BeEmpty())
		//			g.Expect(fetched.Spec.Template.Spec.Affinity).ShouldNot(BeNil())
		//			g.Expect(fetched.Spec.Template.Spec.Affinity.NodeAffinity).ShouldNot(BeNil())
		//		})).Should(Succeed())
		//
		//		patchVolumeSnapshotStatus(backupKey, true)
		//		patchK8sJobStatus(postJobKey, batchv1.JobComplete)
		//
		//		logJobKey := types.NamespacedName{Name: generateUniqueJobName(backup, "status-0-pre"), Namespace: backupKey.Namespace}
		//		patchK8sJobStatus(logJobKey, batchv1.JobComplete)
		//
		//		By("Check backup job completed")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
		//			g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseCompleted))
		//		})).Should(Succeed())
		//
		//		sizeJobKey := types.NamespacedName{Name: generateUniqueJobName(backup, "status-1-post"), Namespace: backupKey.Namespace}
		//		patchK8sJobStatus(sizeJobKey, batchv1.JobComplete)
		//
		//		By("Check pre job cleaned")
		//		Eventually(testapps.CheckObjExists(&testCtx, preJobKey, &batchv1.Job{}, false)).Should(Succeed())
		//		By("Check post job cleaned")
		//		Eventually(testapps.CheckObjExists(&testCtx, postJobKey, &batchv1.Job{}, false)).Should(Succeed())
		//		By("Check if the target pod name is correct")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *snapshotv1.VolumeSnapshot) {
		//			g.Expect(*fetched.Spec.Source.PersistentVolumeClaimName).To(Equal(pvcName))
		//		})).Should(Succeed())
		//	})
		//
		//	It("should fail after pre-job fails", func() {
		//		patchK8sJobStatus(types.NamespacedName{Name: generateUniqueJobName(backup, "hook-pre"), Namespace: backupKey.Namespace}, batchv1.JobFailed)
		//
		//		By("Check backup job failed")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
		//			g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
		//		})).Should(Succeed())
		//	})
		//
		//	It("should fail if volumesnapshot reports error", func() {
		//
		//		By("patching job status to pass check")
		//		preJobKey := types.NamespacedName{Name: generateUniqueJobName(backup, "hook-pre"), Namespace: backupKey.Namespace}
		//		patchK8sJobStatus(preJobKey, batchv1.JobComplete)
		//
		//		By("patching volumesnapshot status with error")
		//		Eventually(testapps.GetAndChangeObjStatus(&testCtx, backupKey, func(tmpVS *snapshotv1.VolumeSnapshot) {
		//			msg := "Failed to set default snapshot class with error: some error"
		//			vsError := snapshotv1.VolumeSnapshotError{
		//				Message: &msg,
		//			}
		//			snapStatus := snapshotv1.VolumeSnapshotStatus{Error: &vsError}
		//			tmpVS.Status = &snapStatus
		//		})).Should(Succeed())
		//
		//		By("checking backup failed")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
		//			g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
		//		})).Should(Succeed())
		//	})
		//})
		//
		//Context("creates a snapshot backup on error", func() {
		//	var backupKey types.NamespacedName
		//
		//	BeforeEach(func() {
		//		viper.Set("VOLUMESNAPSHOT", "true")
		//		By("By remove persistent pvc")
		//		// delete rest mocked objects
		//		inNS := client.InNamespace(testCtx.DefaultNamespace)
		//		ml := client.HasLabels{testCtx.TestObjLabelKey}
		//		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
		//	})
		//
		//	It("should fail when disable volumesnapshot", func() {
		//		viper.Set("VOLUMESNAPSHOT", "false")
		//
		//		By("By creating a backup from backupPolicy: " + backupPolicyName)
		//		backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
		//			SetBackupPolicyName(backupPolicyName).
		//			SetBackupMethod(dpv1alpha1.BackupTypeSnapshot).
		//			Create(&testCtx).GetObject()
		//		backupKey = client.ObjectKeyFromObject(backup)
		//
		//		By("Check backup job failed")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
		//			g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
		//		})).Should(Succeed())
		//	})
		//
		//	It("should fail without pvc", func() {
		//		By("By creating a backup from backupPolicy: " + backupPolicyName)
		//		backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
		//			SetBackupPolicyName(backupPolicyName).
		//			SetBackupMethod(volumeSnapshotBackupMethodName).
		//			Create(&testCtx).GetObject()
		//		backupKey = client.ObjectKeyFromObject(backup)
		//
		//		patchK8sJobStatus(types.NamespacedName{Name: generateUniqueJobName(backup, "hook-pre"), Namespace: backupKey.Namespace}, batchv1.JobComplete)
		//
		//		By("Check backup job failed")
		//		Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
		//			g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
		//		})).Should(Succeed())
		//	})
		//
		//})
	})

	When("with exceptional settings", func() {
		Context("creates a backup with non-existent backup policy", func() {
			var backupKey types.NamespacedName
			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetBackupPolicyName(backupPolicyName).
					SetBackupMethod(backupMethodName).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			})
			It("Should fail", func() {
				By("Check backup status failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dpv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dpv1alpha1.BackupPhaseFailed))
				})).Should(Succeed())
			})
		})
	})
	//
	//When("with backup repo", func() {
	//	var sp *storagev1alpha1.StorageProvider
	//	var repo *dpv1alpha1.BackupRepo
	//	var repoPVCName string
	//	var backupTool *dpv1alpha1.BackupTool
	//
	//	createBackupPolicy := func(pvcName string, repoName string) *dpv1alpha1.BackupPolicy {
	//		builder := testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
	//			SetBackupRepoName(repoName).
	//			AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
	//			AddDataFilePolicy().
	//			SetPVC(pvcName).
	//			SetBackupToolName(backupTool.Name).
	//		return builder.Create(&testCtx).GetObject()
	//	}
	//
	//	createBackup := func(policy *dpv1alpha1.BackupPolicy, change func(*dpv1alpha1.Backup)) *dpv1alpha1.Backup {
	//		if change == nil {
	//			change = func(*dpv1alpha1.Backup) {} // set nop
	//		}
	//		backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
	//			SetBackupPolicyName(backupPolicyName).
	//			SetBackupMethod(backupMethodName).
	//			Apply(change).
	//			Create(&testCtx).GetObject()
	//		return backup
	//	}
	//
	//	createStorageProvider := func() *storagev1alpha1.StorageProvider {
	//		sp := testapps.CreateCustomizedObj(&testCtx, "backup/storageprovider.yaml",
	//			&storagev1alpha1.StorageProvider{})
	//		// the storage provider controller is not running, so set the status manually
	//		Expect(testapps.ChangeObjStatus(&testCtx, sp, func() {
	//			sp.Status.Phase = storagev1alpha1.StorageProviderReady
	//		})).Should(Succeed())
	//		return sp
	//	}
	//
	//	createRepo := func(change func(repo *dpv1alpha1.BackupRepo)) (*dpv1alpha1.BackupRepo, string) {
	//		repo := testapps.CreateCustomizedObj(&testCtx, "backup/backuprepo.yaml",
	//			&dpv1alpha1.BackupRepo{}, func(obj *dpv1alpha1.BackupRepo) {
	//				obj.Spec.StorageProviderRef = sp.Name
	//				if change != nil {
	//					change(obj)
	//				}
	//			})
	//		var repoPVCName string
	//		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(repo), func(g Gomega, repo *dpv1alpha1.BackupRepo) {
	//			g.Expect(repo.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupRepoReady))
	//			g.Expect(repo.Status.BackupPVCName).ShouldNot(BeEmpty())
	//			repoPVCName = repo.Status.BackupPVCName
	//		})).Should(Succeed())
	//		return repo, repoPVCName
	//	}
	//
	//	BeforeEach(func() {
	//		By("creating backup repo")
	//		sp = createStorageProvider()
	//		repo, repoPVCName = createRepo(nil)
	//
	//		By("creating backup tool")
	//		backupTool = testapps.CreateCustomizedObj(&testCtx, "backup/actionset.yaml",
	//			&dpv1alpha1.ActionSet{}, testapps.RandomizedObjName())
	//
	//		viper.SetDefault(constant.CfgKeyBackupPVCName, "")
	//	})
	//
	//	Context("explicitly specify backup repo", func() {
	//		It("should use the backup repo specified in the policy", func() {
	//			By("creating backup policy and backup")
	//			policy := createBackupPolicy("", repo.Name)
	//			backup := createBackup(policy, nil)
	//			By("checking backup, it should use the PVC from the backup repo")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
	//			})).Should(Succeed())
	//		})
	//
	//		It("should use the backup repo specified in the backup object", func() {
	//			By("creating a second backup repo")
	//			repo2, repoPVCName2 := createRepo(nil)
	//			By("creating backup policy and backup")
	//			policy := createBackupPolicy("", repo.Name)
	//			backup := createBackup(policy, func(backup *dpv1alpha1.Backup) {
	//				if backup.Labels == nil {
	//					backup.Labels = map[string]string{}
	//				}
	//				backup.Labels[dataProtectionBackupRepoKey] = repo2.Name
	//			})
	//			By("checking backup, it should use the PVC from repo2")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName2))
	//			})).Should(Succeed())
	//		})
	//	})
	//
	//	Context("default backup repo", func() {
	//		It("should use the default backup repo if it's not specified", func() {
	//			By("creating backup policy and backup")
	//			policy := createBackupPolicy("", "")
	//			backup := createBackup(policy, nil)
	//			By("checking backup, it should use the PVC from the backup repo")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
	//			})).Should(Succeed())
	//		})
	//
	//		It("should associate the default backup repo with the backup object", func() {
	//			By("creating backup policy and backup")
	//			policy := createBackupPolicy("", "")
	//			backup := createBackup(policy, nil)
	//			By("checking backup labels")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Labels[dataProtectionBackupRepoKey]).Should(BeEquivalentTo(repo.Name))
	//			})).Should(Succeed())
	//
	//			By("creating backup2")
	//			backup2 := createBackup(policy, func(backup *dpv1alpha1.Backup) {
	//				backup.Name += "2"
	//			})
	//			By("checking backup2 labels")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup2), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName))
	//				g.Expect(backup.Labels[dataProtectionBackupRepoKey]).Should(BeEquivalentTo(repo.Name))
	//			})).Should(Succeed())
	//		})
	//
	//		Context("multiple default backup repos", func() {
	//			var repoPVCName2 string
	//			var policy *dpv1alpha1.BackupPolicy
	//			BeforeEach(func() {
	//				By("creating a second backup repo")
	//				sp2 := createStorageProvider()
	//				_, repoPVCName2 = createRepo(func(repo *dpv1alpha1.BackupRepo) {
	//					repo.Spec.StorageProviderRef = sp2.Name
	//				})
	//				By("creating backup policy")
	//				policy = createBackupPolicy("", "")
	//			})
	//
	//			It("should fail if there are multiple default backup repos", func() {
	//				By("creating backup")
	//				backup := createBackup(policy, nil)
	//				By("checking backup, it should fail because there are multiple default backup repos")
	//				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//					g.Expect(backup.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupPhaseFailed))
	//					g.Expect(backup.Status.FailureReason).Should(ContainSubstring("multiple default BackupRepo found"))
	//				})).Should(Succeed())
	//			})
	//
	//			It("should only repos in ready status can be selected as the default backup repo", func() {
	//				By("making repo to failed status")
	//				Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(sp),
	//					func(fetched *storagev1alpha1.StorageProvider) {
	//						fetched.Status.Phase = storagev1alpha1.StorageProviderNotReady
	//					})).ShouldNot(HaveOccurred())
	//				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(repo),
	//					func(g Gomega, repo *dpv1alpha1.BackupRepo) {
	//						g.Expect(repo.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupRepoFailed))
	//					})).Should(Succeed())
	//				By("creating backup")
	//				backup := createBackup(policy, func(backup *dpv1alpha1.Backup) {
	//					backup.Name = "second-backup"
	//				})
	//				By("checking backup, it should use the PVC from repo2")
	//				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//					g.Expect(backup.Status.PersistentVolumeClaimName).Should(BeEquivalentTo(repoPVCName2))
	//				})).Should(Succeed())
	//			})
	//		})
	//	})
	//
	//	Context("no backup repo available", func() {
	//		It("should fallback to the legacy PVC settings", func() {
	//			By("making the backup repo as non-default")
	//			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(repo), func(repo *dpv1alpha1.BackupRepo) {
	//				delete(repo.Annotations, dptypes.DefaultBackupRepoAnnotationKey)
	//			})).Should(Succeed())
	//			By("creating backup")
	//			policy := createBackupPolicy("", "")
	//			backup := createBackup(policy, nil)
	//			By("checking backup, it should fail because neither the backup repo nor the legacy PVC are available")
	//			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(backup), func(g Gomega, backup *dpv1alpha1.Backup) {
	//				g.Expect(backup.Status.Phase).Should(BeEquivalentTo(dpv1alpha1.BackupPhaseFailed))
	//				g.Expect(backup.Status.FailureReason).Should(ContainSubstring("the persistentVolumeClaim name of spec.datafile is empty"))
	//			})).Should(Succeed())
	//		})
	//	})
	//})
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
