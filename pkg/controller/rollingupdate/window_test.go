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

package rollingupdate

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestParticipantsRemainStable(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2", "c": "2"}
	got, changed := Participants(owner, revisions, 1, []string{"a", "b", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected initial participant a, got %v", got)
	}
	if !changed {
		t.Fatal("expected initial window to require persistence")
	}

	got, changed = Participants(owner, revisions, 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected participant a after reorder, got %v", got)
	}
	if changed {
		t.Fatal("expected a valid saved window to remain unchanged")
	}

	revisions["a"] = "3"
	got, changed = Participants(owner, revisions, 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected new rollout to select b, got %v", got)
	}
	if !changed {
		t.Fatal("expected a new rollout to require persistence")
	}
}

func TestParticipantsPreserveAdmissionAcrossReplicaChanges(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2", "c": "2"}
	_, _ = Participants(owner, revisions, 1, []string{"a", "b", "c"})

	got, changed := Participants(owner, revisions, 2, []string{"b", "c", "a"})
	if !got.Equal(sets.New("a", "b")) {
		t.Fatalf("expected quota increase to retain a and add b, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota increase to require persistence")
	}

	got, changed = Participants(owner, revisions, 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected quota decrease not to admit c, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota decrease to require persistence")
	}

	got, changed = Participants(owner, revisions, 2, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a", "b")) {
		t.Fatalf("expected quota increase to reuse admitted identities, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota increase to require persistence")
	}
}

func TestParticipantsPreserveAdmissionAcrossNameSetChanges(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"default": "2"}
	_, _ = Participants(owner, revisions, 1, []string{"a", "b", "c"})

	// Scale out and reorder without changing any surviving desired revision.
	got, changed := Participants(owner, revisions, 1, []string{"b", "c", "d", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected scale-out to retain a, got %v", got)
	}
	if changed {
		t.Fatal("expected scale-out alone not to change the window")
	}

	// Scale in an unrelated identity and reorder again.
	got, changed = Participants(owner, revisions, 1, []string{"c", "d", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected scale-in to retain a, got %v", got)
	}
	if changed {
		t.Fatal("expected scale-in alone not to change the window")
	}

	// A desired revision change on a surviving identity starts a new rollout.
	revisions["default"] = "3"
	got, changed = Participants(owner, revisions, 1, []string{"c", "d", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected revision change to rebuild the window with c, got %v", got)
	}
	if !changed {
		t.Fatal("expected revision change to require persistence")
	}
}

func TestParticipantsResetInvalidStateAndRemoveFullWindow(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2", "c": "2"}
	_, _ = Participants(owner, revisions, 1, []string{"a", "b", "c"})

	owner.Annotations[WindowAnnotationKey] = "invalid"
	got, changed := Participants(owner, revisions, 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected invalid state to be rebuilt, got %v", got)
	}
	if !changed {
		t.Fatal("expected invalid state to require persistence")
	}

	owner.Annotations[WindowAnnotationKey] = `{"rolloutID":"` + RolloutID(revisions) +
		`","replicas":1,"participants":["a"]}`
	got, changed = Participants(owner, revisions, 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected an unversioned window to be rebuilt, got %v", got)
	}
	if !changed {
		t.Fatal("expected an unversioned window to require persistence")
	}

	got, changed = Participants(owner, revisions, 3, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a", "b", "c")) {
		t.Fatalf("expected all participants, got %v", got)
	}
	if !changed {
		t.Fatal("expected window removal to require persistence")
	}
	if _, ok := owner.Annotations[WindowAnnotationKey]; ok {
		t.Fatal("expected a full window not to retain the annotation")
	}
}

func TestRolloutIDUsesDesiredRevisionsInsteadOfGeneration(t *testing.T) {
	revisions := map[string]string{"b": "revision-b", "a": "revision-a"}
	rolloutID := RolloutID(revisions)
	if rolloutID != RolloutID(map[string]string{"a": "revision-a", "b": "revision-b"}) {
		t.Fatal("expected rollout ID to be independent of map iteration order")
	}
	if rolloutID == RolloutID(map[string]string{"a": "revision-a", "b": "revision-c"}) {
		t.Fatal("expected a desired revision change to alter the rollout ID")
	}
	if rolloutID == RolloutID(map[string]string{"c": "revision-a", "b": "revision-b"}) {
		t.Fatal("expected a stable template identity change to alter the rollout ID")
	}

	owner := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
	got, changed := Participants(owner, revisions, 1, []string{"a", "b"})
	if !got.Equal(sets.New("a")) || !changed {
		t.Fatalf("expected initial participant a to require persistence, got %v, changed %t", got, changed)
	}

	owner.Generation = 2
	got, changed = Participants(owner, revisions, 1, []string{"b", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected generation-only change to keep participant a, got %v", got)
	}
	if changed {
		t.Fatal("expected generation-only change not to reset the saved window")
	}
}

func TestParticipantsInitializeFromExistingRollout(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2", "c": "2"}
	got, changed := ParticipantsWithInitial(owner, revisions, 1,
		[]string{"b", "c", "a"}, sets.New("a"))
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected existing participant a to be recovered, got %v", got)
	}
	if !changed {
		t.Fatal("expected recovered window to require persistence")
	}

	got, changed = Participants(owner, revisions, 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected recovered participant a to remain stable, got %v", got)
	}
	if changed {
		t.Fatal("expected recovered window to remain unchanged")
	}
}

func TestReset(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, map[string]string{"a": "2", "b": "2"}, 1, []string{"a", "b"})
	if !Reset(owner) {
		t.Fatal("expected a persisted window to be ended")
	}
	if Reset(owner) {
		t.Fatal("expected ending an ended window to be a no-op")
	}

	got, changed := ParticipantsWithInitial(owner, map[string]string{"a": "2", "b": "2"}, 1,
		[]string{"b", "a"}, sets.New("a"))
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected an ended window to start fresh with b, got %v", got)
	}
	if !changed {
		t.Fatal("expected restarting RollingUpdate to replace the end marker")
	}
}
