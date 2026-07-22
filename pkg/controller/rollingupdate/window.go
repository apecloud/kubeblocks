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

const WindowAnnotationKey = "workloads.kubeblocks.io/rolling-update-window"

type window struct {
	RolloutID    string            `json:"rolloutID"`
	Replicas     int               `json:"replicas"`
	Participants []string          `json:"participants"`
	Revisions    map[string]string `json:"revisions,omitempty"`
}

// RolloutID returns a stable identifier for a desired set of instance revisions.
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
	if replicas < 0 {
		replicas = 0
	}
	if replicas >= len(orderedNames) {
		return sets.New(orderedNames...), removeWindow(owner)
	}

	rolloutID := RolloutID(revisions)
	saved, ok := loadWindow(owner)
	if !ok || !sameRollout(saved, revisions, rolloutID) {
		participants := append([]string(nil), orderedNames[:replicas]...)
		saveWindow(owner, window{
			RolloutID:    rolloutID,
			Replicas:     replicas,
			Participants: participants,
			Revisions:    cloneRevisions(revisions),
		})
		return sets.New(participants...), true
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
		RolloutID:    rolloutID,
		Replicas:     replicas,
		Participants: participants,
		Revisions:    cloneRevisions(revisions),
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
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.RolloutID == "" || saved.Replicas < 0 {
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

func sameRollout(saved window, revisions map[string]string, rolloutID string) bool {
	// Windows written before the revision snapshot was introduced can only be
	// reused when their exact rollout ID still matches.
	if saved.Revisions == nil {
		return saved.RolloutID == rolloutID
	}

	common := false
	for name, revision := range saved.Revisions {
		if current, ok := revisions[name]; ok {
			common = true
			if current != revision {
				return false
			}
		}
	}
	return common || len(saved.Revisions) == 0 && len(revisions) == 0
}

func cloneRevisions(revisions map[string]string) map[string]string {
	if len(revisions) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(revisions))
	for name, revision := range revisions {
		cloned[name] = revision
	}
	return cloned
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

// Reset ends any persisted rolling-update admission window.
func Reset(owner metav1.Object) bool {
	return removeWindow(owner)
}
