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

func TestParticipantsResetForReplicaChangeAndInvalidState(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	_, _ = Participants(owner, "2", 1, []string{"a", "b", "c"})

	got, changed := Participants(owner, "2", 2, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b", "a")) {
		t.Fatalf("expected replica change to rebuild window, got %v", got)
	}
	if !changed {
		t.Fatal("expected replica change to require persistence")
	}

	owner.Annotations[WindowAnnotationKey] = "invalid"
	got, changed = Participants(owner, "2", 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected invalid state to be rebuilt, got %v", got)
	}
	if !changed {
		t.Fatal("expected invalid state to require persistence")
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

func TestRolloutIDUsesDesiredRevisionsInsteadOfGeneration(t *testing.T) {
	revisions := map[string]string{"b": "revision-b", "a": "revision-a"}
	rolloutID := RolloutID(revisions)
	if rolloutID != RolloutID(map[string]string{"a": "revision-a", "b": "revision-b"}) {
		t.Fatal("expected rollout ID to be independent of map iteration order")
	}
	if rolloutID == RolloutID(map[string]string{"a": "revision-a", "b": "revision-c"}) {
		t.Fatal("expected a desired revision change to alter the rollout ID")
	}

	owner := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Generation: 1}}
	got, changed := Participants(owner, rolloutID, 1, []string{"a", "b"})
	if !got.Equal(sets.New("a")) || !changed {
		t.Fatalf("expected initial participant a to require persistence, got %v, changed %t", got, changed)
	}

	owner.Generation = 2
	got, changed = Participants(owner, RolloutID(revisions), 1, []string{"b", "a"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected generation-only change to keep participant a, got %v", got)
	}
	if changed {
		t.Fatal("expected generation-only change not to reset the saved window")
	}
}
