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
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Backup Controller test", func() {
	const clusterName = "wesql-cluster"
	const componentName = "replicasets-primary"
	const containerName = "mysql"
	const backupPolicyName = "test-backup-policy"
	const backupRemotePVCName = "backup-remote-pvc"
	const defaultSchedule = "0 3 * * *"
	const defaultTTL = "7d"
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
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.BackupPolicySignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.JobSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.CronJobSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PersistentVolumeClaimSignature, true, inNS)
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
		podGenerateName := clusterName + "-" + componentName
		By("By mocking a pvc belonging to the pod")
		pvc := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+podGenerateName+"-0", clusterName, componentName, "data").
			SetStorage("1Gi").
			Create(&testCtx).GetObject()
		pvcName = pvc.Name

		By("By mocking a pvc belonging to the pod2")
		pvc2 := testapps.NewPersistentVolumeClaimFactory(
			testCtx.DefaultNamespace, "data-"+podGenerateName+"-1", clusterName, componentName, "data").
			SetStorage("1Gi").
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
		BeforeEach(func() {
			By("By creating a backupTool")
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())

			By("By creating a backupPolicy from backupTool: " + backupTool.Name)
			_ = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
				SetTTL(defaultTTL).
				AddSnapshotPolicy().
				SetSchedule(defaultSchedule, true).
				AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
				AddMatchLabels(constant.RoleLabelKey, "leader").
				SetTargetSecretName(clusterName).
				AddHookPreCommand("touch /data/mysql/.restore;sync").
				AddHookPostCommand("rm -f /data/mysql/.restore;sync").
				AddFullPolicy().
				SetBackupToolName(backupTool.Name).
				AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
				AddMatchLabels(constant.RoleLabelKey, "leader").
				SetTargetSecretName(clusterName).
				SetPVC(backupRemotePVCName).
				Create(&testCtx).GetObject()
		})

		Context("creates a full backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
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
				Eventually(testapps.CheckObjExists(&testCtx, backupKey, &batchv1.Job{}, false)).Should(Succeed())
			})

			It("should fail after job fails", func() {
				patchK8sJobStatus(backupKey, batchv1.JobFailed)

				By("Check backup job failed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
				})).Should(Succeed())
			})
		})

		Context("deletes a full backup", func() {
			var backupKey types.NamespacedName

			BeforeEach(func() {
				By("creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)

				By("waiting for finalizers to be added")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, backup *dataprotectionv1alpha1.Backup) {
					g.Expect(backup.GetFinalizers()).ToNot(BeEmpty())
				})).Should(Succeed())

				By("setting backup file path")
				Eventually(testapps.ChangeObjStatus(&testCtx, backup, func() {
					if backup.Status.Manifests == nil {
						backup.Status.Manifests = &dataprotectionv1alpha1.ManifestsStatus{}
					}
					if backup.Status.Manifests.BackupTool == nil {
						backup.Status.Manifests.BackupTool = &dataprotectionv1alpha1.BackupToolManifestsStatus{}
					}
					backup.Status.Manifests.BackupTool.FilePath = "/" + backupName
				})).Should(Succeed())
			})

			It("should create a Job for deleting backup files when being deleted", func() {
				By("deleting a Backup object")
				testapps.DeleteObject(&testCtx, backupKey, &dataprotectionv1alpha1.Backup{})

				By("checking new created Job")
				jobKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      deleteBackupFilesJobNamePrefix + backupName,
				}
				Eventually(testapps.CheckObjExists(&testCtx, jobKey,
					&batchv1.Job{}, true)).Should(Succeed())
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

				By("checking Backup object, it should be retained until the job is done")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dataprotectionv1alpha1.Backup{}, true)).Should(Succeed())

				By("completing the job")
				patchK8sJobStatus(jobKey, batchv1.JobComplete)

				By("checking Backup object, it should be deleted")
				Eventually(testapps.CheckObjExists(&testCtx, backupKey,
					&dataprotectionv1alpha1.Backup{}, false)).Should(Succeed())
			})

			When("BackupPolicy is gone", func() {
				BeforeEach(func() {
					By("deleting BackupPolicy")
					backupPolicyKey := types.NamespacedName{
						Namespace: testCtx.DefaultNamespace,
						Name:      backupPolicyName,
					}
					obj := &dataprotectionv1alpha1.BackupPolicy{}
					testapps.DeleteObject(&testCtx, backupPolicyKey, obj)
					Eventually(testapps.CheckObjExists(&testCtx, backupPolicyKey, obj, false)).Should(Succeed())
				})

				It("should ignore the exception and continue to delete", func() {
					By("deleting a Backup object")
					testapps.DeleteObject(&testCtx, backupKey, &dataprotectionv1alpha1.Backup{})
					By("checking Backup object, it should be deleted")
					Eventually(testapps.CheckObjExists(&testCtx, backupKey,
						&dataprotectionv1alpha1.Backup{}, false)).Should(Succeed())
				})
			})

			When("DeleteBackupFiles job is failed", func() {
				It("should ignore the exception and continue to delete", func() {
					By("deleting a Backup object")
					testapps.DeleteObject(&testCtx, backupKey, &dataprotectionv1alpha1.Backup{})

					By("checking new created Job")
					jobKey := types.NamespacedName{
						Namespace: testCtx.DefaultNamespace,
						Name:      deleteBackupFilesJobNamePrefix + backupName,
					}
					Eventually(testapps.CheckObjExists(&testCtx, jobKey,
						&batchv1.Job{}, true)).Should(Succeed())

					By("setting the job to Failed")
					patchK8sJobStatus(jobKey, batchv1.JobFailed)

					By("checking Backup object, it should be deleted")
					Eventually(testapps.CheckObjExists(&testCtx, backupKey,
						&dataprotectionv1alpha1.Backup{}, false)).Should(Succeed())
				})
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
				backupPolicyKey := types.NamespacedName{Name: backupPolicyName, Namespace: backupKey.Namespace}
				patchBackupPolicySpecBackupStatusUpdates(backupPolicyKey)

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

				logJobKey := types.NamespacedName{Name: backupKey.Name + "-" + strings.ToLower("manifests.backupLog"), Namespace: backupKey.Namespace}
				patchK8sJobStatus(logJobKey, batchv1.JobComplete)

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

	When("with backupTool resources", func() {
		Context("creates a full backup", func() {
			var backupKey types.NamespacedName
			var backupPolicy *dataprotectionv1alpha1.BackupPolicy
			var pathPrefix = "/mysql/backup"
			createBackup := func(backupName string) {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
					SetBackupPolicyName(backupPolicyName).
					SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
					Create(&testCtx).GetObject()
				backupKey = client.ObjectKeyFromObject(backup)
			}

			BeforeEach(func() {
				viper.SetDefault(constant.CfgKeyBackupPVCStorageClass, "")
				By("By creating a backupTool")
				backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
					&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName(),
					func(backupTool *dataprotectionv1alpha1.BackupTool) {
						backupTool.Spec.Resources = nil
					})

				By("By creating a backupPolicy from backupTool: " + backupTool.Name)
				backupPolicy = testapps.NewBackupPolicyFactory(testCtx.DefaultNamespace, backupPolicyName).
					AddAnnotations(constant.BackupDataPathPrefixAnnotationKey, pathPrefix).
					AddFullPolicy().
					SetBackupToolName(backupTool.Name).
					SetSchedule(defaultSchedule, true).
					SetTTL(defaultTTL).
					AddMatchLabels(constant.AppInstanceLabelKey, clusterName).
					SetTargetSecretName(clusterName).
					SetPVC(backupRemotePVCName).
					Create(&testCtx).GetObject()

			})

			It("should succeed after job completes", func() {
				createBackup(backupName)
				patchK8sJobStatus(backupKey, batchv1.JobComplete)
				By("Check backup job completed")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupCompleted))
					g.Expect(fetched.Status.Manifests.BackupTool.FilePath).To(Equal(fmt.Sprintf("/%s%s/%s", backupKey.Namespace, pathPrefix, backupKey.Name)))
				})).Should(Succeed())
			})

			It("creates pvc if the specified pvc not exists", func() {
				createBackup(backupName)
				By("Check pvc created by backup controller")
				Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{
					Name:      backupRemotePVCName,
					Namespace: testCtx.DefaultNamespace,
				}, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
			})

			It("creates pvc if the specified pvc not exists", func() {
				By("set persistentVolumeConfigmap")
				configMapName := "pv-template-configmap"
				Expect(testapps.ChangeObj(&testCtx, backupPolicy, func(tmpObj *dataprotectionv1alpha1.BackupPolicy) {
					tmpObj.Spec.Full.PersistentVolumeClaim.PersistentVolumeConfigMap = &dataprotectionv1alpha1.PersistentVolumeConfigMap{
						Name:      configMapName,
						Namespace: testCtx.DefaultNamespace,
					}
				})).Should(Succeed())

				By("create backup with non existent configmap of pv template")
				createBackup(backupName)
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
					g.Expect(fetched.Status.FailureReason).To(ContainSubstring(fmt.Sprintf(`ConfigMap "%s" not found`, configMapName)))
				})).Should(Succeed())
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapName,
						Namespace: testCtx.DefaultNamespace,
					},
					Data: map[string]string{},
				}
				Expect(testCtx.CreateObj(ctx, configMap)).Should(Succeed())

				By("create backup with the configmap not contains the key 'persistentVolume'")
				createBackup(backupName + "1")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupFailed))
					g.Expect(fetched.Status.FailureReason).To(ContainSubstring("the persistentVolume template is empty in the configMap"))
				})).Should(Succeed())

				By("create backup with the configmap contains the key 'persistentVolume'")
				Expect(testapps.ChangeObj(&testCtx, configMap, func(tmpObj *corev1.ConfigMap) {
					pv := corev1.PersistentVolume{
						Spec: corev1.PersistentVolumeSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteMany,
							},
							Capacity: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
							PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
							PersistentVolumeSource: corev1.PersistentVolumeSource{
								CSI: &corev1.CSIPersistentVolumeSource{
									Driver:       "kubeblocks.com",
									FSType:       "ext4",
									VolumeHandle: pvcName,
								},
							},
						},
					}
					pvString, _ := yaml.Marshal(pv)
					tmpObj.Data = map[string]string{
						"persistentVolume": string(pvString),
					}
				})).Should(Succeed())
				createBackup(backupName + "2")
				Eventually(testapps.CheckObj(&testCtx, backupKey, func(g Gomega, fetched *dataprotectionv1alpha1.Backup) {
					g.Expect(fetched.Status.Phase).To(Equal(dataprotectionv1alpha1.BackupInProgress))
				})).Should(Succeed())

				By("check pvc and pv created by backup controller")
				Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{
					Name:      backupRemotePVCName,
					Namespace: testCtx.DefaultNamespace,
				}, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
				Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{
					Name:      backupRemotePVCName + "-" + testCtx.DefaultNamespace,
					Namespace: testCtx.DefaultNamespace,
				}, &corev1.PersistentVolume{}, true)).Should(Succeed())

			})
		})
	})
	When("with exceptional settings", func() {
		Context("creates a backup with non existent backup policy", func() {
			var backupKey types.NamespacedName
			BeforeEach(func() {
				By("By creating a backup from backupPolicy: " + backupPolicyName)
				backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
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

func patchBackupPolicySpecBackupStatusUpdates(key types.NamespacedName) {
	Eventually(testapps.GetAndChangeObj(&testCtx, key, func(fetched *dataprotectionv1alpha1.BackupPolicy) {
		fetched.Spec.Snapshot.BackupStatusUpdates = []dataprotectionv1alpha1.BackupStatusUpdate{
			{
				Path:          "manifests.backupLog",
				ContainerName: "postgresql",
				Script:        "echo {\"startTime\": \"2023-03-01T00:00:00Z\", \"stopTime\": \"2023-03-01T00:00:00Z\"}",
				UpdateStage:   dataprotectionv1alpha1.PRE,
			},
			{
				Path:          "manifests.backupTool",
				ContainerName: "postgresql",
				Script:        "echo {\"FilePath\": \"/backup/test.file\"}",
				UpdateStage:   dataprotectionv1alpha1.POST,
			},
		}
	})).Should(Succeed())
}
