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

package internal

import (
	"context"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"

	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// check volume snapshot available
func isSnapshotAvailable(cli types2.ReadonlyClient, ctx context.Context) bool {
	if !viper.GetBool("VOLUMESNAPSHOT") {
		return false
	}
	vsList := snapshotv1.VolumeSnapshotList{}
	compatClient := intctrlutil.VolumeSnapshotCompatClient{ReadonlyClient: cli, Ctx: ctx}
	getVSErr := compatClient.List(&vsList)
	return getVSErr == nil
}
