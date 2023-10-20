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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
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
			actionSet = testdp.NewFakeActionSet(&testCtx)
		})

		initResources := func(volumeBinding storagev1.VolumeBindingMode, useVolumeSnapshotBackup, mockBackupCompleted bool) *corev1.PersistentVolumeClaim {
			By("create storageClass")
			createStorageClass(volumeBinding)

			By("create backup")
			backup := mockBackupForRestore(actionSet.Name, "", mockBackupCompleted, useVolumeSnapshotBackup)

			By("create restore ")
			restore := testdp.NewRestoreactory(testCtx.DefaultNamespace, testdp.RestoreName).
				SetBackup(backup.Name, testCtx.DefaultNamespace).
				SetDataSourceRef(testdp.DataVolumeName, testdp.DataVolumeMountPath).
				Create(&testCtx).GetObject()

			By("create PVC and set spec.dataSourceRef to restore")
			pvc := testapps.NewPersistentVolumeClaimFactory(
				testCtx.DefaultNamespace, pvcName, testdp.ClusterName, testdp.ComponentName, testdp.DataVolumeName).
				SetStorage(storageSize).
				SetStorageClass(testdp.StorageClassName).
				SetDataSourceRef(dptypes.DataprotectionAPIGroup, dptypes.RestoreKind, restore.Name).
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
				claim.Annotations[annSelectedNode] = "test-node"
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				g.Expect(len(tmpPVC.Status.Conditions)).Should(Equal(1))
				g.Expect(tmpPVC.Status.Conditions[0].Type).Should(Equal(PersistentVolumeClaimPopulating))
			})).Should(Succeed())

			By("expect for populate pvc created")
			populatePVCName := getPopulatePVCName(pvc.UID)
			populatePVC := &corev1.PersistentVolumeClaim{}
			Eventually(testapps.CheckObjExists(&testCtx, types.NamespacedName{Namespace: testCtx.DefaultNamespace,
				Name: populatePVCName}, populatePVC, true))

			By("expect for job created")
			Eventually(testapps.List(&testCtx, generics.JobSignature,
				client.MatchingLabels{dprestore.DataProtectionLabelPopulatePVCKey: populatePVCName},
				client.InNamespace(testCtx.DefaultNamespace))).Should(HaveLen(1))

			By("mock to create pv and bind to populate pvc")
			pv := mockPV(populatePVCName)

			By("mock job to succeed")
			jobList := &batchv1.JobList{}
			Expect(k8sClient.List(ctx, jobList,
				client.MatchingLabels{dprestore.DataProtectionLabelPopulatePVCKey: getPopulatePVCName(pvc.UID)},
				client.InNamespace(testCtx.DefaultNamespace))).Should(Succeed())
			testdp.ReplaceK8sJobStatus(&testCtx, client.ObjectKeyFromObject(&jobList.Items[0]), batchv1.JobComplete)

			By("expect for pvc has been populated")
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
				g.Expect(tmpPVC.Status.Conditions[0].Reason).Should(Equal(reasonPopulatingSucceed))
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
				client.MatchingLabels{dprestore.DataProtectionLabelPopulatePVCKey: populatePVCName},
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

			It("test VolumePopulator when it fails", func() {
				pvc := initResources(storagev1.VolumeBindingImmediate, false, false)
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(pvc), func(g Gomega, tmpPVC *corev1.PersistentVolumeClaim) {
					g.Expect(len(tmpPVC.Status.Conditions)).Should(Equal(1))
					g.Expect(tmpPVC.Status.Conditions[0].Reason).Should(Equal(reasonPopulatingFailed))
				})).Should(Succeed())

			})

		})
	})
})
