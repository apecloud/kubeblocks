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

package component

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("probe_utils", func() {
	const backupPolicyName = "test-backup-policy"
	const backupName = "test-backup-job"
	var backupToolName string

	Context("build restore info ", func() {

		cleanEnv := func() {
			// must wait until resources deleted and no longer exist before the testcases start,
			// otherwise if later it needs to create some new resource objects with the same name,
			// in race conditions, it will find the existence of old objects, resulting failure to
			// create the new objects.
			By("clean resources")

			// delete rest mocked objects
			inNS := client.InNamespace(testCtx.DefaultNamespace)
			ml := client.HasLabels{testCtx.TestObjLabelKey}
			testapps.ClearResources(&testCtx, generics.BackupSignature, inNS, ml)
			testapps.ClearResources(&testCtx, generics.BackupPolicySignature, inNS, ml)
			// non-namespaced
			testapps.ClearResources(&testCtx, generics.BackupToolSignature, ml)
		}

		BeforeEach(func() {
			cleanEnv()
			backupTool := testapps.CreateCustomizedObj(&testCtx, "backup/backuptool.yaml",
				&dataprotectionv1alpha1.BackupTool{}, testapps.RandomizedObjName())
			backupToolName = backupTool.Name
		})

		updateBackupStatus := func(backup *dataprotectionv1alpha1.Backup, backupToolName string, expectPhase dataprotectionv1alpha1.BackupPhase) {
			Expect(testapps.ChangeObjStatus(&testCtx, backup, func() {
				backup.Status.BackupToolName = backupToolName
				backup.Status.PersistentVolumeClaimName = "backup-pvc"
				backup.Status.Phase = expectPhase
				backup.Status.TotalSize = "1Gi"
			})).Should(Succeed())
		}

		It("build restore component from full backup source ", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(backupPolicyName).
				SetBackupType(dataprotectionv1alpha1.BackupTypeFull).
				Create(&testCtx).GetObject()
			updateBackupStatus(backup, backupToolName, dataprotectionv1alpha1.BackupCompleted)
			component := &SynthesizedComponent{
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "mysql",
						},
					},
				},
			}
			By("build init container to recover from full backup")
			Expect(BuildRestoredInfo(reqCtx, k8sClient, testCtx.DefaultNamespace, component, backupName)).Should(Succeed())
			initContainers := component.PodSpec.InitContainers
			Expect(len(initContainers) == 1).Should(BeTrue())
		})

		It("build restore component from snapshot backup source ", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			backup := testapps.NewBackupFactory(testCtx.DefaultNamespace, backupName).
				SetBackupPolicyName(backupPolicyName).
				SetBackupType(dataprotectionv1alpha1.BackupTypeSnapshot).
				Create(&testCtx).GetObject()
			component := &SynthesizedComponent{
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "data",
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								corev1.ReadWriteOnce,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			}

			By("test when backup is not completed")
			updateBackupStatus(backup, backupToolName, dataprotectionv1alpha1.BackupInProgress)
			Expect(BuildRestoredInfo(reqCtx, k8sClient, testCtx.DefaultNamespace, component, backupName).Error()).
				Should(ContainSubstring("is not completed"))

			By("build volumeClaim dataSource when backup is completed")
			updateBackupStatus(backup, "not-exist-backup-tool", dataprotectionv1alpha1.BackupCompleted)
			Expect(BuildRestoredInfo(reqCtx, k8sClient, testCtx.DefaultNamespace, component, backupName)).Should(Succeed())
			vct := component.VolumeClaimTemplates[0]
			snapshotAPIGroup := snapshotv1.GroupName
			expectDataSource := &corev1.TypedLocalObjectReference{
				APIGroup: &snapshotAPIGroup,
				Kind:     constant.VolumeSnapshotKind,
				Name:     backupName,
			}
			Expect(reflect.DeepEqual(expectDataSource, vct.Spec.DataSource)).Should(BeTrue())

			By("error if request storage is less than backup storage")
			component.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("512Mi")
			Expect(BuildRestoredInfo(reqCtx, k8sClient, testCtx.DefaultNamespace, component, backupName)).Should(HaveOccurred())
		})

	})
})
