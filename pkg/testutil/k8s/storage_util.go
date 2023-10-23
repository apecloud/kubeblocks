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

package testutil

import (
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/testutil"
)

var (
	DefaultStorageClassName        = "default-sc-for-testing"
	defaultVolumeSnapshotClassName = "default-vsc-for-testing"
	defaultProvisioner             = "testing.kubeblocks.io"
)

func GetDefaultStorageClass(testCtx *testutil.TestContext) *storagev1.StorageClass {
	scList := &storagev1.StorageClassList{}
	gomega.Expect(testCtx.Cli.List(testCtx.Ctx, scList)).Should(gomega.Succeed())
	if len(scList.Items) == 0 {
		return nil
	}

	for _, sc := range scList.Items {
		annot := sc.Annotations
		if annot == nil {
			continue
		}
		if isDefaultStorageClassAnnotation(&sc) {
			return &sc
		}
	}
	return nil
}

func isDefaultStorageClassAnnotation(storageClass *storagev1.StorageClass) bool {
	return storageClass.Annotations != nil && storageClass.Annotations[storage.IsDefaultStorageClassAnnotation] == "true"
}

func CreateMockStorageClass(testCtx *testutil.TestContext, storageClassName string) *storagev1.StorageClass {
	sc := getStorageClass(testCtx, storageClassName)
	if sc == nil {
		sc = createStorageClass(testCtx, storageClassName)
	}
	return sc
}

func MockEnableVolumeSnapshot(testCtx *testutil.TestContext, storageClassName string) {
	sc := getStorageClass(testCtx, storageClassName)
	if sc == nil {
		sc = createStorageClass(testCtx, storageClassName)
	}
	vsc := getVolumeSnapshotClass(testCtx, sc)
	if vsc == nil {
		CreateVolumeSnapshotClass(testCtx, sc.Provisioner)
	}
	gomega.Expect(IsMockVolumeSnapshotEnabled(testCtx, storageClassName)).Should(gomega.BeTrue())
}

func MockDisableVolumeSnapshot(testCtx *testutil.TestContext, storageClassName string) {
	sc := getStorageClass(testCtx, storageClassName)
	if sc != nil {
		deleteVolumeSnapshotClass(testCtx, sc)
		deleteStorageClass(testCtx, storageClassName)
	}
}

func IsMockVolumeSnapshotEnabled(testCtx *testutil.TestContext, storageClassName string) bool {
	sc := getStorageClass(testCtx, storageClassName)
	if sc == nil {
		return false
	}
	return getVolumeSnapshotClass(testCtx, sc) != nil
}

func getStorageClass(testCtx *testutil.TestContext, storageClassName string) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{}
	if err := testCtx.Cli.Get(testCtx.Ctx, types.NamespacedName{Name: storageClassName}, sc); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		gomega.Expect(err).Should(gomega.Succeed())
	}
	return sc
}

func createStorageClass(testCtx *testutil.TestContext, storageClassName string) *storagev1.StorageClass {
	allowVolumeExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
		},
		Provisioner:          defaultProvisioner,
		AllowVolumeExpansion: &allowVolumeExpansion,
	}
	gomega.Expect(testCtx.Create(testCtx.Ctx, sc)).Should(gomega.Succeed())
	return sc
}

func deleteStorageClass(testCtx *testutil.TestContext, storageClassName string) {
	sc := getStorageClass(testCtx, storageClassName)
	if sc != nil {
		gomega.Expect(testCtx.Cli.Delete(testCtx.Ctx, sc)).Should(gomega.Succeed())
	}
}

func getVolumeSnapshotClass(testCtx *testutil.TestContext, storageClass *storagev1.StorageClass) *snapshotv1.VolumeSnapshotClass {
	vscList := &snapshotv1.VolumeSnapshotClassList{}
	gomega.Expect(testCtx.Cli.List(testCtx.Ctx, vscList)).Should(gomega.Succeed())
	for i, vsc := range vscList.Items {
		if vsc.Driver == storageClass.Provisioner {
			return &vscList.Items[i]
		}
	}
	return nil
}

func CreateVolumeSnapshotClass(testCtx *testutil.TestContext, storageProvisioner string) *snapshotv1.VolumeSnapshotClass {
	vsc := &snapshotv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultVolumeSnapshotClassName,
		},
		Driver:         storageProvisioner,
		DeletionPolicy: snapshotv1.VolumeSnapshotContentDelete,
	}
	gomega.Expect(testCtx.Create(testCtx.Ctx, vsc)).Should(gomega.Succeed())
	return vsc
}

func deleteVolumeSnapshotClass(testCtx *testutil.TestContext, storageClass *storagev1.StorageClass) {
	vsc := getVolumeSnapshotClass(testCtx, storageClass)
	if vsc != nil {
		gomega.Expect(testCtx.Cli.Delete(testCtx.Ctx, vsc)).Should(gomega.Succeed())
	}
}
