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
	WindowAnnotationKey       = "workloads.kubeblocks.io/rolling-update-window"
	RolloutIDAnnotationKey    = "workloads.kubeblocks.io/rolling-update-id"
	RolloutBasisAnnotationKey = "workloads.kubeblocks.io/rolling-update-basis"
	windowVersion             = 2
)

// IsInternalAnnotation reports whether key is controller bookkeeping that must
// not participate in workload revision calculation.
func IsInternalAnnotation(key string) bool {
	return key == WindowAnnotationKey ||
		key == RolloutIDAnnotationKey ||
		key == RolloutBasisAnnotationKey
}

type window struct {
	Version      int      `json:"version"`
	Ended        bool     `json:"ended,omitempty"`
	Deferred     bool     `json:"deferred,omitempty"`
	RolloutID    string   `json:"rolloutID"`
	Replicas     int      `json:"replicas"`
	Participants []string `json:"participants"`
}

// RolloutID returns a stable identifier for a desired instance revision map.
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

// UpdateRolloutID advances the persisted rollout ID when the desired revision
// of any surviving instance changes. Name-set-only changes preserve the ID.
// The caller must provide the previous and next desired revision maps before
// replacing the previous map in status.
func UpdateRolloutID(owner metav1.Object, previous, next map[string]string) string {
	id := currentRolloutID(owner)
	if id == "" || desiredRevisionChanged(previous, next) {
		id = RolloutID(next)
		setRolloutID(owner, id)
	}
	return id
}

// UpdateLegacyRolloutID tracks the full desired template basis separately from
// legacy pod revisions, which intentionally omit in-place update fields.
// reassigned reports whether a surviving flat ordinal moved between templates.
// Existing objects are also marked for deferred admission when first migrated
// to persisted rollout IDs.
func UpdateLegacyRolloutID(owner metav1.Object, previous, next map[string]string,
	basis string, reassigned bool) string {
	migrating := currentRolloutID(owner) == "" && len(previous) > 0
	id := currentRolloutID(owner)
	previousBasis := currentRolloutBasis(owner)
	if id == "" || (previousBasis != "" && previousBasis != basis) || reassigned {
		nextID := RolloutID(next)
		if id == "" || nextID != id {
			id = nextID
			setRolloutID(owner, id)
		}
	}
	if previousBasis != basis {
		setRolloutBasis(owner, basis)
	}
	if migrating {
		deferLegacyRollout(owner, id)
	}
	return id
}

// CurrentRolloutID returns the persisted rollout ID, initializing a baseline
// when upgrading an object that does not have one yet.
func CurrentRolloutID(owner metav1.Object, revisions map[string]string) string {
	if id := currentRolloutID(owner); id != "" {
		return id
	}
	id := RolloutID(revisions)
	setRolloutID(owner, id)
	return id
}

// CurrentLegacyRolloutID initializes the full rollout state and deferred-
// admission marker when update reconciliation encounters a legacy object
// before revision reconciliation.
func CurrentLegacyRolloutID(owner metav1.Object, revisions, desired map[string]string, basis string) string {
	migrating := currentRolloutID(owner) == "" && len(revisions) > 0
	id := currentRolloutID(owner)
	if id == "" {
		id = RolloutID(desired)
		setRolloutID(owner, id)
		setRolloutBasis(owner, basis)
	}
	if migrating {
		deferLegacyRollout(owner, id)
	}
	return id
}

func desiredRevisionChanged(previous, next map[string]string) bool {
	for name, oldRevision := range previous {
		if newRevision, ok := next[name]; ok && newRevision != oldRevision {
			return true
		}
	}
	return false
}

func currentRolloutID(owner metav1.Object) string {
	return owner.GetAnnotations()[RolloutIDAnnotationKey]
}

func setRolloutID(owner metav1.Object, id string) {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[RolloutIDAnnotationKey] = id
	owner.SetAnnotations(annotations)
}

func currentRolloutBasis(owner metav1.Object) string {
	return owner.GetAnnotations()[RolloutBasisAnnotationKey]
}

func setRolloutBasis(owner metav1.Object, basis string) {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[RolloutBasisAnnotationKey] = basis
	owner.SetAnnotations(annotations)
}

// Participants returns the stable set of instance names admitted to a rolling
// update. orderedNames must already be sorted according to the update policy.
// The second return value reports whether the persisted window was changed.
// Callers must commit that change before admitting any child update.
func Participants(owner metav1.Object, rolloutID string, replicas int, orderedNames []string) (sets.Set[string], bool) {
	return participants(owner, rolloutID, replicas, orderedNames)
}

func participants(owner metav1.Object, rolloutID string, replicas int,
	orderedNames []string) (sets.Set[string], bool) {
	if replicas < 0 {
		replicas = 0
	}
	if replicas >= len(orderedNames) {
		return sets.New(orderedNames...), removeWindow(owner)
	}

	saved, ok := loadWindow(owner)
	if !ok || saved.Ended || saved.RolloutID != rolloutID {
		participants := append([]string(nil), orderedNames[:replicas]...)
		saveWindow(owner, window{
			Version:      windowVersion,
			RolloutID:    rolloutID,
			Replicas:     replicas,
			Participants: participants,
		})
		return sets.New(participants...), true
	}

	if saved.Deferred {
		state := saved
		state.Replicas = replicas
		changed := !reflect.DeepEqual(saved, state)
		if changed {
			saveWindow(owner, state)
		}
		return sets.New(orderedNames[:replicas]...), changed
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

func deferLegacyRollout(owner metav1.Object, rolloutID string) {
	if hasWindow(owner) {
		return
	}
	saveWindow(owner, window{
		Version:   windowVersion,
		Deferred:  true,
		RolloutID: rolloutID,
	})
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
		if saved.Deferred || saved.RolloutID != "" || saved.Replicas != 0 || len(saved.Participants) != 0 {
			return window{}, false
		}
		return saved, true
	}
	if saved.RolloutID == "" || saved.Replicas < 0 {
		return window{}, false
	}
	if saved.Deferred && len(saved.Participants) != 0 {
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
// an intentional strategy transition from a legacy object whose current
// rollout needs the deferred-admission baseline.
func Reset(owner metav1.Object) bool {
	ended := window{Version: windowVersion, Ended: true}
	if saved, ok := loadWindow(owner); ok && reflect.DeepEqual(saved, ended) {
		return false
	}
	saveWindow(owner, ended)
	return true
}
