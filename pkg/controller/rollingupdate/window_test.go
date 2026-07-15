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
	got := Participants(owner, "2", 1, []string{"a", "b", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected initial participant a, got %v", got)
	}

	got = Participants(owner, "2", 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("a")) {
		t.Fatalf("expected participant a after reorder, got %v", got)
	}

	got = Participants(owner, "3", 1, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b")) {
		t.Fatalf("expected new rollout to select b, got %v", got)
	}
}

func TestParticipantsResetForReplicaChangeAndInvalidState(t *testing.T) {
	owner := &metav1.PartialObjectMetadata{}
	Participants(owner, "2", 1, []string{"a", "b", "c"})

	got := Participants(owner, "2", 2, []string{"b", "a", "c"})
	if !got.Equal(sets.New("b", "a")) {
		t.Fatalf("expected replica change to rebuild window, got %v", got)
	}

	owner.Annotations[WindowAnnotationKey] = "invalid"
	got = Participants(owner, "2", 1, []string{"c", "b", "a"})
	if !got.Equal(sets.New("c")) {
		t.Fatalf("expected invalid state to be rebuilt, got %v", got)
	}

	got = Participants(owner, "2", 3, []string{"c", "b", "a"})
	if !got.Equal(sets.New("a", "b", "c")) {
		t.Fatalf("expected all participants, got %v", got)
	}
	if _, ok := owner.Annotations[WindowAnnotationKey]; ok {
		t.Fatal("expected a full window not to retain the annotation")
	}
}
