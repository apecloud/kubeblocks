/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/require"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Volume Populator Controller test", func() {
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
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)

		// wait all backup to be deleted, otherwise the controller maybe create
		// job to delete the backup between the ClearResources function delete
		// the job and get the job list, resulting the ClearResources panic.
		Eventually(testapps.List(&testCtx, generics.BackupSignature, inNS)).Should(HaveLen(0))

		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.RestoreSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.JobSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ClusterSignature, true, inNS)

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ActionSetSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StorageClassSignature, true, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	When("volume populator controller test", func() {
		var (
			actionSet   *dpv1alpha1.ActionSet
			pvcName     = "data-mysql-mysql-0"
			storageSize = "20Gi"
			// intreeProvisioner = "kubernetes.io/no-provisioner"
			csiProvisioner = "csi.test.io/provisioner"
		)

		createStorageClass := func(volumeBinding storagev1.VolumeBindingMode) *storagev1.StorageClass {
			storageClass := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: testdp.StorageClassName,
				},
				Provisioner:       csiProvisioner,
				VolumeBindingMode: &volumeBinding,
			}
			Expect(testCtx.CreateObj(testCtx.Ctx, storageClass)).Should(Succeed())
			return storageClass
		}

		BeforeEach(func() {
			By("create actionSet")
			actionSet = testdp.NewFakeActionSet(&testCtx, nil)
		})

		initResources := func(volumeBinding storagev1.VolumeBindingMode, useVolumeSnapshotBackup, mockBackupCompleted bool) *corev1.PersistentVolumeClaim {
			By("create storageClass")
			createStorageClass(volumeBinding)

			By("create backup")
			backup := mockBackupForRestore(actionSet.Name, "", "", mockBackupCompleted, useVolumeSnapshotBackup, "")

			By("create PVC and set spec.dataSourceRef to backup")
			pvc := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
				SetAnnotations(map[string]string{
					constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
					constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
				}).
				SetStorage(storageSize).
				SetStorageClass(testdp.StorageClassName).
				SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
				Create(&testCtx).GetObject()
			return pvc
		}

		mockPV := func(populatePVCName string) *corev1.PersistentVolume {
			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pv",
				},
				Spec: corev1.PersistentVolumeSpec{
					StorageClassName: testdp.StorageClassName,
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Capacity: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse(storageSize),
					},
					ClaimRef: &corev1.ObjectReference{
						Namespace: testCtx.DefaultNamespace,
						Name:      populatePVCName,
					},
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       "kubernetes.io",
							VolumeHandle: "test-volume-handle",
						},
					},
				},
			}
			Expect(testCtx.Create(ctx, pv)).Should(Succeed())
			populatePVC := &corev1.PersistentVolumeClaim{}
			// bind pv
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: populatePVCName, Namespace: testCtx.DefaultNamespace}, populatePVC)).Should(Succeed())
			Expect(testapps.ChangeObj(&testCtx, populatePVC, func(c *corev1.PersistentVolumeClaim) {
				c.Spec.VolumeName = pv.Name
			})).Should(Succeed())
			return pv
		}

		checkJobsSA := func(jobList *batchv1.JobList) {
			for _, job := range jobList.Items {
				Expect(job.Spec.Template.Spec.ServiceAccountName).Should(Equal(viper.GetString(dptypes.CfgKeyWorkerServiceAccountName)))
			}
		}

		findPVCCondition := func(pvc *corev1.PersistentVolumeClaim, conditionType corev1.PersistentVolumeClaimConditionType) *corev1.PersistentVolumeClaimCondition {
			for i := range pvc.Status.Conditions {
				if pvc.Status.Conditions[i].Type == conditionType {
					return &pvc.Status.Conditions[i]
				}
			}
			return nil
		}

		findRestoreCondition := func(restore *dpv1alpha1.Restore, conditionType string) *metav1.Condition {
			for i := range restore.Status.Conditions {
				if restore.Status.Conditions[i].Type == conditionType {
					return &restore.Status.Conditions[i]
				}
			}
			return nil
		}

		testVolumePopulate := func(volumeBinding storagev1.VolumeBindingMode, useVolumeSnapshotBackup bool) {
			pvc := initResources(volumeBinding, useVolumeSnapshotBackup, true)

			pvcKey := client.ObjectKeyFromObject(pvc)
			if volumeBinding == storagev1.VolumeBindingWaitForFirstConsumer {
				By("wait for pvc has selected the node")
				Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					g.Expect(len(tmpPVC.Status.Conditions)).Should(Equal(0))
				})).Should(Succeed())
			}

			By("mock pvc has selected the node")
			Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
				if claim.Annotations == nil {
					claim.Annotations = map[string]string{}
				}
				claim.Annotations[AnnSelectedNode] = "test-node"
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
				g.Expect(populatingCondition).ShouldNot(BeNil())
				g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingProcessing))
				restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
				g.Expect(restoreCondition).ShouldNot(BeNil())
				g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionUnknown))
			})).Should(Succeed())

			By("expect for populate pvc created")
			populatePVCName := getPopulatePVCName(pvc.UID)
			populatePVC := &corev1.PersistentVolumeClaim{}
			Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, populatePVC, true))
			Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, func(g Gomega, restore *dpv1alpha1.Restore) {
				g.Expect(restore.Spec.Backup.Name).Should(Equal(pvc.Spec.DataSourceRef.Name))
				g.Expect(restore.Spec.PrepareDataConfig.DataSourceRef.VolumeSource).Should(Equal(testdp.DataVolumeName))
				g.Expect(restore.OwnerReferences).ShouldNot(BeEmpty())
				g.Expect(restore.OwnerReferences[0].UID).Should(Equal(pvc.UID))
			})).Should(Succeed())

			By("expect for job created")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: populatePVCName},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(1))

			By("mock to create pv and bind to populate pvc")
			pv := mockPV(populatePVCName)

			By("mock job to succeed")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: getPopulatePVCName(pvc.UID)},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			checkJobsSA(jobList)
			testdp.ReplaceK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&jobList.Items[0]), batchv1.JobComplete)

			By("expect for pvc has been populated")
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
				g.Expect(populatingCondition).ShouldNot(BeNil())
				g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingSucceed))
				restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
				g.Expect(restoreCondition).ShouldNot(BeNil())
				g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionTrue))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, func(g Gomega, restore *dpv1alpha1.Restore) {
				condition := findRestoreCondition(restore, dprestore.ConditionTypeRestorePreparedData)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Status).Should(Equal(metav1.ConditionTrue))
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pv), func(g Gomega, tmpPV *corev1.PersistentVolume) {
				g.Expect(tmpPV.Spec.ClaimRef.Name).Should(Equal(pvc.Name))
				g.Expect(tmpPV.Spec.ClaimRef.UID).Should(Equal(pvc.UID))
			})).Should(Succeed())

			// mock pvc.spec.volumeName
			Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
				claim.Spec.VolumeName = pv.Name
			})).Should(Succeed())

			By("expect for resources are cleaned up")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionPopulatePVCLabelKey: populatePVCName},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(0))
			Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, populatePVC, false))
		}

		Context("test volume populator", func() {
			It("test VolumePopulator when volumeBinding of storageClass is WaitForFirstConsumer", func() {
				testVolumePopulate(storagev1.VolumeBindingWaitForFirstConsumer, false)
			})

			It("test VolumePopulator when volumeBinding of storageClass is Immediate", func() {
				testVolumePopulate(storagev1.VolumeBindingImmediate, false)
			})

			It("test VolumePopulator when backup uses volume snapshot", func() {
				testVolumePopulate(storagev1.VolumeBindingWaitForFirstConsumer, true)
			})

			It("infers source target from PVC labels for multi-target backups", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{
						{
							BackupTarget: dpv1alpha1.BackupTarget{
								Name: "other",
								PodSelector: &dpv1alpha1.PodSelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											constant.AppInstanceLabelKey:    "source-cluster",
											constant.KBAppComponentLabelKey: "other",
										},
									},
									Strategy: dpv1alpha1.PodSelectionStrategyAny,
								},
							},
						},
						{
							BackupTarget: dpv1alpha1.BackupTarget{
								Name: "mysql",
								PodSelector: &dpv1alpha1.PodSelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											constant.AppInstanceLabelKey:    "source-cluster",
											constant.KBAppComponentLabelKey: testdp.ComponentName,
										},
									},
									Strategy: dpv1alpha1.PodSelectionStrategyAny,
								},
							},
						},
					}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
					}).
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					g.Expect(restore.Spec.Backup.SourceTargetName).Should(Equal("mysql"))
				})).Should(Succeed())
			})

			It("infers source target pod for all-pod target backups", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{{
						BackupTarget: dpv1alpha1.BackupTarget{
							Name: "mysql",
							PodSelector: &dpv1alpha1.PodSelector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										constant.KBAppComponentLabelKey: testdp.ComponentName,
									},
								},
								Strategy: dpv1alpha1.PodSelectionStrategyAll,
							},
						},
						SelectedTargetPods: []string{"source-mysql-0", "source-mysql-1"},
					}}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
					}).
					AddLabels(constant.KBAppPodNameLabelKey, "target-mysql-1").
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					requiredPolicy := restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection
					g.Expect(restore.Spec.Backup.SourceTargetName).Should(Equal("mysql"))
					g.Expect(requiredPolicy).ShouldNot(BeNil())
					g.Expect(requiredPolicy.DataRestorePolicy).Should(Equal(dpv1alpha1.OneToManyRestorePolicy))
					g.Expect(requiredPolicy.SourceOfOneToMany).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany.TargetPodName).Should(Equal("source-mysql-1"))
				})).Should(Succeed())
			})

			It("uses explicit source target pod annotation when instance template ordinals overlap", func() {
				createStorageClass(storagev1.VolumeBindingImmediate)
				backup := mockBackupForRestore(actionSet.Name, "", "", true, false, "")
				Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
					backup.Status.Target = nil
					backup.Status.Targets = []dpv1alpha1.BackupStatusTarget{{
						BackupTarget: dpv1alpha1.BackupTarget{
							Name: "mysql",
							PodSelector: &dpv1alpha1.PodSelector{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										constant.KBAppComponentLabelKey: testdp.ComponentName,
									},
								},
								Strategy: dpv1alpha1.PodSelectionStrategyAll,
							},
						},
						SelectedTargetPods: []string{"source-mysql-tpl-a-1", "source-mysql-tpl-b-1"},
					}}
				})).Should(Succeed())

				pvc := testapps.NewPersistentVolumeClaimFactory(
					testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
					SetAnnotations(map[string]string{
						constant.RestoreSourceNamespaceAnnotationKey: testCtx.DefaultNamespace,
						constant.RestoreVolumeTemplateAnnotationKey:  testdp.DataVolumeName,
						constant.RestoreParametersAnnotationKey:      fmt.Sprintf(`{"%s":"source-mysql-tpl-b-1"}`, dptypes.SourceTargetPodNameAnnotationKey),
					}).
					AddLabels(constant.KBAppPodNameLabelKey, "target-mysql-tpl-b-1").
					AddLabels(constant.KBAppInstanceTemplateLabelKey, "tpl-b").
					SetStorage(storageSize).
					SetStorageClass(testdp.StorageClassName).
					SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.BackupKind, backup.Name).
					Create(&testCtx).GetObject()

				Eventually(testapps.CheckObj(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
					Name: getPopulatePVCName(pvc.UID)}, func(g Gomega, restore *dpv1alpha1.Restore) {
					requiredPolicy := restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection
					g.Expect(requiredPolicy).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany).ShouldNot(BeNil())
					g.Expect(requiredPolicy.SourceOfOneToMany.TargetPodName).Should(Equal("source-mysql-tpl-b-1"))
				})).Should(Succeed())
			})

			It("restores system account secrets before volume population", func() {
				pvc := initResources(storagev1.VolumeBindingWaitForFirstConsumer, false, true)
				Expect(testCtx.CreateObj(testCtx.Ctx, &kbappsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      testdp.ClusterName,
					},
					Spec: kbappsv1.ClusterSpec{
						TerminationPolicy: kbappsv1.Delete,
					},
				})).Should(Succeed())
				Expect(testCtx.CreateObj(testCtx.Ctx, &kbappsv1.Component{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testCtx.DefaultNamespace,
						Name:      constant.GenerateClusterComponentName(testdp.ClusterName, testdp.ComponentName),
					},
					Spec: kbappsv1.ComponentSpec{
						TerminationPolicy: kbappsv1.Delete,
						CompDef:           testdp.ComponentName,
						Replicas:          1,
					},
				})).Should(Succeed())
				pvcKey := client.ObjectKeyFromObject(pvc)
				backupKey := types.NamespacedName{Namespace: testCtx.DefaultNamespace, Name: pvc.Spec.DataSourceRef.Name}
				encryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
				encryptedPassword, err := encryptor.Encrypt([]byte("restored-password"))
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(testapps.GetAndChangeObj(&testCtx, backupKey, func(backup *dpv1alpha1.Backup) {
					if backup.Annotations == nil {
						backup.Annotations = map[string]string{}
					}
					backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey] = fmt.Sprintf(`{"%s":{"admin":"%s"}}`, testdp.ComponentName, encryptedPassword)
				})).Should(Succeed())

				Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
					if claim.Annotations == nil {
						claim.Annotations = map[string]string{}
					}
					claim.Annotations[AnnSelectedNode] = "test-node"
				})).Should(Succeed())

				secretKey := types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateAccountSecretName(testdp.ClusterName, testdp.ComponentName, "admin"),
				}
				Eventually(testapps.CheckObj(&testCtx, secretKey, func(g Gomega, secret *corev1.Secret) {
					g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte("admin")))
					g.Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("restored-password")))
					g.Expect(secret.Annotations).Should(HaveKeyWithValue(constant.SystemAccountProvisionedAnnotationKey, "true"))
				})).Should(Succeed())
				Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
					g.Expect(restoreCondition).ShouldNot(BeNil())
					g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionUnknown))
				})).Should(Succeed())
			})

			It("test VolumePopulator when it fails", func() {
				pvc := initResources(storagev1.VolumeBindingImmediate, false, false)
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvc), func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					populatingCondition := findPVCCondition(tmpPVC, PersistentVolumeClaimPopulating)
					g.Expect(populatingCondition).ShouldNot(BeNil())
					g.Expect(populatingCondition.Reason).Should(Equal(ReasonPopulatingFailed))
					restoreCondition := findPVCCondition(tmpPVC, corev1.PersistentVolumeClaimConditionType(kbappsv1.ConditionTypeRestore))
					g.Expect(restoreCondition).ShouldNot(BeNil())
					g.Expect(restoreCondition.Status).Should(Equal(corev1.ConditionFalse))
				})).Should(Succeed())

			})

		})
	})
})

