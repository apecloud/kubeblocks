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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// These tests cover the deterministic formatting helpers used by the
// alignment reconciler V(1) instrumentation. They intentionally exercise
// the formatters directly rather than capturing controller log output, so
// the assertions are stable and do not depend on logger plumbing.

func TestFormatPodSnapshot_NilPod(t *testing.T) {
	got := formatPodSnapshot("foo-0", nil, 0)
	want := "name=foo-0 oldPodFound=false"
	if got != want {
		t.Fatalf("nil pod: got %q want %q", got, want)
	}
}

func TestFormatPodSnapshot_TerminatingPod(t *testing.T) {
	truth := true
	dts := metav1.NewTime(time.Date(2026, 5, 14, 22, 33, 36, 0, time.UTC))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "foo-0",
			UID:               "11111111-2222-3333-4444-555555555555",
			DeletionTimestamp: &dts,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "InstanceSet", Name: "foo", UID: "aaa", Controller: &truth},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Date(2026, 5, 14, 22, 30, 0, 0, time.UTC)),
				},
			},
		},
	}
	got := formatPodSnapshot("foo-0", pod, 0)
	// A pod with a DeletionTimestamp is "terminating": IsPodReady and
	// IsPodAvailable both return false even though the Ready condition is
	// True. This distinction is critical for diagnosing recovery delays
	// where the API object survives past the user-perceived deletion.
	want := "name=foo-0 uid=11111111-2222-3333-4444-555555555555 phase=Running deletionTimestamp=2026-05-14T22:33:36Z ownerRef=InstanceSet/foo/aaa/controller=true ready=false available=false"
	if got != want {
		t.Fatalf("terminating pod: got %q want %q", got, want)
	}
}

func TestFormatPodSnapshot_HealthyPod(t *testing.T) {
	truth := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo-0",
			UID:  "uid-healthy",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "InstanceSet", Name: "foo", UID: "aaa", Controller: &truth},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour)),
				},
			},
		},
	}
	got := formatPodSnapshot("foo-0", pod, 0)
	want := "name=foo-0 uid=uid-healthy phase=Running deletionTimestamp= ownerRef=InstanceSet/foo/aaa/controller=true ready=true available=true"
	if got != want {
		t.Fatalf("healthy pod: got %q want %q", got, want)
	}
}

func TestFormatPodOwnerRef_None(t *testing.T) {
	got := formatPodOwnerRef(nil)
	if got != "<none>" {
		t.Fatalf("nil refs: got %q want <none>", got)
	}
	got = formatPodOwnerRef([]metav1.OwnerReference{})
	if got != "<none>" {
		t.Fatalf("empty refs: got %q want <none>", got)
	}
}

func TestFormatPodOwnerRef_ControllerPreferred(t *testing.T) {
	f := false
	truth := true
	refs := []metav1.OwnerReference{
		{Kind: "Other", Name: "x", UID: "y", Controller: &f},
		{Kind: "InstanceSet", Name: "foo", UID: "aaa", Controller: &truth},
	}
	got := formatPodOwnerRef(refs)
	want := "InstanceSet/foo/aaa/controller=true"
	if got != want {
		t.Fatalf("controller-preferred: got %q want %q", got, want)
	}
}

func TestFormatPodOwnerRef_NoControllerFallback(t *testing.T) {
	f := false
	refs := []metav1.OwnerReference{
		{Kind: "Other", Name: "x", UID: "y", Controller: &f},
	}
	got := formatPodOwnerRef(refs)
	want := "Other/x/y/controller=false"
	if got != want {
		t.Fatalf("no-controller fallback: got %q want %q", got, want)
	}
}

func TestFormatPodOwnerRef_NilControllerFlag(t *testing.T) {
	refs := []metav1.OwnerReference{
		{Kind: "Other", Name: "x", UID: "y"},
	}
	got := formatPodOwnerRef(refs)
	// nil Controller pointer is treated as not-controller; we still report
	// the first owner with controller=false.
	want := "Other/x/y/controller=false"
	if got != want {
		t.Fatalf("nil controller flag: got %q want %q", got, want)
	}
}

func TestFormatOldInstanceMapSnapshot_SortedByName(t *testing.T) {
	pods := map[string]*corev1.Pod{
		"foo-2": {ObjectMeta: metav1.ObjectMeta{Name: "foo-2", UID: "uid-2"}},
		"foo-0": {ObjectMeta: metav1.ObjectMeta{Name: "foo-0", UID: "uid-0"}},
		"foo-1": {ObjectMeta: metav1.ObjectMeta{Name: "foo-1", UID: "uid-1"}},
	}
	got := formatOldInstanceMapSnapshot(pods, 0)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	wantOrder := []string{"foo-0", "foo-1", "foo-2"}
	for i, name := range wantOrder {
		prefix := "name=" + name + " "
		if !strings.HasPrefix(got[i], prefix) {
			t.Fatalf("entry %d: expected prefix %q, got %q", i, prefix, got[i])
		}
	}
}

func TestFormatOldInstanceMapSnapshot_EmptyMap(t *testing.T) {
	got := formatOldInstanceMapSnapshot(map[string]*corev1.Pod{}, 0)
	if len(got) != 0 {
		t.Fatalf("empty map: expected empty slice, got %v", got)
	}
}

func TestFormatOldInstanceMapSnapshot_NilPodEntry(t *testing.T) {
	// A nil pod value in the map (theoretically unreachable, but the
	// formatter is defensive) renders as the nil-safe form so a "we expected
	// an old pod here but got nil" case is distinguishable.
	pods := map[string]*corev1.Pod{
		"foo-0": nil,
	}
	got := formatOldInstanceMapSnapshot(pods, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %v", got)
	}
	want := "name=foo-0 oldPodFound=false"
	if got[0] != want {
		t.Fatalf("nil entry: got %q want %q", got[0], want)
	}
}

func TestFormatNameSetSnapshot(t *testing.T) {
	oldSet := sets.New[string]("foo-0", "foo-1", "foo-2")
	newSet := sets.New[string]("foo-0", "foo-1", "foo-3")
	createSet := newSet.Difference(oldSet)
	deleteSet := oldSet.Difference(newSet)
	got := formatNameSetSnapshot(oldSet, newSet, createSet, deleteSet)

	cases := map[string][]string{
		"oldNameSet":    {"foo-0", "foo-1", "foo-2"},
		"newNameSet":    {"foo-0", "foo-1", "foo-3"},
		"createNameSet": {"foo-3"},
		"deleteNameSet": {"foo-2"},
	}
	for key, want := range cases {
		if !equalStringSlices(got[key], want) {
			t.Fatalf("key %s: got %v want %v", key, got[key], want)
		}
	}
}

func TestFormatNameSetSnapshot_EmptySets(t *testing.T) {
	empty := sets.New[string]()
	got := formatNameSetSnapshot(empty, empty, empty, empty)
	for _, key := range []string{"oldNameSet", "newNameSet", "createNameSet", "deleteNameSet"} {
		v, ok := got[key]
		if !ok {
			t.Fatalf("missing key %s", key)
		}
		if v == nil {
			t.Fatalf("key %s: expected non-nil empty slice, got nil", key)
		}
		if len(v) != 0 {
			t.Fatalf("key %s: expected empty slice, got %v", key, v)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
