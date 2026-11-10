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

package coordination

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func fixturePod() *corev1.Pod {
	controller := true
	started := metav1.NewTime(time.Unix(100, 0))
	finished := metav1.NewTime(time.Unix(120, 0))
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "backup", UID: "pod-1", ResourceVersion: "9", OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: "attempt", UID: "job-1", Controller: &controller}}}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "business", Image: "example/backup@sha256:" + strings.Repeat("a", 64)}}}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{{Name: "business", ContainerID: "containerd://instance-1", ImageID: "containerd://image-1", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ContainerID: "containerd://instance-1", StartedAt: started, FinishedAt: finished}}}}}}
}
func fixtureExecution(t *testing.T) Execution {
	t.Helper()
	p := fixturePod()
	a, err := ApprovePod(p, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	return Execution{Identity: Identity{BackupUID: "backup-1", LeaseUID: "lease-1", Epoch: 1, PlanDigest: strings.Repeat("b", 64), JobUID: "job-1"}, Phase: Open, Pod: &a}
}
func copyExecution(e Execution) Execution {
	if e.Pod != nil {
		x := *e.Pod
		e.Pod = &x
	}
	if e.Exit != nil {
		x := *e.Exit
		e.Exit = &x
	}
	if e.Terminal != nil {
		x := *e.Terminal
		e.Terminal = &x
	}
	if e.Release != nil {
		x := *e.Release
		e.Release = &x
	}
	return e
}
func completed(t *testing.T) Execution {
	t.Helper()
	e := fixtureExecution(t)
	e.Phase = Sealed
	proof, err := CollectTermination(*e.Pod, fixturePod())
	if err != nil {
		t.Fatal(err)
	}
	e.Exit = &proof
	return e
}
func terminal(t *testing.T) Execution {
	t.Helper()
	e := completed(t)
	e.Phase = TerminalObserved
	e.Terminal = &TerminalReceipt{BackupUID: e.Identity.BackupUID, PlanDigest: e.Identity.PlanDigest, ResourceVersion: "104", Outcome: Succeeded}
	return e
}
func TestFullLifecycle(t *testing.T) {
	registered := fixtureExecution(t)
	initial := copyExecution(registered)
	initial.Pod = nil
	sealed := copyExecution(registered)
	sealed.Phase = Sealed
	exited := completed(t)
	observed := terminal(t)
	released := copyExecution(observed)
	released.Phase = Released
	released.Release = &ReleaseReceipt{LeaseUID: observed.Identity.LeaseUID, Epoch: 1, PreviousHolderUID: observed.Identity.BackupUID, ResourceVersion: "105"}
	stages := []Execution{initial, registered, sealed, exited, observed, released}
	for i, e := range stages {
		if err := Validate(e); err != nil {
			t.Fatalf("stage %d: %v", i, err)
		}
		if err := ValidateTransition(e, e); err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			if err := ValidateTransition(stages[i-1], e); err != nil {
				t.Fatalf("transition %d: %v", i, err)
			}
		}
	}
	if ok, err := PermitsStart(registered, registered.Identity, registered.Pod.UID); err != nil || !ok {
		t.Fatalf("open gate: %v %v", ok, err)
	}
	if ok, err := PermitsStart(sealed, sealed.Identity, sealed.Pod.UID); err != nil || ok {
		t.Fatalf("sealed gate: %v %v", ok, err)
	}
	if !ReadyToRelease(observed) || ReadyToRelease(exited) || ReadyToRelease(released) {
		t.Fatal("release boundary")
	}
}
func TestTerminationRejectsIncompleteOrChangedEvidence(t *testing.T) {
	mutations := map[string]func(*corev1.Pod){
		"empty-statuses":   func(p *corev1.Pod) { p.Status.ContainerStatuses = nil },
		"missing-expected": func(p *corev1.Pod) { p.Status.ContainerStatuses[0].Name = "other" },
		"duplicate-status": func(p *corev1.Pod) {
			p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, p.Status.ContainerStatuses[0])
		},
		"extra-status": func(p *corev1.Pod) {
			p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{Name: "injected"})
		},
		"only-last-termination": func(p *corev1.Pod) {
			s := &p.Status.ContainerStatuses[0]
			s.LastTerminationState = s.State
			s.State = corev1.ContainerState{}
		},
		"running": func(p *corev1.Pod) {
			p.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
		},
		"conflicting-state": func(p *corev1.Pod) { p.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{} },
		"restart":           func(p *corev1.Pod) { p.Status.ContainerStatuses[0].RestartCount = 1 },
		"missing-instance":  func(p *corev1.Pod) { p.Status.ContainerStatuses[0].ContainerID = "" },
		"instance-mismatch": func(p *corev1.Pod) { p.Status.ContainerStatuses[0].State.Terminated.ContainerID = "containerd://other" },
		"empty-image-id":    func(p *corev1.Pod) { p.Status.ContainerStatuses[0].ImageID = "" },
		"missing-start":     func(p *corev1.Pod) { p.Status.ContainerStatuses[0].State.Terminated.StartedAt = metav1.Time{} },
		"missing-finish":    func(p *corev1.Pod) { p.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.Time{} },
		"backward-time": func(p *corev1.Pod) {
			p.Status.ContainerStatuses[0].State.Terminated.FinishedAt = metav1.NewTime(time.Unix(99, 0))
		},
		"pending":                func(p *corev1.Pod) { p.Status.Phase = corev1.PodPending },
		"unknown":                func(p *corev1.Pod) { p.Status.Phase = corev1.PodUnknown },
		"node-lost":              func(p *corev1.Pod) { p.Status.Reason = "NodeLost" },
		"uid-changed":            func(p *corev1.Pod) { p.UID = "pod-2" },
		"namespace-changed":      func(p *corev1.Pod) { p.Namespace = "other" },
		"name-changed":           func(p *corev1.Pod) { p.Name = "other" },
		"owner-uid":              func(p *corev1.Pod) { p.OwnerReferences[0].UID = "job-2" },
		"owner-name":             func(p *corev1.Pod) { p.OwnerReferences[0].Name = "other" },
		"owner-kind":             func(p *corev1.Pod) { p.OwnerReferences[0].Kind = "Backup" },
		"owner-version":          func(p *corev1.Pod) { p.OwnerReferences[0].APIVersion = "batch/v2" },
		"owner-missing":          func(p *corev1.Pod) { p.OwnerReferences = nil },
		"extra-controller":       func(p *corev1.Pod) { p.OwnerReferences = append(p.OwnerReferences, p.OwnerReferences[0]) },
		"image-changed":          func(p *corev1.Pod) { p.Spec.Containers[0].Image = "other@sha256:" + strings.Repeat("c", 64) },
		"command-changed":        func(p *corev1.Pod) { p.Spec.Containers[0].Command = []string{"unsafe"} },
		"volume-changed":         func(p *corev1.Pod) { p.Spec.Volumes = []corev1.Volume{{Name: "changed"}} },
		"init-added":             func(p *corev1.Pod) { p.Spec.InitContainers = []corev1.Container{{Name: "init"}} },
		"init-status-added":      func(p *corev1.Pod) { p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "init"}} },
		"ephemeral-added":        func(p *corev1.Pod) { p.Spec.EphemeralContainers = []corev1.EphemeralContainer{{}} },
		"ephemeral-status-added": func(p *corev1.Pod) { p.Status.EphemeralContainerStatuses = []corev1.ContainerStatus{{Name: "debug"}} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			p := fixturePod()
			a, err := ApprovePod(p, "job-1")
			if err != nil {
				t.Fatal(err)
			}
			mutate(p)
			if proof, err := CollectTermination(a, p); err == nil || proof.Pod.UID != "" {
				t.Fatalf("accepted incomplete evidence: %+v %v", proof, err)
			}
		})
	}
	t.Run("nil-pod", func(t *testing.T) {
		a, _ := ApprovePod(fixturePod(), "job-1")
		if _, err := CollectTermination(a, nil); err == nil {
			t.Fatal("nil treated as exited")
		}
	})
}
func TestScheduledNodeAndFailedProcess(t *testing.T) {
	p := fixturePod()
	a, err := ApprovePod(p, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	p.Spec.NodeName = "scheduled-node"
	p.Status.Phase = corev1.PodFailed
	p.Status.ContainerStatuses[0].State.Terminated.ExitCode = 137
	proof, err := CollectTermination(a, p)
	if err != nil || proof.ExitCode != 137 {
		t.Fatalf("termination is not business success: %+v %v", proof, err)
	}
	e := fixtureExecution(t)
	e.Phase = TerminalObserved
	e.Exit = &proof
	e.Terminal = &TerminalReceipt{BackupUID: e.Identity.BackupUID, PlanDigest: e.Identity.PlanDigest, ResourceVersion: "4", Outcome: Succeeded}
	if Validate(e) == nil {
		t.Fatal("nonzero exit accepted as success")
	}
	e.Terminal.Outcome = Failed
	if err := Validate(e); err != nil {
		t.Fatal(err)
	}
}
func TestApprovalRejectsUnsupportedShape(t *testing.T) {
	for name, mutate := range map[string]func(*corev1.Pod){
		"restart":         func(p *corev1.Pod) { p.Spec.RestartPolicy = corev1.RestartPolicyAlways },
		"zero-containers": func(p *corev1.Pod) { p.Spec.Containers = nil },
		"sidecar":         func(p *corev1.Pod) { p.Spec.Containers = append(p.Spec.Containers, p.Spec.Containers[0]) },
		"init":            func(p *corev1.Pod) { p.Spec.InitContainers = []corev1.Container{{Name: "installer"}} },
		"image-tag":       func(p *corev1.Pod) { p.Spec.Containers[0].Image = "backup:latest" },
		"hook":            func(p *corev1.Pod) { p.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{} },
		"probe":           func(p *corev1.Pod) { p.Spec.Containers[0].ReadinessProbe = &corev1.Probe{} },
	} {
		t.Run(name, func(t *testing.T) {
			p := fixturePod()
			mutate(p)
			if _, err := ApprovePod(p, "job-1"); err == nil {
				t.Fatal("unsupported shape accepted")
			}
		})
	}
}
func TestMonotonicTransitions(t *testing.T) {
	tests := []struct {
		name   string
		before func(*testing.T) Execution
		mutate func(*Execution)
	}{
		{"forget-pod", fixtureExecution, func(e *Execution) { e.Pod = nil }},
		{"replace-pod", fixtureExecution, func(e *Execution) { e.Pod.UID = "pod-2" }},
		{"change-spec", fixtureExecution, func(e *Execution) { e.Pod.SpecDigest = strings.Repeat("d", 64) }},
		{"change-backup", fixtureExecution, func(e *Execution) { e.Identity.BackupUID = "other" }},
		{"change-lease", fixtureExecution, func(e *Execution) { e.Identity.LeaseUID = "other" }},
		{"change-epoch", fixtureExecution, func(e *Execution) { e.Identity.Epoch++ }},
		{"change-plan", fixtureExecution, func(e *Execution) { e.Identity.PlanDigest = strings.Repeat("d", 64) }},
		{"change-job", fixtureExecution, func(e *Execution) { e.Identity.JobUID = "other" }},
		{"unseal", completed, func(e *Execution) { e.Phase = Open }},
		{"forget-exit", completed, func(e *Execution) { e.Exit = nil }},
		{"replace-exit", completed, func(e *Execution) { e.Exit.ContainerID = "other" }},
		{"forget-terminal", terminal, func(e *Execution) { e.Terminal = nil }},
		{"replace-terminal", terminal, func(e *Execution) { e.Terminal.ResourceVersion = "new" }},
		{"skip-seal", fixtureExecution, func(e *Execution) { *e = terminal(t) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.before(t)
			after := copyExecution(before)
			tc.mutate(&after)
			if err := ValidateTransition(before, after); err == nil {
				t.Fatal("unsafe mutation accepted")
			}
		})
	}
	t.Run("registration-after-seal", func(t *testing.T) {
		b := fixtureExecution(t)
		b.Pod = nil
		b.Phase = Sealed
		a := copyExecution(b)
		a.Pod = fixtureExecution(t).Pod
		if ValidateTransition(b, a) == nil {
			t.Fatal("late registration")
		}
	})
}
func TestReleaseAndReceipts(t *testing.T) {
	for name, mutate := range map[string]func(*Execution){
		"no-exit":               func(e *Execution) { e.Exit = nil },
		"no-terminal-rv":        func(e *Execution) { e.Terminal.ResourceVersion = "" },
		"wrong-terminal-uid":    func(e *Execution) { e.Terminal.BackupUID = "other" },
		"wrong-terminal-digest": func(e *Execution) { e.Terminal.PlanDigest = strings.Repeat("c", 64) },
		"wrong-exit-pod":        func(e *Execution) { e.Exit.Pod.UID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			e := terminal(t)
			mutate(&e)
			if ReadyToRelease(e) || Validate(e) == nil {
				t.Fatal("invalid release accepted")
			}
		})
	}
	for name, mutate := range map[string]func(*ReleaseReceipt){
		"lease": func(r *ReleaseReceipt) { r.LeaseUID = "other" }, "epoch": func(r *ReleaseReceipt) { r.Epoch = 2 }, "holder": func(r *ReleaseReceipt) { r.PreviousHolderUID = "other" }, "still-held": func(r *ReleaseReceipt) { r.CurrentHolderUID = "next" }, "missing-rv": func(r *ReleaseReceipt) { r.ResourceVersion = "" },
	} {
		t.Run(name, func(t *testing.T) {
			e := terminal(t)
			e.Phase = Released
			e.Release = &ReleaseReceipt{LeaseUID: e.Identity.LeaseUID, Epoch: 1, PreviousHolderUID: e.Identity.BackupUID, ResourceVersion: "8"}
			mutate(e.Release)
			if Validate(e) == nil {
				t.Fatal("wrong release receipt")
			}
		})
	}
}
func TestUnstartedCancellationAndGateIdentity(t *testing.T) {
	e := fixtureExecution(t)
	e.Pod = nil
	e.Phase = Sealed
	n := copyExecution(e)
	n.Phase = TerminalObserved
	n.Terminal = &TerminalReceipt{BackupUID: e.Identity.BackupUID, PlanDigest: e.Identity.PlanDigest, ResourceVersion: "6", Outcome: Cancelled}
	if err := ValidateTransition(e, n); err != nil {
		t.Fatal(err)
	}
	n.Terminal.Outcome = Succeeded
	if Validate(n) == nil {
		t.Fatal("unstarted success")
	}
	e = fixtureExecution(t)
	for _, uid := range []types.UID{"", "another-pod"} {
		if ok, _ := PermitsStart(e, e.Identity, uid); ok {
			t.Fatal("wrong pod gate")
		}
	}
	wrong := e.Identity
	wrong.Epoch++
	if ok, _ := PermitsStart(e, wrong, e.Pod.UID); ok {
		t.Fatal("old epoch gate")
	}
	e.Phase = "unexpected"
	if ok, err := PermitsStart(e, e.Identity, e.Pod.UID); ok || err == nil {
		t.Fatal("unknown phase")
	}
}

