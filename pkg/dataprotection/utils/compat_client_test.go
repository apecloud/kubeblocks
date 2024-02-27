/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("VolumeSnapshot compat client", func() {
	const snapName = "test-volumesnapshot-name"

	var (
		pvcName       = "test-pvc-name"
		snapClassName = "test-vsc-name"
	)

	viper.SetDefault("VOLUMESNAPSHOT_API_BETA", "true")

	It("test compat client create/get/list/patch/delete", func() {
		compatClient := NewCompatClient(k8sClient)
		snap := &vsv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapName,
				Namespace: "default",
			},
			Spec: vsv1.VolumeSnapshotSpec{
				Source: vsv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
			},
		}
		snapKey := client.ObjectKeyFromObject(snap)
		snapGet := &vsv1.VolumeSnapshot{}

		By("check object not found")
		exists, err := intctrlutil.CheckResourceExists(ctx, compatClient, snapKey, snapGet)
		Expect(err).Should(BeNil())
		Expect(exists).Should(BeFalse())

		By("create volumesnapshot")
		Expect(compatClient.Create(ctx, snap)).Should(Succeed())

		By("check object exists")
		exists, err = intctrlutil.CheckResourceExists(ctx, compatClient, snapKey, snapGet)
		Expect(err).Should(BeNil())
		Expect(exists).Should(BeTrue())

		By("get volumesnapshot")
		Expect(compatClient.Get(ctx, snapKey, snapGet)).Should(Succeed())
		Expect(snapKey.Name).Should(Equal(snapName))

		By("list volumesnapshots")
		snapList := &vsv1.VolumeSnapshotList{}
		Expect(compatClient.List(ctx, snapList)).Should(Succeed())
		Expect(snapList.Items).ShouldNot(BeEmpty())

		By("patch volumesnapshot")
		snapPatch := client.MergeFrom(snap.DeepCopy())
		snap.Spec.VolumeSnapshotClassName = &snapClassName
		Expect(compatClient.Patch(ctx, snap, snapPatch)).Should(Succeed())
		Expect(compatClient.Get(ctx, snapKey, snapGet)).Should(Succeed())
		Expect(*snapGet.Spec.VolumeSnapshotClassName).Should(Equal(snapClassName))

		By("delete volumesnapshot")
		Expect(compatClient.Delete(ctx, snap)).Should(Succeed())
		Eventually(func() error {
			return compatClient.Get(ctx, snapKey, snapGet)
		}).Should(Satisfy(apierrors.IsNotFound))
	})
})
