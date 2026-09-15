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

package v1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestInstanceStatusSnapshotValidity(t *testing.T) {
	its := &InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec:       InstanceSetSpec{Replicas: ptr.To[int32](0)},
		Status: InstanceSetStatus{
			ObservedGeneration:               2,
			InstanceStatusObservedGeneration: 1,
		},
	}
	if its.IsInstanceStatusSnapshotValid() || its.IsInstancesReady() {
		t.Fatal("an older allocation snapshot must not be ready")
	}

	its.Status.InstanceStatusObservedGeneration = its.Generation
	if !its.IsInstanceStatusSnapshotValid() || !its.IsInstancesReady() {
		t.Fatal("an observed empty allocation is valid zero work")
	}

	its.Status.InstanceStatus = []InstanceStatus{{PodName: "demo-0", CurrentState: InstanceCurrentStateUnknown}}
	if !its.IsInstanceStatusSnapshotValid() {
		t.Fatal("an unknown child row must not invalidate the complete allocation snapshot")
	}
}