func TestMalformedEvidenceCannotAuthorize(t *testing.T) {
	for name, mutate := range map[string]func(*Execution){
		"empty-backup-uid":         func(e *Execution) { e.Identity.BackupUID = "" },
		"empty-lease-uid":          func(e *Execution) { e.Identity.LeaseUID = "" },
		"zero-epoch":               func(e *Execution) { e.Identity.Epoch = 0 },
		"bad-plan-digest":          func(e *Execution) { e.Identity.PlanDigest = "not-a-digest" },
		"bad-spec-digest":          func(e *Execution) { e.Pod.SpecDigest = "not-a-digest" },
		"blank-job-uid":            func(e *Execution) { e.Identity.JobUID = "" },
		"blank-pod-uid":            func(e *Execution) { e.Pod.UID = "" },
		"participant-job-mismatch": func(e *Execution) { e.Pod.JobUID = "different" },
	} {
		t.Run(name, func(t *testing.T) {
			e := fixtureExecution(t)
			mutate(&e)
			if ok, err := PermitsStart(e, e.Identity, "pod-1"); ok || err == nil {
				t.Fatal("malformed record authorized")
			}
		})
	}
	t.Run("missing-pod-resource-version", func(t *testing.T) {
		p := fixturePod()
		a, _ := ApprovePod(p, "job-1")
		p.ResourceVersion = ""
		if _, err := CollectTermination(a, p); err == nil {
			t.Fatal("missing observation version")
		}
	})
	t.Run("immutable-release", func(t *testing.T) {
		b := terminal(t)
		b.Phase = Released
		b.Release = &ReleaseReceipt{LeaseUID: b.Identity.LeaseUID, Epoch: 1, PreviousHolderUID: b.Identity.BackupUID, ResourceVersion: "8"}
		a := copyExecution(b)
		a.Release.ResourceVersion = "9"
		if ValidateTransition(b, a) == nil {
			t.Fatal("release receipt replaced")
		}
	})
}
