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
	got, changed := Participants(owner, "2", 1, []string{"a", "b", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected initial participant a, got %v", got)
	}
	if !changed {
		t.Fatal("expected initial window to require persistence")
	}

	got, changed = Participants(owner, "2", 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected participant a after reorder, got %v", got)
	}
	if changed {
		t.Fatal("expected a valid saved window to remain unchanged")
	}

	got, changed = Participants(owner, "3", 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected new rollout to select b, got %v", got)
	}
	if !changed {
		t.Fatal("expected a new rollout to require persistence")
	}
}

func TestParticipantsPreserveAdmissionAcrossReplicaChanges(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, "2", 1, []string{"a", "b", "c"})

	got, changed := Participants(owner, "2", 2, []string{"b", "c", "a"})
	if !got.Equal(sets.New("a", "b")) {
		t.Fatalf("expected quota increase to retain a and add b, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota increase to require persistence")
	}

	got, changed = Participants(owner, "2", 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected quota decrease not to admit c, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota decrease to require persistence")
	}

	got, changed = Participants(owner, "2", 2, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a", "b")) {
		t.Fatalf("expected quota increase to reuse admitted identities, got %v", got)
	}
	if !changed {
		t.Fatal("expected quota increase to require persistence")
	}
}

func TestParticipantsPreserveAdmissionAcrossNameSetChanges(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, "2", 1, []string{"a", "b", "c"})

	// Scale out and reorder without changing any surviving desired revision.
	got, changed := Participants(owner, "2", 1, []string{"b", "c", "d", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected scale-out to retain a, got %v", got)
	}
	if changed {
		t.Fatal("expected scale-out alone not to change the window")
	}

	// Scale in an unrelated identity and reorder again.
	got, changed = Participants(owner, "2", 1, []string{"c", "d", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected scale-in to retain a, got %v", got)
	}
	if changed {
		t.Fatal("expected scale-in alone not to change the window")
	}

	// A desired revision change on a surviving identity starts a new rollout.
	got, changed = Participants(owner, "3", 1, []string{"c", "d", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected revision change to rebuild the window with c, got %v", got)
	}
	if !changed {
		t.Fatal("expected revision change to require persistence")
	}
}

func TestParticipantsResetInvalidStateAndRemoveFullWindow(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, "2", 1, []string{"a", "b", "c"})

	owner.Annotations[WindowAnnotationKey] = "invalid"
	got, changed := Participants(owner, "2", 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected invalid state to be rebuilt, got %v", got)
	}
	if !changed {
		t.Fatal("expected invalid state to require persistence")
	}

	owner.Annotations[WindowAnnotationKey] = `{"version":1,"rolloutID":"2","replicas":1,"participants":["a"]}`
	got, changed = Participants(owner, "2", 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected a previous-version window to be rebuilt, got %v", got)
	}
	if !changed {
		t.Fatal("expected a previous-version window to require persistence")
	}

	got, changed = Participants(owner, "2", 3, []string{"c", "b", "a"})
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

func TestRolloutIDUsesDesiredRevisions(t *testing.T) {
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
}

func TestUpdateRolloutIDTracksDesiredChangesOnSurvivingInstances(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	previous := map[string]string{"a": "revision-a", "b": "revision-b"}
	id := UpdateRolloutID(owner, nil, previous)
	if id == "" || owner.Annotations[RolloutIDAnnotationKey] != id {
		t.Fatal("expected the initial rollout ID to be persisted")
	}

	scaleOut := map[string]string{"a": "revision-a", "b": "revision-b", "c": "revision-c"}
	if got := UpdateRolloutID(owner, previous, scaleOut); got != id {
		t.Fatal("expected scale-out alone to preserve the rollout ID")
	}
	scaleIn := map[string]string{"a": "revision-a", "c": "revision-c"}
	if got := UpdateRolloutID(owner, scaleOut, scaleIn); got != id {
		t.Fatal("expected scale-in alone to preserve the rollout ID")
	}

	changed := map[string]string{"a": "revision-a2", "c": "revision-c"}
	nextID := UpdateRolloutID(owner, scaleIn, changed)
	if nextID == id {
		t.Fatal("expected a desired revision change to advance the rollout ID")
	}

	reassigned := map[string]string{"a": "revision-c", "c": "revision-a2"}
	if got := UpdateRolloutID(owner, changed, reassigned); got == nextID {
		t.Fatal("expected ordinal reassignment to advance the rollout ID")
	}
}

func TestLegacyParticipantsDeferStableAdmissionUntilNextRollout(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2", "c": "2"}
	rolloutID := UpdateLegacyRolloutID(owner, revisions, revisions, "basis-2", false)
	got, changed := Participants(owner, rolloutID, 1, []string{"b", "c", "a"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected the legacy first participant b, got %v", got)
	}
	if !changed {
		t.Fatal("expected the deferred quota to require persistence")
	}

	got, changed = Participants(owner, rolloutID, 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected the current legacy rollout to keep first-N behavior, got %v", got)
	}
	if changed {
		t.Fatal("expected a reorder not to rewrite the deferred baseline")
	}

	nextRevisions := map[string]string{"a": "3", "b": "3", "c": "3"}
	nextRolloutID := UpdateLegacyRolloutID(owner, revisions, nextRevisions, "basis-3", false)
	got, changed = Participants(owner, nextRolloutID, 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) || !changed {
		t.Fatalf("expected the next rollout to persist participant c, got %v, changed %t", got, changed)
	}

	got, changed = Participants(owner, nextRolloutID, 1, []string{"b", "c", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected the next rollout participant c to remain stable, got %v", got)
	}
	if changed {
		t.Fatal("expected the stable window to remain unchanged after reorder")
	}
}

func TestUpdateLegacyRolloutIDUsesFullBasisAndReassignment(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	initial := map[string]string{"a": "revision-a"}
	id := UpdateLegacyRolloutID(owner, nil, initial, "basis-1", false)

	scaleOut := map[string]string{"a": "revision-a", "b": "revision-b"}
	if got := UpdateLegacyRolloutID(owner, initial, scaleOut, "basis-1", false); got != id {
		t.Fatal("expected scale-out with an unchanged basis to preserve the rollout ID")
	}

	changed := map[string]string{"a": "revision-a2", "b": "revision-b2"}
	nextID := UpdateLegacyRolloutID(owner, scaleOut, changed, "basis-2", false)
	if nextID == id {
		t.Fatal("expected a full desired template change to advance the rollout ID")
	}

	reassigned := map[string]string{"a": "revision-b2", "b": "revision-a2"}
	if got := UpdateLegacyRolloutID(owner, changed, reassigned, "basis-2", true); got == nextID {
		t.Fatal("expected a surviving ordinal reassignment to advance the rollout ID")
	}
}

func TestNewLegacyObjectStartsWithStableAdmission(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	revisions := map[string]string{"a": "2", "b": "2"}
	rolloutID := UpdateLegacyRolloutID(owner, nil, revisions, "basis-2", false)
	got, changed := Participants(owner, rolloutID, 1, []string{"a", "b"})
	if !got.Equal(sets.New("a")) || !changed {
		t.Fatalf("expected a new object to persist participant a, got %v, changed %t", got, changed)
	}

	got, changed = Participants(owner, rolloutID, 1, []string{"b", "a"})
	if !got.Equal(sets.New("a")) || changed {
		t.Fatalf("expected participant a to remain stable, got %v, changed %t", got, changed)
	}
}

func TestReset(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, "2", 1, []string{"a", "b"})
	if !Reset(owner) {
		t.Fatal("expected a persisted window to be ended")
	}
	if Reset(owner) {
		t.Fatal("expected ending an ended window to be a no-op")
	}

	got, changed := Participants(owner, "2", 1, []string{"b", "a"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected an ended window to start fresh with b, got %v", got)
	}
	if !changed {
		t.Fatal("expected restarting RollingUpdate to replace the end marker")
	}
}