func TestResolveSourceTargetPodNameRequiresExplicitMappingForInstanceTemplate(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{},
				Strategy:      dpv1alpha1.PodSelectionStrategyAll,
			},
		},
		SelectedTargetPods: []string{"source-mysql-tpl-a-1", "source-mysql-tpl-b-1"},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-mysql-tpl-b-1",
			Labels: map[string]string{
				constant.KBAppPodNameLabelKey:          "target-mysql-tpl-b-1",
				constant.KBAppInstanceTemplateLabelKey: "tpl-b",
			},
			Annotations: map[string]string{},
		},
	}

	sourcePodName, err := resolveSourceTargetPodName(target, pvc)
	require.Error(t, err)
	require.Empty(t, sourcePodName)

	parameters, err := json.Marshal(map[string]string{
		dptypes.SourceTargetPodNameAnnotationKey: "source-mysql-tpl-b-1",
	})
	require.NoError(t, err)
	pvc.Annotations[constant.RestoreParametersAnnotationKey] = string(parameters)
	sourcePodName, err = resolveSourceTargetPodName(target, pvc)
	require.NoError(t, err)
	require.Equal(t, "source-mysql-tpl-b-1", sourcePodName)
}

func TestRestoreParametersKeepRuntimeSettingsOutOfActionParameters(t *testing.T) {
	parameters := map[string]string{
		"restore-param":                                       "restore-value",
		dptypes.SourceTargetPodNameAnnotationKey:              "source-mysql-0",
		dptypes.VolumeRestorePolicyParameterKey:               string(dpv1alpha1.VolumeClaimRestorePolicySerial),
		dptypes.DeferPostReadyUntilClusterRunningParameterKey: "true",
	}

	actionParameters := restoreActionParameters(parameters)

	require.Equal(t, map[string]string{"restore-param": "restore-value"}, actionParameters)
	policy, err := volumeRestorePolicyFromParameters(parameters)
	require.NoError(t, err)
	require.Equal(t, dpv1alpha1.VolumeClaimRestorePolicySerial, policy)
}

