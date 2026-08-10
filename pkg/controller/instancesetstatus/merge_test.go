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

package instancesetstatus

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
		Current: []CurrentObservation{
			{InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent},
			{InstanceName: "demo-fast-0", State: workloads.InstanceCurrentStatePresent},
		},
		Runtime: map[string]RuntimeStatus{
			"demo-0": {Role: "leader"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %#v", result.Statuses)
	}
	assertStatus(t, result.Statuses[0], "demo-0", "", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStatePresent)
	assertStatus(t, result.Statuses[1], "demo-2", "flat", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStateAbsent)
	assertStatus(t, result.Statuses[2], "demo-fast-0", "fast", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStatePresent)
}

func TestBuildOfflineLifecycleRetainsIdentityAndTemplate(t *testing.T) {
	previous := []workloads.InstanceStatus{{
		PodName:      "demo-fast-0",
		TemplateName: ptr.To("fast"),
		DesiredState: workloads.InstanceDesiredStateActive,
		CurrentState: workloads.InstanceCurrentStatePresent,
		Role:         "leader",
	}}
	states := []workloads.InstanceCurrentState{
		workloads.InstanceCurrentStatePresent,
		workloads.InstanceCurrentStateTerminating,
		workloads.InstanceCurrentStateAbsent,
	}
	for _, state := range states {
		input := BuildInput{Previous: previous, Offline: []string{"demo-fast-0"}}
		if state != workloads.InstanceCurrentStateAbsent {
			input.Current = []CurrentObservation{{InstanceName: "demo-fast-0", State: state}}
		}
		result, err := Build(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Statuses) != 1 {
			t.Fatalf("state %s: expected retained status, got %#v", state, result.Statuses)
		}
		assertStatus(t, result.Statuses[0], "demo-fast-0", "fast", workloads.InstanceDesiredStateOffline, state)
		if result.Statuses[0].Role != "" {
			t.Fatalf("state %s: stale runtime role was retained", state)
		}
		previous = result.Statuses
	}
}

