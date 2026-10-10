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
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

func TestBuildPreservesTrackedFactsAcrossRuntimeStates(t *testing.T) {
	for _, tracked := range []*bool{nil, ptr.To(false), ptr.To(true)} {
		for _, state := range []workloads.InstanceCurrentState{workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating, workloads.InstanceCurrentStateAbsent} {
			for _, offline := range []bool{false, true} {
				input := Input{Previous: []workloads.InstanceStatus{{PodName: "demo-0", Provisioned: true, DataLoaded: tracked, MemberJoined: tracked,
					CurrentRevision: "stale", Ready: true, Available: true, Failed: true, Role: "stale", Configs: []workloads.InstanceConfigStatus{{Name: "stale"}}, VolumeExpansion: true}}}
				if offline {
					input.Offline = []string{"demo-0"}
				} else {
					input.DesiredAssignments = []TemplateAssignment{{InstanceName: "demo-0"}}
				}
				if state != workloads.InstanceCurrentStateAbsent {
					input.Observations = []Observation{{InstanceName: "demo-0", State: state}}
				}
				result, err := Build(input)
				if err != nil || len(result) != 1 {
					t.Fatalf("state=%s offline=%v: result=%#v err=%v", state, offline, result, err)
				}
				status := result[0]
				if !status.Provisioned || !reflect.DeepEqual(status.DataLoaded, tracked) || !reflect.DeepEqual(status.MemberJoined, tracked) {
					t.Fatalf("tracked facts changed: %#v", status)
				}
				if status.Ready || status.Available || status.Failed || status.Role != "" || len(status.Configs) != 0 || status.VolumeExpansion {
					t.Fatalf("stale runtime fields survived a fresh observation: %#v", status)
				}
				if tracked != nil {
					if status.DataLoaded == tracked || status.MemberJoined == tracked || status.DataLoaded == status.MemberJoined {
						t.Fatal("tracked fact pointers alias the input or each other")
					}
					*status.DataLoaded = !*tracked
					if *status.MemberJoined != *tracked {
						t.Fatal("mutating the returned data fact changed membership")
					}
				}
			}
		}
	}
}

func TestBuildProvisionedRequiresObservedRuntime(t *testing.T) {
	for _, state := range []workloads.InstanceCurrentState{"", workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating} {
		input := Input{DesiredAssignments: []TemplateAssignment{{InstanceName: "demo-0"}}}
		if state != "" {
			input.Observations = []Observation{{InstanceName: "demo-0", State: state}}
		}
		result, err := Build(input)
		if err != nil || len(result) != 1 || result[0].Provisioned != (state != "") {
			t.Fatalf("state=%s: result=%#v err=%v", state, result, err)
		}
		result, err = Build(Input{Previous: result, Offline: []string{"demo-0"}})
		if err != nil || len(result) != 1 || result[0].Provisioned != (state != "") {
			t.Fatalf("provisioned fact changed after runtime disappearance: %#v err=%v", result, err)
		}
	}
}

func TestBuildReleasedRetentionUsesMembershipAndCurrentResources(t *testing.T) {
	previous := []workloads.InstanceStatus{{PodName: "demo-0", Provisioned: true, DataLoaded: ptr.To(false), MemberJoined: ptr.To(true)}}
	result, err := Build(Input{Previous: previous})
	if err != nil || len(result) != 1 || result[0].DesiredState != workloads.InstanceDesiredStateReleased || result[0].CurrentState != workloads.InstanceCurrentStateAbsent || !ptr.Deref(result[0].MemberJoined, false) {
		t.Fatalf("joined membership did not retain Released+Absent: %#v err=%v", result, err)
	}
	result[0].MemberJoined = ptr.To(false)
	result, err = Build(Input{Previous: result})
	if err != nil || len(result) != 0 {
		t.Fatalf("provisioned/data facts retained a released identity after leave: %#v err=%v", result, err)
	}
	result, err = Build(Input{Previous: []workloads.InstanceStatus{{PodName: "demo-0", Provisioned: true, DataLoaded: ptr.To(true)}}})
	if err != nil || len(result) != 0 {
		t.Fatalf("provisioned/data facts retained a released identity with unknown membership: %#v err=%v", result, err)
	}
	for _, resource := range []ResourceObservation{{InstanceName: "demo-0", InstancePresent: true}, {InstanceName: "demo-0", StorageCleanup: true}, {InstanceName: "demo-0"}} {
		result, err = Build(Input{Resources: []ResourceObservation{resource}})
		if err != nil {
			t.Fatal(err)
		}
		if !resource.InstancePresent && !resource.StorageCleanup {
			if len(result) != 0 {
				t.Fatalf("empty resource flags retained a status: %#v", result)
			}
			continue
		}
		if len(result) != 1 || result[0].Provisioned || result[0].CurrentState != workloads.InstanceCurrentStateAbsent || result[0].DesiredState != workloads.InstanceDesiredStateReleased {
			t.Fatalf("resource existence was treated as observed runtime: %#v", result)
		}
	}
}