func TestRestoreEnvFromParameters(t *testing.T) {
	envJSON, err := json.Marshal([]corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}})
	require.NoError(t, err)

	env, err := restoreEnvFromParameters(map[string]string{
		dptypes.RestoreEnvParameterKey: string(envJSON),
	})

	require.NoError(t, err)
	require.Equal(t, []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "true"}}, env)
}

func TestDecidePVCRestoreUsesTargetVolumes(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	pvc := newPVCForRestoreDecision("logs", "mysql", "")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)

	pvc = newPVCForRestoreDecision("data", "mysql", "")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreTreatsNilTargetVolumesAsProvisionOnly(t *testing.T) {
	reconciler := &VolumePopulatorReconciler{}
	backup := newBackupForRestoreDecision(nil, nil)
	backup.Status.BackupMethod.TargetVolumes = nil
	pvc := newPVCForRestoreDecision("data", "mysql", "")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.False(t, decision.skipPostReady)
}

func TestDecidePVCRestoreAssignsShardingTargetsByStableComponentOrder(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(3)
	components := []client.Object{
		newComponentForShardingDecision("shard-c"),
		newComponentForShardingDecision("shard-a"),
		newComponentForShardingDecision("shard-b"),
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(append([]client.Object{cluster}, components...)...).
			Build(),
	}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b"})
	pvc := newPVCForRestoreDecision("data", "shard-b", "shard")

	decision, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeRestoreData, decision.mode)
	require.False(t, decision.skipPostReady)
	require.NotNil(t, decision.sourceTarget)
	require.Equal(t, "target-b", decision.sourceTarget.Name)

	pvc = newPVCForRestoreDecision("data", "shard-c", "shard")
	decision, err = reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.NoError(t, err)
	require.Equal(t, pvcRestoreModeProvisionOnly, decision.mode)
	require.True(t, decision.skipPostReady)
	require.Nil(t, decision.sourceTarget)
}

