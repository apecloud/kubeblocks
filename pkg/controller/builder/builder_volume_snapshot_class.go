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

package builder

import (
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
)

type VolumeSnapshotClassBuilder struct {
	BaseBuilder[snapshotv1.VolumeSnapshotClass, *snapshotv1.VolumeSnapshotClass, VolumeSnapshotClassBuilder]
}

func NewVolumeSnapshotClassBuilder(namespace, name string) *VolumeSnapshotClassBuilder {
	builder := &VolumeSnapshotClassBuilder{}
	builder.init(namespace, name, &snapshotv1.VolumeSnapshotClass{}, builder)
	return builder
}

func (builder *VolumeSnapshotClassBuilder) SetDriver(driver string) *VolumeSnapshotClassBuilder {
	builder.get().Driver = driver
	return builder
}

func (builder *VolumeSnapshotClassBuilder) SetDeletionPolicy(policy snapshotv1.DeletionPolicy) *VolumeSnapshotClassBuilder {
	builder.get().DeletionPolicy = policy
	return builder
}
