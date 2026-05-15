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

package instanceset

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// alignment_log.go contains deterministic formatting helpers used by the
// instance alignment reconciler to emit V(1) instrumentation logs. The
// helpers intentionally produce compact, lossy summaries (no labels,
// annotations, or full spec/status content) so that enabling V(1) verbosity
// does not produce unbounded log size and does not leak arbitrary metadata.

// formatPodSnapshot returns a single compact summary line for a Pod. It is
// nil-safe: when pod is nil the result is "name=<name> oldPodFound=false",
// allowing the caller to emit a clear "we expected an old pod here but the
// map lookup returned nil" signal.
//
// The format is intentionally key=value separated by single spaces so that
// the resulting log argument is one line per pod.
func formatPodSnapshot(name string, pod *corev1.Pod, minReadySeconds int32) string {
	if pod == nil {
		return fmt.Sprintf("name=%s oldPodFound=false", name)
	}
	dts := ""
	if pod.DeletionTimestamp != nil {
		dts = pod.DeletionTimestamp.UTC().Format("2006-01-02T15:04:05Z")
	}
	return fmt.Sprintf(
		"name=%s uid=%s phase=%s deletionTimestamp=%s ownerRef=%s ready=%t available=%t",
		name,
		string(pod.UID),
		string(pod.Status.Phase),
		dts,
		formatPodOwnerRef(pod.OwnerReferences),
		intctrlutil.IsPodReady(pod),
		intctrlutil.IsPodAvailable(pod, minReadySeconds),
	)
}

// formatPodOwnerRef returns a compact owner-reference summary. The controller
// owner (Controller=true) is preferred when present; if no owner is set the
// result is "<none>"; otherwise the first reference is reported. Only
// kind/name/uid/controller-flag are included by design - additional fields
// (apiVersion, blockOwnerDeletion) are dropped to keep the summary short.
func formatPodOwnerRef(refs []metav1.OwnerReference) string {
	if len(refs) == 0 {
		return "<none>"
	}
	for i := range refs {
		ref := &refs[i]
		if ref.Controller != nil && *ref.Controller {
			return fmt.Sprintf("%s/%s/%s/controller=true", ref.Kind, ref.Name, string(ref.UID))
		}
	}
	ref := refs[0]
	return fmt.Sprintf("%s/%s/%s/controller=false", ref.Kind, ref.Name, string(ref.UID))
}

// formatOldInstanceMapSnapshot returns a deterministic, name-sorted slice of
// compact pod summary strings for use in V(1) instrumentation logs. Each
// element is the output of formatPodSnapshot for one pod in oldInstanceMap.
// The slice ordering is by ascending name so that log diffs across reconciles
// are stable and grep-friendly.
func formatOldInstanceMapSnapshot(oldInstanceMap map[string]*corev1.Pod, minReadySeconds int32) []string {
	names := make([]string, 0, len(oldInstanceMap))
	for name := range oldInstanceMap {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, formatPodSnapshot(name, oldInstanceMap[name], minReadySeconds))
	}
	return out
}

// formatNameSetSnapshot returns a deterministic map view of the four name
// sets used by the alignment loop. Each value is a sorted slice of names; an
// empty set is rendered as an empty (non-nil) slice. Callers pass the four
// sets as separate keyed entries so that the resulting structured log keys
// are stable.
func formatNameSetSnapshot(oldNameSet, newNameSet, createNameSet, deleteNameSet sets.Set[string]) map[string][]string {
	return map[string][]string{
		"oldNameSet":    sortedNamesFromSet(oldNameSet),
		"newNameSet":    sortedNamesFromSet(newNameSet),
		"createNameSet": sortedNamesFromSet(createNameSet),
		"deleteNameSet": sortedNamesFromSet(deleteNameSet),
	}
}

func sortedNamesFromSet(s sets.Set[string]) []string {
	if s.Len() == 0 {
		return []string{}
	}
	out := s.UnsortedList()
	sort.Strings(out)
	return out
}