func TestDecidePVCRestoreFailsWhenShardingTargetsExceedShards(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(2)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b", "target-c"})
	pvc := newPVCForRestoreDecision("data", "shard-a", "shard")

	_, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
}

func TestDecidePVCRestoreRequeuesWhenShardingComponentsIncomplete(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	cluster := newClusterForShardingDecision(3)
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).
			WithObjects(cluster, newComponentForShardingDecision("shard-a")).
			Build(),
	}
	backup := newBackupForRestoreDecision([]string{"data"}, []string{"target-a", "target-b"})
	pvc := newPVCForRestoreDecision("data", "shard-a", "shard")

	_, err := reconciler.decidePVCRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup, nil)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
}

func TestProvisionOnlyCreatesPopulatePVCWithoutJob(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "logs-target-0",
			UID:       "logs-target-0-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(pvc).WithObjects(pvc).Build(),
		Scheme:   scheme,
		Recorder: record.NewFakeRecorder(10),
	}
	restoreCtx := &pvcRestoreContext{
		mode: pvcRestoreModeProvisionOnly,
		restoreMgr: dprestore.NewRestoreManager(&dpv1alpha1.Restore{
			Spec: dpv1alpha1.RestoreSpec{Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "default"}},
		}, nil, scheme, reconciler.Client),
	}

	err := reconciler.ProvisionOnly(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreCtx)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
	populatePVC := &corev1.PersistentVolumeClaim{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: getPopulatePVCName(pvc.UID)}, populatePVC))
	jobs := &batchv1.JobList{}
	require.NoError(t, reconciler.Client.List(context.Background(), jobs))
	require.Empty(t, jobs.Items)
}

