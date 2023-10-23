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
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockVolumeSnapshotFactory struct {
	testapps.BaseFactory[vsv1.VolumeSnapshot, *vsv1.VolumeSnapshot, MockVolumeSnapshotFactory]
}

func NewVolumeSnapshotFactory(namespace, name string) *MockVolumeSnapshotFactory {
	f := &MockVolumeSnapshotFactory{}
	f.Init(namespace, name,
		&vsv1.VolumeSnapshot{
			Spec: vsv1.VolumeSnapshotSpec{},
		}, f)
	return f
}

func (f *MockVolumeSnapshotFactory) SetSourcePVCName(name string) *MockVolumeSnapshotFactory {
	f.Get().Spec.Source.PersistentVolumeClaimName = &name
	return f
}
