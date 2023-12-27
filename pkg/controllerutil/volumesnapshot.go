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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func InVolumeSnapshotV1Beta1() bool {
	return viper.GetBool("VOLUMESNAPSHOT_API_BETA")
}

// VolumeSnapshotCompatClient client is compatible with VolumeSnapshot v1 and v1beta1
type VolumeSnapshotCompatClient struct {
	client.Client
	client.Reader
	Ctx context.Context
}

func (c *VolumeSnapshotCompatClient) Create(obj client.Object, opts ...client.CreateOption) error {
	if InVolumeSnapshotV1Beta1() {
		objV1Beta1 := typeofV1Beta1(obj).(client.Object)
		if err := convertObjectBetweenAPIVersion(obj, objV1Beta1); err != nil {
			return err
		}
		return c.Client.Create(c.Ctx, objV1Beta1, opts...)
	}
	return c.Client.Create(c.Ctx, obj, opts...)
}

func (c *VolumeSnapshotCompatClient) Get(key client.ObjectKey, snapshot client.Object, opts ...client.GetOption) error {
	if c.Reader == nil {
		c.Reader = c.Client
	}
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1 := typeofV1Beta1(snapshot).(client.Object)
		err := c.Reader.Get(c.Ctx, key, snapshotV1Beta1, opts...)
		if err != nil {
			return err
		}
		if err = convertObjectBetweenAPIVersion(snapshotV1Beta1, snapshot); err != nil {
			return err
		}
		return nil
	}
	return c.Reader.Get(c.Ctx, key, snapshot, opts...)
}

func (c *VolumeSnapshotCompatClient) Delete(snapshot client.Object) error {
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1 := typeofV1Beta1(snapshot).(client.Object)
		if err := convertObjectBetweenAPIVersion(snapshot, snapshotV1Beta1); err != nil {
			return err
		}
		return BackgroundDeleteObject(c.Client, c.Ctx, snapshotV1Beta1)
	}
	return BackgroundDeleteObject(c.Client, c.Ctx, snapshot)
}

func (c *VolumeSnapshotCompatClient) Patch(snapshot client.Object, deepCopy client.Object, opts ...client.PatchOption) error {
	if InVolumeSnapshotV1Beta1() {
		snapshotV1Beta1 := typeofV1Beta1(snapshot).(client.Object)
		if err := convertObjectBetweenAPIVersion(snapshot, snapshotV1Beta1); err != nil {
			return err
		}
		snapshotV1Beta1Patch := typeofV1Beta1(deepCopy).(client.Object)
		if err := convertObjectBetweenAPIVersion(deepCopy, snapshotV1Beta1Patch); err != nil {
			return err
		}
		patch := client.MergeFrom(snapshotV1Beta1Patch)
		return c.Client.Patch(c.Ctx, snapshotV1Beta1, patch, opts...)
	}
	snapPatch := client.MergeFrom(deepCopy)
	return c.Client.Patch(c.Ctx, snapshot, snapPatch, opts...)
}

func (c *VolumeSnapshotCompatClient) List(objList client.ObjectList, opts ...client.ListOption) error {
	if c.Reader == nil {
		c.Reader = c.Client
	}
	if InVolumeSnapshotV1Beta1() {
		objV1Beta1List := typeofV1Beta1(objList).(client.ObjectList)
		err := c.Reader.List(c.Ctx, objV1Beta1List, opts...)
		if err != nil {
			return err
		}
		if err = convertObjectBetweenAPIVersion(objV1Beta1List, objList); err != nil {
			return err
		}
		return nil
	}
	return c.Reader.List(c.Ctx, objList, opts...)
}

// CheckResourceExists checks whether resource exist or not.
func (c *VolumeSnapshotCompatClient) CheckResourceExists(key client.ObjectKey, obj client.Object) (bool, error) {
	if err := c.Get(key, obj); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	// if found, return true
	return true, nil
}

func convertObjectBetweenAPIVersion[T1 any, T2 any](from T1, to T2) error {
	fromJSONBytes, err := json.Marshal(from)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(fromJSONBytes, to); err != nil {
		return err
	}
	return nil
}

func typeofV1Beta1(v any) any {
	switch v.(type) {
	// object
	case *snapshotv1.VolumeSnapshot:
		return &snapshotv1beta1.VolumeSnapshot{}
	case *snapshotv1.VolumeSnapshotClass:
		return &snapshotv1beta1.VolumeSnapshotClass{}
	case *snapshotv1beta1.VolumeSnapshot:
		return &snapshotv1.VolumeSnapshot{}
	case *snapshotv1beta1.VolumeSnapshotClass:
		return &snapshotv1.VolumeSnapshotClass{}
	// object list
	case *snapshotv1.VolumeSnapshotList:
		return &snapshotv1beta1.VolumeSnapshotList{}
	case *snapshotv1.VolumeSnapshotClassList:
		return &snapshotv1beta1.VolumeSnapshotClassList{}
	case *snapshotv1beta1.VolumeSnapshotList:
		return &snapshotv1.VolumeSnapshotList{}
	case *snapshotv1beta1.VolumeSnapshotClassList:
		return &snapshotv1.VolumeSnapshotClassList{}
	default:
		return nil
	}
}

// IsVolumeSnapshotEnabled checks if the CSI supports the volume snapshot.
func IsVolumeSnapshotEnabled(ctx context.Context, cli client.Client, pvName string) (bool, error) {
	if len(pvName) == 0 {
		return false, nil
	}
	pv := &corev1.PersistentVolume{}
	if err := cli.Get(ctx, types.NamespacedName{Name: pvName}, pv); err != nil {
		return false, err
	}
	if pv.Spec.CSI == nil {
		return false, nil
	}
	vsCli := VolumeSnapshotCompatClient{
		Client: cli,
		Ctx:    ctx,
	}
	vscList := snapshotv1.VolumeSnapshotClassList{}
	if err := vsCli.List(&vscList); err != nil {
		return false, err
	}
	for _, vsc := range vscList.Items {
		if vsc.Driver == pv.Spec.CSI.Driver {
			return true, nil
		}
	}
	return false, nil
}