func TestBuildPostReadyRestoreSelectsHighestPriorityRole(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	compDef := &kbappsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: kbappsv1.ComponentDefinitionSpec{
			Roles: []kbappsv1.ReplicaRole{
				{Name: "follower", UpdatePriority: 1},
				{Name: "leader", UpdatePriority: 10},
			},
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
		},
		Spec: kbappsv1.ComponentSpec{CompDef: compDef.Name},
	}
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{Name: backup.Name}
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, compDef).Build(),
		Scheme: scheme,
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{}, nil, scheme, reconciler.Client)

	restore, err := reconciler.buildPostReadyRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreMgr, comp)

	require.NoError(t, err)
	require.Equal(t, "leader", restore.Spec.ReadyConfig.JobAction.Target.PodSelector.LabelSelector.MatchLabels[instanceset.RoleLabelKey])
	require.NotContains(t, restore.Spec.ReadyConfig.ExecAction.Target.PodSelector.MatchLabels, instanceset.RoleLabelKey)
}

func TestBuildPostReadyRestoreUsesInitAccountFromComponentDefinition(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	compDef := &kbappsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
		Spec: kbappsv1.ComponentDefinitionSpec{
			SystemAccounts: []kbappsv1.SystemAccount{
				{Name: "app"},
				{Name: "root", InitAccount: true},
			},
		},
	}
	comp := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
		},
		Spec: kbappsv1.ComponentSpec{CompDef: compDef.Name},
	}
	pvc := newPVCForRestoreDecision("data", "mysql", "")
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{Name: backup.Name}
	pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = backup.Namespace
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, compDef).Build(),
		Scheme: scheme,
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{}, nil, scheme, reconciler.Client)

	restore, err := reconciler.buildPostReadyRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, restoreMgr, comp)

	require.NoError(t, err)
	require.NotNil(t, restore.Spec.ReadyConfig.ConnectionCredential)
	require.Equal(t, constant.GenerateAccountSecretName("cluster", "mysql", "root"), restore.Spec.ReadyConfig.ConnectionCredential.SecretName)
}

