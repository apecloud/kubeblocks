/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestIsPVCPopulationCompleted(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{}
	require.False(t, IsPVCPopulationCompleted(nil))
	require.False(t, IsPVCPopulationCompleted(pvc))

	pvc.Spec.VolumeName = "bound-before-restore"
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   PersistentVolumeClaimPopulating,
		Status: corev1.ConditionTrue,
		Reason: "Processing",
	}}
	require.False(t, IsPVCPopulationCompleted(pvc))

	pvc.Status.Conditions[0].Reason = ReasonPopulatingSucceed
	require.True(t, IsPVCPopulationCompleted(pvc))
	pvc.Status.Conditions[0].Reason = ReasonPopulatingProvisioned
	require.True(t, IsPVCPopulationCompleted(pvc))
	pvc.Status.Conditions[0].Status = corev1.ConditionFalse
	require.False(t, IsPVCPopulationCompleted(pvc))
}
