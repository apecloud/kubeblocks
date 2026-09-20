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

package operations

import (
	"testing"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
)

func TestStopOpsMutuallyExcludeQueueScopes(t *testing.T) {
	runningVolumeExpansion := []opsv1alpha1.OpsRecorder{{
		Type:        opsv1alpha1.VolumeExpansionType,
		QueueBySelf: true,
	}}
	if !mustWaitForOps(&opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.StopType}}, runningVolumeExpansion, OpsBehaviour{QueueByCluster: true}) {
		t.Fatal("Stop did not wait for a running VolumeExpansion")
	}

	runningStop := []opsv1alpha1.OpsRecorder{{Type: opsv1alpha1.StopType}}
	forceScale := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType, Force: true}}
	if !mustWaitForOps(forceScale, runningStop, OpsBehaviour{QueueByCluster: true}) {
		t.Fatal("Force request bypassed a running Stop")
	}

	queuedStop := []opsv1alpha1.OpsRecorder{{Type: opsv1alpha1.StopType, InQueue: true}}
	if !mustWaitForOps(forceScale, queuedStop, OpsBehaviour{QueueByCluster: true}) {
		t.Fatal("Force request bypassed a queued Stop")
	}
}
