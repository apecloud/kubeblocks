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
	"context"
	"encoding/json"

	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
)

func InVolumeSnapshotV1Beta1() bool {
	return viper.GetBool("VOLUMESNAPSHOT_API_BETA")
}

// VolumeSnapshotCompatClient client is compatible both VolumeSnapshot v1 and v1beta1
type VolumeSnapshotCompatClient struct {
	client.Client
	roclient.ReadonlyClient
	Ctx context.Context
}

func (c *VolumeSnapshotCompatClient) Create(snapshot *snapshotv1.VolumeSnapshot, opts ...client.CreateOption) error {
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1, err := convertV1ToV1beta1(snapshot)
		if err != nil {
			return err
		}
		return c.Client.Create(c.Ctx, snapshotV1Beta1, opts...)
	}
	return c.Client.Create(c.Ctx, snapshot, opts...)
}

func (c *VolumeSnapshotCompatClient) Get(key client.ObjectKey, snapshot *snapshotv1.VolumeSnapshot, opts ...client.GetOption) error {
	if c.ReadonlyClient == nil {
		c.ReadonlyClient = c.Client
	}
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1 := &snapshotv1beta1.VolumeSnapshot{}
		err := c.ReadonlyClient.Get(c.Ctx, key, snapshotV1Beta1, opts...)
		if err != nil {
			return err
		}
		snap, err := convertV1Beta1ToV1(snapshotV1Beta1)
		if err != nil {
			return err
		}
		*snapshot = *snap
		return nil
	}
	return c.ReadonlyClient.Get(c.Ctx, key, snapshot, opts...)
}

func (c *VolumeSnapshotCompatClient) Delete(snapshot *snapshotv1.VolumeSnapshot, opts ...client.DeleteOption) error {
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1, err := convertV1ToV1beta1(snapshot)
		if err != nil {
			return err
		}
		return BackgroundDeleteObject(c.Client, c.Ctx, snapshotV1Beta1)
	}
	return BackgroundDeleteObject(c.Client, c.Ctx, snapshot)
}

func (c *VolumeSnapshotCompatClient) Patch(snapshot *snapshotv1.VolumeSnapshot, deepCopy *snapshotv1.VolumeSnapshot, opts ...client.PatchOption) error {
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1, err := convertV1ToV1beta1(snapshot)
		if err != nil {
			return err
		}
		snapshotV1Beta1Patch, err := convertV1ToV1beta1(deepCopy)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(snapshotV1Beta1Patch)
		return c.Client.Patch(c.Ctx, snapshotV1Beta1, patch, opts...)
	}
	snapPatch := client.MergeFrom(deepCopy)
	return c.Client.Patch(c.Ctx, snapshot, snapPatch, opts...)
}

func (c *VolumeSnapshotCompatClient) List(snapshotList *snapshotv1.VolumeSnapshotList, opts ...client.ListOption) error {
	if c.ReadonlyClient == nil {
		c.ReadonlyClient = c.Client
	}
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1List := &snapshotv1beta1.VolumeSnapshotList{}
		err := c.ReadonlyClient.List(c.Ctx, snapshotV1Beta1List, opts...)
		if err != nil {
			return err
		}
		snaps, err := convertListV1Beta1ToV1(snapshotV1Beta1List)
		if err != nil {
			return err
		}
		*snapshotList = *snaps
		return nil
	}
	return c.ReadonlyClient.List(c.Ctx, snapshotList, opts...)
}

// CheckResourceExists checks whether resource exist or not.
func (c *VolumeSnapshotCompatClient) CheckResourceExists(key client.ObjectKey, obj *snapshotv1.VolumeSnapshot) (bool, error) {
	if err := c.Get(key, obj); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	// if found, return true
	return true, nil
}

func convertV1ToV1beta1(snapshot *snapshotv1.VolumeSnapshot) (*snapshotv1beta1.VolumeSnapshot, error) {
	v1beta1Snapshot := &snapshotv1beta1.VolumeSnapshot{}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(snapshotBytes, v1beta1Snapshot); err != nil {
		return nil, err
	}

	return v1beta1Snapshot, nil
}

func convertV1Beta1ToV1(snapshot *snapshotv1beta1.VolumeSnapshot) (*snapshotv1.VolumeSnapshot, error) {
	v1Snapshot := &snapshotv1.VolumeSnapshot{}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(snapshotBytes, v1Snapshot); err != nil {
		return nil, err
	}

	return v1Snapshot, nil
}

func convertListV1Beta1ToV1(snapshots *snapshotv1beta1.VolumeSnapshotList) (*snapshotv1.VolumeSnapshotList, error) {
	v1Snapshots := &snapshotv1.VolumeSnapshotList{}
	snapshotBytes, err := json.Marshal(snapshots)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(snapshotBytes, v1Snapshots); err != nil {
		return nil, err
	}

	return v1Snapshots, nil
}
