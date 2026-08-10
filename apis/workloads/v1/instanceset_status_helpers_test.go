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

import "testing"

func TestInstanceStatusViewHelpers(t *testing.T) {
	its := &InstanceSet{Status: InstanceSetStatus{InstanceStatus: []InstanceStatus{
		{PodName: "legacy-active"},
		{PodName: "active-absent", DesiredState: InstanceDesiredStateActive, CurrentPodState: CurrentPodStateAbsent},
		{PodName: "active-present", DesiredState: InstanceDesiredStateActive, CurrentPodState: CurrentPodStatePresent},
		{PodName: "offline", DesiredState: InstanceDesiredStateOffline, CurrentPodState: CurrentPodStateAbsent},
		{PodName: "released", DesiredState: InstanceDesiredStateReleased, CurrentPodState: CurrentPodStatePresent},
	}}}
	if got := len(its.ActiveInstanceStatuses()); got != 3 {
		t.Fatalf("expected 3 Active entries including legacy empty desiredState, got %d", got)
	}
	if got := len(its.OfflineInstanceStatuses()); got != 1 {
		t.Fatalf("expected 1 Offline entry, got %d", got)
	}
	if got := len(its.RetainedInstanceStatuses()); got != 4 {
		t.Fatalf("expected 4 retained entries, got %d", got)
	}
	if got := len(its.PresentInstanceStatuses()); got != 2 {
		t.Fatalf("expected 2 Present entries, got %d", got)
	}
	if !its.HasPresentPod("active-present") || !its.HasPresentPod("released") || its.HasPresentPod("active-absent") {
		t.Fatal("HasPresentPod does not reflect the current Pod dimension")
	}
	if its.FindInstanceStatus("offline") == nil || its.FindInstanceStatus("missing") != nil {
		t.Fatal("FindInstanceStatus returned an unexpected result")
	}
	if its.FindInstanceStatus("legacy-active").EffectiveCurrentPodState() != CurrentPodStateUnknown {
		t.Fatal("legacy empty currentPodState must remain Unknown")
	}
}