func TestWaitForSerialPredecessorsWaitsForEarlierUnboundPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	apiGroup := dptypes.DataprotectionAPIGroup
	previous := newRestorePVCForSerialTest("data-target-0", "")
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build(),
		Recorder: record.NewFakeRecorder(10),
	}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKeyFromObject(current), current))
	current.Spec.DataSourceRef.APIGroup = &apiGroup

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.Error(t, err)
	require.True(t, intctrlutil.IsRequeueError(err), err.Error())
}

func TestWaitForSerialPredecessorsAllowsAfterEarlierBoundPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	previous := newRestorePVCForSerialTest("data-target-0", "pv-0")
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build()}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.NoError(t, err)
}

func TestWaitForSerialPredecessorsSkipsProvisionOnlyPVC(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	previous := newRestorePVCForSerialTest("logs-target-0", "")
	previous.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs"
	previous.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = "logs"
	current := newRestorePVCForSerialTest("data-target-1", "")
	backup := newBackupForRestoreDecision([]string{"data"}, nil)
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(previous, current, backup).Build()}
	restoreMgr := dprestore.NewRestoreManager(&dpv1alpha1.Restore{
		Spec: dpv1alpha1.RestoreSpec{
			PrepareDataConfig: &dpv1alpha1.PrepareDataConfig{
				VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicySerial,
			},
		},
	}, nil, scheme, reconciler.Client)

	err := reconciler.waitForSerialPredecessors(intctrlutil.RequestCtx{Ctx: context.Background()}, current, restoreMgr)

	require.NoError(t, err)
}

func newRestorePVCForSerialTest(name, volumeName string) *corev1.PersistentVolumeClaim {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			UID:       types.UID(name + "-uid"),
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          "mysql",
				constant.KBAppPodNameLabelKey:            name,
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
			Annotations: map[string]string{
				constant.RestoreSourceKindAnnotationKey:      dptypes.BackupKind,
				constant.RestoreSourceNamespaceAnnotationKey: "default",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: volumeName,
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup: &apiGroup,
				Kind:     dptypes.BackupKind,
				Name:     "backup",
			},
		},
	}
}

func newBackupForRestoreDecision(targetVolumes []string, targets []string) *dpv1alpha1.Backup {
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
		},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{},
			Target: &dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{Name: "target"},
			},
		},
	}
	if targetVolumes != nil {
		backup.Status.BackupMethod.TargetVolumes = &dpv1alpha1.TargetVolumeInfo{Volumes: targetVolumes}
	}
	if targets != nil {
		backup.Status.Target = nil
		for _, target := range targets {
			backup.Status.Targets = append(backup.Status.Targets, dpv1alpha1.BackupStatusTarget{
				BackupTarget: dpv1alpha1.BackupTarget{Name: target},
			})
		}
	}
	return backup
}

