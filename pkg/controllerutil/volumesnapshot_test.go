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

package controllerutil

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("VolumeSnapshot compat client", func() {
	const snapName = "test-volumesnapshot-name"

	var (
		pvcName       = "test-pvc-name"
		snapClassName = "test-vsc-name"
	)

	viper.SetDefault("VOLUMESNAPSHOT_API_BETA", "true")

	It("test create/get/list/patch/delete", func() {
		compatClient := VolumeSnapshotCompatClient{Client: k8sClient, Ctx: ctx}
		snap := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapName,
				Namespace: "default",
			},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
			},
		}
		snapKey := rtclient.ObjectKeyFromObject(snap)
		snapGet := &snapshotv1.VolumeSnapshot{}

		By("create volumesnapshot")
		// check object not found
		exists, err := compatClient.CheckResourceExists(snapKey, snapGet)
		Expect(err).Should(BeNil())
		Expect(exists).Should(BeFalse())
		// create
		Expect(compatClient.Create(snap)).Should(Succeed())
		// check object exists
		exists, err = compatClient.CheckResourceExists(snapKey, snapGet)
		Expect(err).Should(BeNil())
		Expect(exists).Should(BeTrue())

		By("get volumesnapshot")
		Expect(compatClient.Get(snapKey, snapGet)).Should(Succeed())
		Expect(snapKey.Name).Should(Equal(snapName))

		By("list volumesnapshots")
		snapList := &snapshotv1.VolumeSnapshotList{}
		Expect(compatClient.List(snapList)).Should(Succeed())
		Expect(snapList.Items).ShouldNot(BeEmpty())

		By("patch volumesnapshot")
		snapPatch := snap.DeepCopy()
		snap.Spec.VolumeSnapshotClassName = &snapClassName
		Expect(compatClient.Patch(snap, snapPatch)).Should(Succeed())
		Expect(compatClient.Get(snapKey, snapGet)).Should(Succeed())
		Expect(*snapGet.Spec.VolumeSnapshotClassName).Should(Equal(snapClassName))

		By("delete volumesnapshot")
		Expect(compatClient.Delete(snap)).Should(Succeed())
		Eventually(func() error {
			return compatClient.Get(snapKey, snapGet)
		}).Should(Satisfy(apierrors.IsNotFound))
	})
})