func TestBuildReleasedCleanupIsBounded(t *testing.T) {
	previous := []workloads.InstanceStatus{
		{PodName: "demo-0", TemplateName: ptr.To("")},
		{PodName: "demo-1", TemplateName: ptr.To("")},
	}
	result, err := Build(BuildInput{
		Previous: previous,
		Current: []CurrentObservation{
			{InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent},
			{InstanceName: "demo-1", State: workloads.InstanceCurrentStateTerminating},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 2 {
		t.Fatalf("expected released Pods to remain, got %#v", result.Statuses)
	}
	for _, status := range result.Statuses {
		if status.DesiredState != workloads.InstanceDesiredStateReleased {
			t.Fatalf("expected Released, got %#v", status)
		}
	}

	result, err = Build(BuildInput{Previous: result.Statuses})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 0 {
		t.Fatalf("Released+Absent history was not removed: %#v", result.Statuses)
	}
	for i := 0; i < 20; i++ {
		result, err = Build(BuildInput{Previous: result.Statuses})
		if err != nil || len(result.Statuses) != 0 {
			t.Fatalf("iteration %d grew history: %#v, %v", i, result.Statuses, err)
		}
	}
}

func TestBuildRecomputesRuntimeForSameNamePod(t *testing.T) {
	previous := []workloads.InstanceStatus{{
		PodName:         "demo-0",
		TemplateName:    ptr.To(""),
		DesiredState:    workloads.InstanceDesiredStateActive,
		CurrentState:    workloads.InstanceCurrentStatePresent,
		Role:            "old-role",
		Configs:         []workloads.InstanceConfigStatus{{Name: "old"}},
		VolumeExpansion: true,
	}}
	result, err := Build(BuildInput{
		Previous: previous,
		Active:   []Allocation{{PodName: "demo-0", TemplateName: ""}},
		Current:  []CurrentObservation{{InstanceName: "demo-0", State: workloads.InstanceCurrentStatePresent}},
		Runtime: map[string]RuntimeStatus{
			"demo-0": {Role: "new-role", Configs: []workloads.InstanceConfigStatus{{Name: "new"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	status := result.Statuses[0]
	if status.Role != "new-role" || len(status.Configs) != 1 || status.Configs[0].Name != "new" || status.VolumeExpansion {
		t.Fatalf("runtime fields were not recomputed: %#v", status)
	}

	result, err = Build(BuildInput{Previous: result.Statuses, Active: []Allocation{{PodName: "demo-0", TemplateName: ""}}})
	if err != nil {
		t.Fatal(err)
	}
	status = result.Statuses[0]
	if status.CurrentState != workloads.InstanceCurrentStateAbsent || status.Role != "" || status.Configs != nil || status.VolumeExpansion {
		t.Fatalf("Absent Pod retained runtime state: %#v", status)
	}
}

func TestBuildRejectsInconsistentInputsAtomically(t *testing.T) {
	tests := []struct {
		name  string
		input BuildInput
	}{
		{name: "active offline overlap", input: BuildInput{Active: []Allocation{{PodName: "demo-0"}}, Offline: []string{"demo-0"}}},
		{name: "duplicate old status", input: BuildInput{Previous: []workloads.InstanceStatus{{PodName: "demo-0"}, {PodName: "demo-0"}}}},
		{name: "duplicate active", input: BuildInput{Active: []Allocation{{PodName: "demo-0"}, {PodName: "demo-0"}}}},
		{name: "conflicting active template", input: BuildInput{Active: []Allocation{{PodName: "demo-0", TemplateName: "a"}, {PodName: "demo-0", TemplateName: "b"}}}},
		{name: "conflicting retained template", input: BuildInput{
			Previous:      []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("a")}},
			Offline:       []string{"demo-0"},
			TemplateHints: []Allocation{{PodName: "demo-0", TemplateName: "b"}},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Build(test.input); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestBuildLegacyStatusMigration(t *testing.T) {
	result, err := Build(BuildInput{
		Previous: []workloads.InstanceStatus{{PodName: "demo-0", Role: "stale"}},
		Active:   []Allocation{{PodName: "demo-0", TemplateName: ""}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, result.Statuses[0], "demo-0", "", workloads.InstanceDesiredStateActive, workloads.InstanceCurrentStateAbsent)
	if result.Statuses[0].Role != "" {
		t.Fatalf("legacy runtime field was retained: %#v", result.Statuses[0])
	}
}

func TestBuildActiveUsesCurrentAuthoritativeTemplate(t *testing.T) {
	result, err := Build(BuildInput{
		Previous: []workloads.InstanceStatus{{PodName: "demo-0", TemplateName: ptr.To("old")}},
		Active:   []Allocation{{PodName: "demo-0", TemplateName: "current"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Statuses[0].TemplateName == nil || *result.Statuses[0].TemplateName != "current" {
		t.Fatalf("active template did not use the current allocation: %#v", result.Statuses[0])
	}
}

func TestBuildDoesNotInventUnknownHistoricalTemplate(t *testing.T) {
	result, err := Build(BuildInput{Offline: []string{"legacy-offline"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Statuses) != 1 || result.Statuses[0].TemplateName != nil {
		t.Fatalf("unknown template was invented as default: %#v", result.Statuses)
	}
	if len(result.UnknownTemplateNames) != 1 || result.UnknownTemplateNames[0] != "legacy-offline" {
		t.Fatalf("unknown template was not reported: %#v", result.UnknownTemplateNames)
	}
}

func assertStatus(t *testing.T, status workloads.InstanceStatus, name, template string, desired workloads.InstanceDesiredState, current workloads.InstanceCurrentState) {
	t.Helper()
	if status.PodName != name || status.TemplateName == nil || *status.TemplateName != template || status.DesiredState != desired || status.CurrentState != current {
		t.Fatalf("unexpected status: %#v", status)
	}
}
