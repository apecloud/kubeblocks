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

package component

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		})

	})
})
