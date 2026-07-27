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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	WindowAnnotationKey = "workloads.kubeblocks.io/rolling-update-window"
	windowVersion       = 1
)

type window struct {
	Version      int      `json:"version"`
	Ended        bool     `json:"ended,omitempty"`
	RolloutID    string   `json:"rolloutID"`
	Replicas     int      `json:"replicas"`
	Participants []string `json:"participants"`
}

// RolloutID returns a stable identifier for desired revisions keyed by their
// stable workload template names.
func RolloutID(revisions map[string]string) string {
	names := make([]string, 0, len(revisions))
	for name := range revisions {
		names = append(names, name)
	}
	sort.Strings(names)

	hash := sha256.New()
	for _, name := range names {
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", name, revisions[name])
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// Participants returns the stable set of instance names admitted to a rolling
// update. orderedNames must already be sorted according to the update policy.
// The second return value reports whether the persisted window was changed.
// Callers must commit that change before admitting any child update.
func Participants(owner metav1.Object, revisions map[string]string, replicas int, orderedNames []string) (sets.Set[string], bool) {
	return participants(owner, RolloutID(revisions), replicas, orderedNames, nil)
}

// ParticipantsWithInitial behaves like Participants, but initializes a missing
// window with identities known to have already reached the desired revision.
// It is used when upgrading an already-running legacy rollout.
func ParticipantsWithInitial(owner metav1.Object, revisions map[string]string, replicas int,
	orderedNames []string, initial sets.Set[string]) (sets.Set[string], bool) {
	return participants(owner, RolloutID(revisions), replicas, orderedNames, initial)
}

func participants(owner metav1.Object, rolloutID string, replicas int,
	orderedNames []string, initial sets.Set[string]) (sets.Set[string], bool) {
	if replicas < 0 {
		replicas = 0
	}
	if replicas >= len(orderedNames) {
		return sets.New(orderedNames...), removeWindow(owner)
	}

	saved, ok := loadWindow(owner)
	if !ok || saved.Ended || saved.RolloutID != rolloutID {
		participants := initialParticipants(replicas, orderedNames, nil)
		if !hasWindow(owner) {
			participants = initialParticipants(replicas, orderedNames, initial)
		}
		saveWindow(owner, window{
			Version:      windowVersion,
			RolloutID:    rolloutID,
			Replicas:     replicas,
			Participants: participants,
		})
		active := participants
		if len(active) > replicas {
			active = active[:replicas]
		}
		return sets.New(active...), true
	}

	validNames := sets.New(orderedNames...)
	participants := make([]string, 0, len(saved.Participants))
	admitted := sets.New[string]()
	for _, name := range saved.Participants {
		if validNames.Has(name) && !admitted.Has(name) {
			participants = append(participants, name)
			admitted.Insert(name)
		}
	}

	// Keep admission monotonic for a desired rollout. A quota increase may fill
	// the delta, while a decrease only narrows the active prefix of identities
	// that were already admitted. An unchanged quota may fill holes left by
	// scale-in, preserving the valid participant intersection first.
	if replicas >= saved.Replicas {
		for _, name := range orderedNames {
			if len(participants) >= replicas {
				break
			}
			if admitted.Has(name) {
				continue
			}
			participants = append(participants, name)
			admitted.Insert(name)
		}
	}

	state := window{
		Version:      windowVersion,
		RolloutID:    rolloutID,
		Replicas:     replicas,
		Participants: participants,
	}
	changed := !reflect.DeepEqual(saved, state)
	if changed {
		saveWindow(owner, state)
	}
	active := participants
	if len(active) > replicas {
		active = active[:replicas]
	}
	return sets.New(active...), changed
}

func initialParticipants(replicas int, orderedNames []string, initial sets.Set[string]) []string {
	participants := make([]string, 0, replicas)
	admitted := sets.New[string]()
	for _, name := range orderedNames {
		if initial != nil && initial.Has(name) {
			participants = append(participants, name)
			admitted.Insert(name)
		}
	}
	for _, name := range orderedNames {
		if len(participants) >= replicas {
			break
		}
		if admitted.Has(name) {
			continue
		}
		participants = append(participants, name)
	}
	return participants
}

func loadWindow(owner metav1.Object) (window, bool) {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		return window{}, false
	}
	raw, ok := annotations[WindowAnnotationKey]
	if !ok {
		return window{}, false
	}

	var saved window
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.Version != windowVersion {
		return window{}, false
	}
	if saved.Ended {
		if saved.RolloutID != "" || saved.Replicas != 0 || len(saved.Participants) != 0 {
			return window{}, false
		}
		return saved, true
	}
	if saved.RolloutID == "" || saved.Replicas < 0 {
		return window{}, false
	}

	participants := sets.New[string]()
	for _, name := range saved.Participants {
		if participants.Has(name) {
			return window{}, false
		}
		participants.Insert(name)
	}
	return saved, true
}

func hasWindow(owner metav1.Object) bool {
	_, ok := owner.GetAnnotations()[WindowAnnotationKey]
	return ok
}

func saveWindow(owner metav1.Object, state window) {
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[WindowAnnotationKey] = string(data)
	owner.SetAnnotations(annotations)
}

func removeWindow(owner metav1.Object) bool {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		return false
	}
	if _, ok := annotations[WindowAnnotationKey]; !ok {
		return false
	}
	delete(annotations, WindowAnnotationKey)
	owner.SetAnnotations(annotations)
	return true
}

// Reset explicitly ends the rolling-update lifecycle. The marker distinguishes
// an intentional strategy transition from a legacy object that has never
// persisted a window and may need participant recovery.
func Reset(owner metav1.Object) bool {
	ended := window{Version: windowVersion, Ended: true}
	if saved, ok := loadWindow(owner); ok && reflect.DeepEqual(saved, ended) {
		return false
	}
	saveWindow(owner, ended)
	return true
}
