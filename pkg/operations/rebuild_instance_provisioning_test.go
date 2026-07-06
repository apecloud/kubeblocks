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

	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// TestRebuildInstanceProvisioned pins the completion contract of the rebuild
// h-scale path: a scaled-out replacement instance that is Available but whose
// scale-out provisioning record (memberJoin/dataLoad in the ITS replicas-status)
// is still open must NOT be treated as successfully rebuilt.
func TestRebuildInstanceProvisioned(t *testing.T) {
	const insName = "test-cluster-kafka-1"

	compDefWithMemberJoin := &appsv1.ComponentDefinition{
		Spec: appsv1.ComponentDefinitionSpec{
			LifecycleActions: &appsv1.ComponentLifecycleActions{
				MemberJoin: &appsv1.Action{},
			},
		},
	}
	compDefWithoutActions := &appsv1.ComponentDefinition{}

	newWorkload := func(unprovisioned sets.Set[string], sourced bool) Workload {
		return &defaultWorkload{
			currentRevisionMap:        map[string]string{insName: "rev-a"},
			notReadySet:               sets.New[string](),
			notAvailableSet:           sets.New[string](),
			failedSet:                 sets.New[string](),
			instanceNames:             sets.New(insName),
			unprovisionedSet:          unprovisioned,
			provisioningStatusSourced: sourced,
		}
	}

	cases := []struct {
		name     string
		workload Workload
		compDef  *appsv1.ComponentDefinition
		want     bool
	}{
		// an open provisioning record (memberJoined=false / dataLoaded=false) means
		// the replacement replica has not finished rebuilding, even if Available.
		{"provisioning record open", newWorkload(sets.New(insName), true), compDefWithMemberJoin, false},
		// a closed (or absent) record with a record source means provisioning is done.
		{"provisioning record closed", newWorkload(sets.New[string](), true), compDefWithMemberJoin, true},
		// unknown source + declared provisioning actions = not closed; no silent fallback.
		{"unknown source with provisioning actions", newWorkload(sets.New[string](), false), compDefWithMemberJoin, false},
		// unknown source without provisioning actions: nothing to wait for.
		{"unknown source without provisioning actions", newWorkload(sets.New[string](), false), compDefWithoutActions, true},
		{"unknown source with nil compDef", newWorkload(sets.New[string](), false), nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rebuildInstanceProvisioned(c.workload, c.compDef, insName)
			if got != c.want {
				t.Fatalf("rebuildInstanceProvisioned(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
