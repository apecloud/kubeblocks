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

package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/action"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testdp "github.com/apecloud/kubeblocks/pkg/testutil/dataprotection"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("CreateVolumeSnapshotAction Test", func() {
	const (
		actionName = "test-create-vs-action"
		pvcName    = "test-pvc"
		volumeName = "test-volume"
	)

	cleanEnv := func() {
		By("clean resources")
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.BackupSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.VolumeSnapshotSignature, true, inNS)
	}

	BeforeEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, testdp.KBToolImage)
	})

	AfterEach(func() {
		cleanEnv()
		viper.Set(constant.KBToolsImage, "")
	})

	Context("create action that create volume snapshot", func() {
		It("should return error when PVC is empty", func() {
			act := &action.CreateVolumeSnapshotAction{}
			status, err := act.Execute(buildActionCtx())
			Expect(err).To(HaveOccurred())
			Expect(status.Phase).Should(Equal(dpv1alpha1.ActionPhaseFailed))
		})

		It("should success to execute action", func() {
			pvc := testdp.NewFakePVC(&testCtx, pvcName)
			Expect(testapps.ChangeObj(&testCtx, pvc, func(claim *corev1.PersistentVolumeClaim) {
				claim.Spec.VolumeName = volumeName
			})).Should(Succeed())
			act := &action.CreateVolumeSnapshotAction{
				Name:  actionName,
				Owner: testdp.NewFakeBackup(&testCtx, nil),
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      actionName,
				},
				PersistentVolumeClaimWrappers: []action.PersistentVolumeClaimWrapper{
					{
						PersistentVolumeClaim: *pvc,
						VolumeName:            volumeName,
					},
				},
			}

			// mock pv
			testapps.NewPersistentVolumeFactory(testCtx.DefaultNamespace, volumeName, pvcName).
				SetCSIDriver(testutil.DefaultCSIDriver).SetStorage("1Gi").Create(&testCtx)

			By("execute action, its status should be running")
			status, err := act.Execute(buildActionCtx())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Phase).Should(Equal(dpv1alpha1.ActionPhaseRunning))

			By("check volume snapshot be created")
			key := client.ObjectKey{
				Namespace: testCtx.DefaultNamespace,
				Name:      dputils.GetBackupVolumeSnapshotName(actionName, volumeName),
			}
			Eventually(testapps.CheckObjExists(&testCtx, key, &vsv1.VolumeSnapshot{}, true)).Should(Succeed())
		})
	})
})