func newPVCForRestoreDecision(volumeName, componentName, shardingName string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      volumeName + "-" + componentName + "-0",
			Labels: map[string]string{
				constant.AppManagedByLabelKey:            constant.AppName,
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          componentName,
				constant.VolumeClaimTemplateNameLabelKey: volumeName,
			},
			Annotations: map[string]string{
				constant.RestoreVolumeTemplateAnnotationKey: volumeName,
			},
		},
	}
	if shardingName != "" {
		pvc.Labels[constant.KBAppShardingNameLabelKey] = shardingName
	}
	return pvc
}

func newClusterForShardingDecision(shards int32) *kbappsv1.Cluster {
	return &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
		},
		Spec: kbappsv1.ClusterSpec{
			Shardings: []kbappsv1.ClusterSharding{{
				Name:   "shard",
				Shards: shards,
			}},
		},
	}
}

func newComponentForShardingDecision(componentName string) *kbappsv1.Component {
	return &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-" + componentName,
			Labels: map[string]string{
				constant.AppManagedByLabelKey:      constant.AppName,
				constant.AppInstanceLabelKey:       "cluster",
				constant.KBAppComponentLabelKey:    componentName,
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
	}
}

func TestRebindPVCAndPVWaitsUntilPopulatePVCIsBound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "data-target-0",
			UID:             "target-pvc-uid",
			ResourceVersion: "1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: "backup"},
		},
	}
	populatePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "populate",
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	rebound, err := reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.False(t, rebound)

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv"}}
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv).Build()
	populatePVC.Spec.VolumeName = pv.Name

	rebound, err = reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.True(t, rebound)

	patchedPV := &corev1.PersistentVolume{}
	require.NoError(t, reconciler.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: pv.Name}, patchedPV))
	require.NotNil(t, patchedPV.Spec.ClaimRef)
	require.Equal(t, pvc.Name, patchedPV.Spec.ClaimRef.Name)
	require.Equal(t, pvc.UID, patchedPV.Spec.ClaimRef.UID)
}

func TestRestoreSystemAccountSecretsUsesShardingSecretName(t *testing.T) {
	require.Equal(t, "cluster-shard-admin", systemAccountSecretName(systemAccountSecretScopeSharding, "cluster", "shard", "admin"))
	require.Equal(t, constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
		systemAccountSecretName(systemAccountSecretScopeComponent, "cluster", "mysql", "admin"))
}

func TestRestoreSystemAccountSecretsRestoresComponentAndShardingSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, kbappsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))

	encryptor := intctrlutil.NewEncryptor("")
	componentPassword, err := encryptor.Encrypt([]byte("component-password"))
	require.NoError(t, err)
	shardingPassword, err := encryptor.Encrypt([]byte("sharding-password"))
	require.NoError(t, err)
	accounts, err := json.Marshal(map[string]map[string]string{
		"mysql": {
			"admin": componentPassword,
		},
		"shard": {
			"root": shardingPassword,
		},
	})
	require.NoError(t, err)
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: string(accounts),
			},
		},
	}
	cluster := &kbappsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
			UID:       types.UID("cluster-uid"),
		},
	}
	component := &kbappsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       types.UID("component-uid"),
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:       "cluster",
				constant.KBAppComponentLabelKey:    "mysql",
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, cluster, component).Build(),
		Scheme: scheme,
	}

	err = reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.NoError(t, err)

	componentSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
	}, componentSecret))
	require.Equal(t, []byte("component-password"), componentSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, componentSecret.OwnerReferences, 1)
	require.Equal(t, "Component", componentSecret.OwnerReferences[0].Kind)
	require.Equal(t, component.Name, componentSecret.OwnerReferences[0].Name)

	shardingSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "cluster-shard-root",
	}, shardingSecret))
	require.Equal(t, []byte("sharding-password"), shardingSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, shardingSecret.OwnerReferences, 1)
	require.Equal(t, "Cluster", shardingSecret.OwnerReferences[0].Kind)
	require.Equal(t, cluster.Name, shardingSecret.OwnerReferences[0].Name)
}

func TestRestoreSystemAccountSecretsReturnsFatalForInvalidAccountsPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: "{",
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup).Build()}

	err := reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
}
