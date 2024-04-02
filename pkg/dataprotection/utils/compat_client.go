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
	"context"
	"encoding/json"

	vsv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	supportsVolumeSnapshotV1 *bool
	supportsCronJobV1        *bool
)

// CompatClient is compatible with VolumeSnapshot v1 and v1beta1, and
// CronJob v1 and v1beta1.
type CompatClient struct {
	client.Client
}

func NewCompatClient(cli client.Client) *CompatClient {
	return &CompatClient{Client: cli}
}

func (c *CompatClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if shouldConvert(obj) {
		objV1Beta1 := typeofV1Beta1(obj).(client.Object)
		if err := convertObjectBetweenAPIVersion(obj, objV1Beta1); err != nil {
			return err
		}
		return c.Client.Create(ctx, objV1Beta1, opts...)
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *CompatClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if shouldConvert(obj) {
		compatObj := typeofV1Beta1(obj).(client.Object)
		err := c.Client.Get(ctx, key, compatObj, opts...)
		if err != nil {
			return err
		}
		if err = convertObjectBetweenAPIVersion(compatObj, obj); err != nil {
			return err
		}
		return nil
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *CompatClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if shouldConvert(obj) {
		compatObj := typeofV1Beta1(obj).(client.Object)
		if err := convertObjectBetweenAPIVersion(obj, compatObj); err != nil {
			return err
		}
		return controllerutil.BackgroundDeleteObject(c.Client, ctx, compatObj)
	}
	return controllerutil.BackgroundDeleteObject(c.Client, ctx, obj)
}

func (c *CompatClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if shouldConvert(obj) {
		compatObj := typeofV1Beta1(obj).(client.Object)
		if err := convertObjectBetweenAPIVersion(obj, compatObj); err != nil {
			return err
		}
		return c.Client.Patch(ctx, compatObj, patch, opts...)
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *CompatClient) List(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error {
	if shouldConvert(objList) {
		compatObjList := typeofV1Beta1(objList).(client.ObjectList)
		err := c.Client.List(ctx, compatObjList, opts...)
		if err != nil {
			return err
		}
		if err = convertObjectBetweenAPIVersion(compatObjList, objList); err != nil {
			return err
		}
		return nil
	}
	return c.Client.List(ctx, objList, opts...)
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
	case *vsv1.VolumeSnapshot:
		return &vsv1beta1.VolumeSnapshot{}
	case *vsv1.VolumeSnapshotClass:
		return &vsv1beta1.VolumeSnapshotClass{}
	case *batchv1.CronJob:
		return &batchv1beta1.CronJob{}
	// object list
	case *vsv1.VolumeSnapshotList:
		return &vsv1beta1.VolumeSnapshotList{}
	case *vsv1.VolumeSnapshotClassList:
		return &vsv1beta1.VolumeSnapshotClassList{}
	case *batchv1.CronJobList:
		return &batchv1beta1.CronJobList{}
	default:
		return nil
	}
}

func shouldConvert(v any) bool {
	if supportsVolumeSnapshotV1 == nil {
		b := SupportsVolumeSnapshotV1()
		supportsVolumeSnapshotV1 = &b
	}
	if supportsCronJobV1 == nil {
		b := SupportsCronJobV1()
		supportsCronJobV1 = &b
	}
	switch v.(type) {
	case *vsv1.VolumeSnapshot,
		*vsv1.VolumeSnapshotClass,
		*vsv1.VolumeSnapshotList,
		*vsv1.VolumeSnapshotClassList:
		return !(*supportsVolumeSnapshotV1)
	case *batchv1.CronJob,
		*batchv1.CronJobList:
		return !(*supportsCronJobV1)
	default:
		return false
	}
}
