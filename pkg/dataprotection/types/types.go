/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package types

import corev1 "k8s.io/api/core/v1"

const (
	PersistentVolumeClaimPopulating corev1.PersistentVolumeClaimConditionType = "Populating"
	ReasonPopulatingSucceed                                                   = "Succeed"
	ReasonPopulatingProvisioned                                               = "Provisioned"
)

var (
	// DefaultBackOffLimit is the default backoff limit for jobs.
	DefaultBackOffLimit = int32(2)
)

// IsPVCPopulationCompleted reports whether the volume populator has verified
// the prepareData/provisioning result and released the helper PVC. A bound PVC
// alone is not sufficient because it may have been bound before restore data
// was prepared.
func IsPVCPopulationCompleted(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	for i := range pvc.Status.Conditions {
		condition := &pvc.Status.Conditions[i]
		if condition.Type != PersistentVolumeClaimPopulating || condition.Status != corev1.ConditionTrue {
			continue
		}
		return condition.Reason == ReasonPopulatingSucceed || condition.Reason == ReasonPopulatingProvisioned
	}
	return false
}
