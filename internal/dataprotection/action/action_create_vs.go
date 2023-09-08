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

package action

import (
	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// CreateVolumeSnapshotAction is an action that creates the volume snapshot.
type CreateVolumeSnapshotAction struct {
	// Name is the Name of the action.
	Name string

	// PersistentVolumeClaims is the list of persistent volume claims to snapshot.
	PersistentVolumeClaims []corev1.PersistentVolumeClaim
}

func (c *CreateVolumeSnapshotAction) GetName() string {
	return c.Name
}

func (c *CreateVolumeSnapshotAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeExec
}

func (c *CreateVolumeSnapshotAction) Execute(ctx Context) (*dpv1alpha1.ActionStatus, error) {
	return nil, nil
}

var _ Action = &CreateVolumeSnapshotAction{}
