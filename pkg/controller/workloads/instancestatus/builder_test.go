/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instancestatus

import (
	"testing"

	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestBuildDesiredAndCurrentDimensions(t *testing.T) {
	result, err := Build(BuildInput{
		Active: []Allocation{
			{PodName: "demo-0", TemplateName: ""},
			{PodName: "demo-fast-0", TemplateName: "fast"},
			{PodName: "demo-2", TemplateName: "flat"},
		},
		UpdateRevisions: map[string]string{"demo-0": "r2", "demo-fast-0": "r2", "demo-2": "r2"},
		Current: []CurrentObservation{
			{
				InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent, CurrentRevision: "r1",
				Ready: true, Available: true, UpToDate: false, Role: "leader",
			},
			{
				InstanceName: "demo-fast-0", State: workloads.InstanceCurrentStatePresent, CurrentRevision: "r2",
				Ready: true, Available: true, UpToDate: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %#v", result.Statuses)
	}
	assertStatus(t, result.Statuses[0], "demo-0", "", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStatePresent)
	if result.Statuses[0].CurrentRevision != "r1" || result.Statuses[0].UpdateRevision != "r2" || result.Statuses[0].UpToDate || !result.Statuses[0].Ready || !result.Statuses[0].Available {
		t.Fatalf("unexpected present status: %#v", result.Statuses[0])
	}
	assertStatus(t, result.Statuses[1], "demo-2", "flat", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStateAbsent)
	if result.Statuses[1].UpdateRevision != "r2" || result.Statuses[1].CurrentRevision != "" || result.Statuses[1].Ready {
		t.Fatalf("unexpected absent status: %#v", result.Statuses[1])
	}
	assertStatus(t, result.Statuses[2], "demo-fast-0", "fast", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStatePresent)
	if !result.Statuses[2].UpToDate {
		t.Fatalf("expected the observed instance to be up to date: %#v", result.Statuses[2])
	}
}

func TestBuildOfflineLifecycleRetainsOnlyIdentityWhenAbsent(t *testing.T) {
	previous := []workloads.InstanceStatus{{
		PodName:         "demo-fast-0",
		TemplateName:    ptr.To("fast"),
		DesiredState:    workloads.InstanceDesiredStateActive,
		CurrentState:    workloads.InstanceCurrentStatePresent,
		CurrentRevision: "old",
		UpdateRevision:  "new",
		UpToDate:        true,
		Ready:           true,
		Available:       true,
		Failed:          true,
		Role:            "leader",
	}}
	result, err := Build(BuildInput{
		Previous: previous,
		Offline:  []string{"demo-fast-0"},
		Current: []CurrentObservation{{
			InstanceName: "demo-fast-0", State: workloads.InstanceCurrentStatePresent,
			CurrentRevision: "current", UpToDate: true, Ready: true, Available: true, Failed: true, Role: "follower",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := result.Statuses[0]
	assertStatus(t, status, "demo-fast-0", "fast", workloads.InstanceDesiredStateOffline, workloads.InstanceCurrentStatePresent)
	if status.UpdateRevision != "" || status.UpToDate || status.CurrentRevision != "current" || !status.Ready || !status.Available || !status.Failed || status.Role != "follower" {
		t.Fatalf("unexpected Offline+Present status: %#v", status)
	}

	result, err = Build(BuildInput{Previous: result.Statuses, Offline: []string{"demo-fast-0"}})
	if err != nil {
		t.Fatal(err)
	}
	status = result.Statuses[0]
	assertStatus(t, status, "demo-fast-0", "fast", workloads.InstanceDesiredStateOffline, workloads.InstanceCurrentStateAbsent)
	if status.CurrentRevision != "" || status.UpdateRevision != "" || status.UpToDate || status.Ready || status.Available || status.Failed || status.Role != "" {
		t.Fatalf("Offline+Absent retained current fields: %#v", status)
	}
}

func TestBuildReleasedCleanupIsBounded(t *testing.T) {
	previous := []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("")}}
	result, err := Build(BuildInput{
		Previous: previous,
		Current: []CurrentObservation{{
			InstanceName: "demo-0", State: workloads.InstanceCurrentStateTerminating, CurrentRevision: "r1",
			UpToDate: true, Ready: true, Available: true, Failed: true, Role: "old",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := result.Statuses[0]
	if status.DesiredState != workloads.InstanceDesiredStateReleased || status.CurrentState != workloads.InstanceCurrentStateTerminating || status.CurrentRevision != "r1" {
		t.Fatalf("unexpected Released+Terminating status: %#v", status)
	}
	if status.UpToDate || status.Ready || status.Available || status.Failed || status.Role != "" {
		t.Fatalf("terminating instance retained health or runtime fields: %#v", status)
	}

	result, err = Build(BuildInput{Previous: result.Statuses})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 0 {
		t.Fatalf("Released+Absent history was not removed: %#v", result.Statuses)
	}
}

func TestBuildRecomputesCurrentFieldsForSameName(t *testing.T) {
	previous := []workloads.InstanceStatus{{
		PodName: "demo-0", TemplateName: ptr.To(""), DesiredState: workloads.InstanceDesiredStateActive,
		CurrentState: workloads.InstanceCurrentStatePresent, CurrentRevision: "old", UpdateRevision: "old-target",
		UpToDate: true, Ready: true, Available: true, Failed: true, Role: "old-role",
		Configs: []workloads.InstanceConfigStatus{{Name: "old"}}, VolumeExpansion: true,
	}}
	result, err := Build(BuildInput{
		Previous:        previous,
		Active:          []Allocation{{PodName: "demo-0", TemplateName: ""}},
		UpdateRevisions: map[string]string{"demo-0": "new-target"},
		Current: []CurrentObservation{{
			InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent, CurrentRevision: "new",
			Role: "new-role", Configs: []workloads.InstanceConfigStatus{{Name: "new"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := result.Statuses[0]
	if status.CurrentRevision != "new" || status.UpdateRevision != "new-target" || status.Role != "new-role" || len(status.Configs) != 1 || status.Configs[0].Name != "new" {
		t.Fatalf("current fields were not recomputed: %#v", status)
	}
	if status.UpToDate || status.Ready || status.Available || status.Failed || status.VolumeExpansion {
		t.Fatalf("stale boolean fields were retained: %#v", status)
	}
}

func TestBuildNormalizesAvailability(t *testing.T) {
	result, err := Build(BuildInput{
		Active: []Allocation{{PodName: "demo-0"}},
		Current: []CurrentObservation{{
			InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent, Ready: false, Available: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Statuses[0].Available {
		t.Fatalf("Available must be false when Ready is false: %#v", result.Statuses[0])
	}
}

func TestBuildStateCombinations(t *testing.T) {
	tests := []struct {
		name         string
		active       bool
		offline      bool
		current      workloads.InstanceCurrentState
		wantCount    int
		wantDesired  workloads.InstanceDesiredState
		wantTarget   bool
		wantCurrent  bool
		wantRuntime  bool
		wantUpToDate bool
	}{
		{name: "active present", active: true, current: workloads.InstanceCurrentStatePresent, wantCount: 1, wantDesired: workloads.InstanceDesiredStateActive, wantTarget: true, wantCurrent: true, wantRuntime: true, wantUpToDate: true},
		{name: "active terminating", active: true, current: workloads.InstanceCurrentStateTerminating, wantCount: 1, wantDesired: workloads.InstanceDesiredStateActive, wantTarget: true, wantCurrent: true},
		{name: "active absent", active: true, current: workloads.InstanceCurrentStateAbsent, wantCount: 1, wantDesired: workloads.InstanceDesiredStateActive, wantTarget: true},
		{name: "offline present", offline: true, current: workloads.InstanceCurrentStatePresent, wantCount: 1, wantDesired: workloads.InstanceDesiredStateOffline, wantCurrent: true, wantRuntime: true},
		{name: "offline terminating", offline: true, current: workloads.InstanceCurrentStateTerminating, wantCount: 1, wantDesired: workloads.InstanceDesiredStateOffline, wantCurrent: true},
		{name: "offline absent", offline: true, current: workloads.InstanceCurrentStateAbsent, wantCount: 1, wantDesired: workloads.InstanceDesiredStateOffline},
		{name: "released present", current: workloads.InstanceCurrentStatePresent, wantCount: 1, wantDesired: workloads.InstanceDesiredStateReleased, wantCurrent: true, wantRuntime: true},
		{name: "released terminating", current: workloads.InstanceCurrentStateTerminating, wantCount: 1, wantDesired: workloads.InstanceDesiredStateReleased, wantCurrent: true},
		{name: "released absent", current: workloads.InstanceCurrentStateAbsent, wantCount: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := BuildInput{
				Previous:        []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("template")}},
				UpdateRevisions: map[string]string{"demo-0": "target"},
			}
			if test.active {
				input.Active = []Allocation{{PodName: "demo-0", TemplateName: "template"}}
			}
			if test.offline {
				input.Offline = []string{"demo-0"}
			}
			if test.current != workloads.InstanceCurrentStateAbsent {
				input.Current = []CurrentObservation{{
					InstanceName: "demo-0", State: test.current, CurrentRevision: "current", UpToDate: true,
					Ready: true, Available: true, Failed: true, Role: "leader",
					Configs: []workloads.InstanceConfigStatus{{Name: "config"}}, VolumeExpansion: true,
				}}
			}

			result, err := Build(input)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Statuses) != test.wantCount {
				t.Fatalf("got %d statuses, want %d: %#v", len(result.Statuses), test.wantCount, result.Statuses)
			}
			if test.wantCount == 0 {
				return
			}
			status := result.Statuses[0]
			if status.DesiredState != test.wantDesired || status.CurrentState != test.current {
				t.Fatalf("unexpected state pair: %#v", status)
			}
			if (status.UpdateRevision != "") != test.wantTarget {
				t.Fatalf("unexpected target revision: %#v", status)
			}
			if (status.CurrentRevision != "") != test.wantCurrent {
				t.Fatalf("unexpected current revision: %#v", status)
			}
			hasRuntime := status.Ready && status.Available && status.Failed && status.Role == "leader" && len(status.Configs) == 1 && status.VolumeExpansion
			if hasRuntime != test.wantRuntime {
				t.Fatalf("unexpected runtime fields: %#v", status)
			}
			if status.UpToDate != test.wantUpToDate {
				t.Fatalf("unexpected UpToDate: %#v", status)
			}
		})
	}
}

func TestBuildRejectsInconsistentInputs(t *testing.T) {
	tests := []BuildInput{
		{Active: []Allocation{{PodName: "demo-0"}}, Offline: []string{"demo-0"}},
		{Previous: []workloads.InstanceStatus{{PodName: "demo-0"}, {PodName: "demo-0"}}},
		{Active: []Allocation{{PodName: "demo-0"}, {PodName: "demo-0"}}},
		{Current: []CurrentObservation{{InstanceName: "demo-0", State: workloads.InstanceCurrentStateAbsent}}},
		{
			Previous: []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("a")}},
			Offline:  []string{"demo-0"}, TemplateHints: []Allocation{{PodName: "demo-0", TemplateName: "b"}},
		},
	}
	for i, input := range tests {
		if _, err := Build(input); err == nil {
			t.Fatalf("case %d: expected an error", i)
		}
	}
}

func TestBuildActiveAllocationOverridesStaleTemplateHints(t *testing.T) {
	result, err := Build(BuildInput{
		Previous:      []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("old")}},
		Active:        []Allocation{{PodName: "demo-0", TemplateName: "new"}},
		TemplateHints: []Allocation{{PodName: "demo-0", TemplateName: "old"}},
		Current: []CurrentObservation{{
			InstanceName: "demo-0",
			State:        workloads.InstanceCurrentStatePresent,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 1 || result.Statuses[0].TemplateName == nil || *result.Statuses[0].TemplateName != "new" {
		t.Fatalf("active allocation was overridden by a stale hint: %#v", result.Statuses)
	}
}

func TestBuildDoesNotInventUnknownHistoricalTemplate(t *testing.T) {
	result, err := Build(BuildInput{Offline: []string{"historical-offline"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 1 || result.Statuses[0].TemplateName != nil {
		t.Fatalf("unknown template was invented as default: %#v", result.Statuses)
	}
}

func TestInstanceStatusViewHelpers(t *testing.T) {
	its := &workloads.InstanceSet{Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
		{PodName: "old-active"},
		{PodName: "active-absent", DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent},
		{PodName: "active-present", DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent},
		{PodName: "offline", DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStatePresent},
		{PodName: "released", DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStatePresent},
	}}}
	if len(its.ActiveInstanceStatuses()) != 3 || len(its.OfflineInstanceStatuses()) != 1 || len(its.RetainedInstanceStatuses()) != 4 {
		t.Fatal("desired-state helpers returned an unexpected view")
	}
	if len(its.ActivePresentInstanceStatuses()) != 2 || len(its.PresentInstanceStatuses()) != 4 {
		t.Fatal("current-state helpers returned an unexpected view")
	}
	if !its.HasPresentInstance("old-active") || !its.HasPresentInstance("active-present") || !its.HasPresentInstance("offline") || its.HasPresentInstance("active-absent") {
		t.Fatal("HasPresentInstance does not reflect current state")
	}
}

func assertStatus(t *testing.T, status workloads.InstanceStatus, name, template string, desired workloads.InstanceDesiredState, current workloads.InstanceCurrentState) {
	t.Helper()
	if status.PodName != name || status.TemplateName == nil || *status.TemplateName != template || status.DesiredState != desired || status.CurrentState != current {
		t.Fatalf("unexpected status: %#v", status)
	}
}
