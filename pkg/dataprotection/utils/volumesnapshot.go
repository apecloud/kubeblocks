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

	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func SupportsVolumeSnapshotV1() bool {
	return !viper.GetBool("VOLUMESNAPSHOT_API_BETA")
}

// IsVolumeSnapshotEnabled checks if the CSI supports the volume snapshot.
func IsVolumeSnapshotEnabled(ctx context.Context, cli client.Client, pvName string) (bool, error) {
	if len(pvName) == 0 {
		return false, nil
	}
	pv := &corev1.PersistentVolume{}
	if err := cli.Get(ctx, types.NamespacedName{Name: pvName}, pv, inDataContext()); err != nil {
		return false, err
	}
	if pv.Spec.CSI == nil {
		return false, nil
	}
	vsCli := NewCompatClient(cli)
	vscList := vsv1.VolumeSnapshotClassList{}
	if err := vsCli.List(ctx, &vscList, inDataContext()); err != nil {
		return false, err
	}
	for _, vsc := range vscList.Items {
		if vsc.Driver == pv.Spec.CSI.Driver {
			return true, nil
		}
	}
	return false, nil
}

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext()
}
