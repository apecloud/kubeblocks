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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const WindowAnnotationKey = "workloads.kubeblocks.io/rolling-update-window"

type window struct {
	RolloutID    string   `json:"rolloutID"`
	Replicas     int      `json:"replicas"`
	Participants []string `json:"participants"`
}

// Participants returns the stable set of instance names admitted to a rolling
// update. orderedNames must already be sorted according to the update policy.
func Participants(owner metav1.Object, rolloutID string, replicas int, orderedNames []string) sets.Set[string] {
	if replicas < 0 {
		replicas = 0
	}
	if replicas >= len(orderedNames) {
		removeWindow(owner)
		return sets.New(orderedNames...)
	}

	if saved, ok := loadWindow(owner, rolloutID, replicas, orderedNames); ok {
		return sets.New(saved.Participants...)
	}

	participants := append([]string(nil), orderedNames[:replicas]...)
	saveWindow(owner, window{
		RolloutID:    rolloutID,
		Replicas:     replicas,
		Participants: participants,
	})
	return sets.New(participants...)
}

func loadWindow(owner metav1.Object, rolloutID string, replicas int, orderedNames []string) (window, bool) {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		return window{}, false
	}
	raw, ok := annotations[WindowAnnotationKey]
	if !ok {
		return window{}, false
	}

	var saved window
	if json.Unmarshal([]byte(raw), &saved) != nil || saved.RolloutID != rolloutID || saved.Replicas != replicas ||
		len(saved.Participants) != replicas {
		return window{}, false
	}

	validNames := sets.New(orderedNames...)
	participants := sets.New[string]()
	for _, name := range saved.Participants {
		if !validNames.Has(name) || participants.Has(name) {
			return window{}, false
		}
		participants.Insert(name)
	}
	return saved, true
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

func removeWindow(owner metav1.Object) {
	annotations := owner.GetAnnotations()
	if annotations == nil {
		return
	}
	if _, ok := annotations[WindowAnnotationKey]; !ok {
		return
	}
	delete(annotations, WindowAnnotationKey)
	owner.SetAnnotations(annotations)
}
