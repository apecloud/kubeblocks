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

// Package coordination defines an opt-in execution evidence core. It is not yet
// wired into Backup reconciliation, admission, RBAC, or workload construction.
// Inputs must come from trusted controller reads. These pure functions neither
// authenticate observations nor perform an API resourceVersion compare-and-swap.
package coordination

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Phase string

const (
	Open             Phase = "Open"
	Sealed           Phase = "Sealed"
	TerminalObserved Phase = "TerminalObserved"
	Released         Phase = "Released"
)

type Outcome string

const (
	Succeeded Outcome = "Succeeded"
	Failed    Outcome = "Failed"
	Cancelled Outcome = "Cancelled"
)

// Identity is fixed before a Pod is registered. Epoch is a protocol generation,
// never a timeout. Names cannot substitute for any of these full object UIDs.
type Identity struct {
	BackupUID  types.UID
	LeaseUID   types.UID
	Epoch      int64
	PlanDigest string
	JobUID     types.UID
}

// PodApproval freezes one actual Pod's identity and spec, excluding only the
// scheduler-assigned nodeName. It does NOT attest script, image, or mount safety.
type PodApproval struct {
	Namespace, Name              string
	UID                          types.UID
	JobName                      string
	JobUID                       types.UID
	SpecDigest, Container, Image string
}

// ExitProof describes current termination of the sole approved business
// container. Nonzero ExitCode still proves termination, not backup success.
type ExitProof struct {
	Pod                   PodApproval
	PodResourceVersion    string
	ContainerID, ImageID  string
	StartedAt, FinishedAt metav1.Time
	ExitCode              int32
}

// TerminalReceipt must be constructed by an adapter after a direct API read of
// the same Backup UID, matching execution digest, outcome, and persisted proof.
// This type cannot prove the API read or artifact validity by itself.
type TerminalReceipt struct {
	BackupUID                   types.UID
	PlanDigest, ResourceVersion string
	Outcome                     Outcome
}

// ReleaseReceipt records an authenticated readback of the same Lease after CAS
// cleared this holder. Observing a new holder is not this receipt.
type ReleaseReceipt struct {
	LeaseUID                            types.UID
	Epoch                               int64
	PreviousHolderUID, CurrentHolderUID types.UID
	ResourceVersion                     string
}
type Execution struct {
	Identity Identity
	Phase    Phase
	Pod      *PodApproval
	Exit     *ExitProof
	Terminal *TerminalReceipt
	Release  *ReleaseReceipt
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var pinnedImagePattern = regexp.MustCompile(`^.+@sha256:[0-9a-f]{64}$`)

func fail(message string) error { return fmt.Errorf("execution evidence: %s", message) }
func rank(p Phase) int {
	switch p {
	case Open:
		return 0
	case Sealed:
		return 1
	case TerminalObserved:
		return 2
	case Released:
		return 3
	}
	return -1
}
func validApproval(a PodApproval) bool {
	return a.Namespace != "" && a.Name != "" && a.UID != "" && a.JobName != "" && a.JobUID != "" && digestPattern.MatchString(a.SpecDigest) && a.Container != "" && pinnedImagePattern.MatchString(a.Image)
}
func validExit(p ExitProof) bool {
	return validApproval(p.Pod) && p.PodResourceVersion != "" && p.ContainerID != "" && p.ImageID != "" && !p.StartedAt.IsZero() && !p.FinishedAt.IsZero() && !p.FinishedAt.Before(&p.StartedAt)
}

// ApprovePod captures identity/shape from an actual API Pod. The caller must
// separately verify the known gate profile, frozen scripts, credentials, deletion
// protection, Backup ownership, and source-domain mapping before persisting it.
// The first slice admits no installer, sidecar, probe, hook or restart profile.
func ApprovePod(p *corev1.Pod, jobUID types.UID) (PodApproval, error) {
	empty := PodApproval{}
	if p == nil || p.UID == "" || p.Name == "" || p.Namespace == "" || jobUID == "" {
		return empty, fail("missing pod identity")
	}
	var jobName string
	owners := 0
	for _, o := range p.OwnerReferences {
		if o.Controller != nil && *o.Controller {
			owners++
			if o.APIVersion != "batch/v1" || o.Kind != "Job" || o.UID != jobUID || o.Name == "" {
				return empty, fail("job owner mismatch")
			}
			jobName = o.Name
		}
	}
	if owners != 1 {
		return empty, fail("expected one job controller owner")
	}
	s := p.Spec.DeepCopy()
	if s.RestartPolicy != corev1.RestartPolicyNever || len(s.Containers) != 1 || len(s.InitContainers) != 0 || len(s.EphemeralContainers) != 0 {
		return empty, fail("unsupported container shape")
	}
	c := s.Containers[0]
	if c.Name == "" || !pinnedImagePattern.MatchString(c.Image) || c.Lifecycle != nil || c.StartupProbe != nil || c.LivenessProbe != nil || c.ReadinessProbe != nil {
		return empty, fail("unfrozen container entry shape")
	}
	s.NodeName = ""
	encoded, err := json.Marshal(s)
	if err != nil {
		return empty, fmt.Errorf("execution evidence: encode pod spec: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return PodApproval{Namespace: p.Namespace, Name: p.Name, UID: p.UID, JobName: jobName, JobUID: jobUID, SpecDigest: hex.EncodeToString(sum[:]), Container: c.Name, Image: c.Image}, nil
}

// CollectTermination rejects an empty status list, lastTerminationState, lost
// objects and partial evidence. It does not turn NotFound into an exited Pod.
// An API adapter must persist and read back the returned proof before release.
func CollectTermination(expected PodApproval, p *corev1.Pod) (ExitProof, error) {
	empty := ExitProof{}
	if !validApproval(expected) {
		return empty, fail("invalid approved pod")
	}
	actual, err := ApprovePod(p, expected.JobUID)
	if err != nil {
		return empty, err
	}
	if actual != expected {
		return empty, fail("pod identity or approved spec changed")
	}
	if p.ResourceVersion == "" || (p.Status.Phase != corev1.PodSucceeded && p.Status.Phase != corev1.PodFailed) || p.Status.Reason == "NodeLost" {
		return empty, fail("pod termination unknown")
	}
	if len(p.Status.ContainerStatuses) != 1 || len(p.Status.InitContainerStatuses) != 0 || len(p.Status.EphemeralContainerStatuses) != 0 {
		return empty, fail("incomplete or extra container statuses")
	}
	s := p.Status.ContainerStatuses[0]
	t := s.State.Terminated
	if s.Name != expected.Container || t == nil || s.State.Running != nil || s.State.Waiting != nil || s.RestartCount != 0 || s.ContainerID == "" || t.ContainerID != s.ContainerID {
		return empty, fail("current container instance not terminated")
	}
	proof := ExitProof{Pod: expected, PodResourceVersion: p.ResourceVersion, ContainerID: s.ContainerID, ImageID: s.ImageID, StartedAt: t.StartedAt, FinishedAt: t.FinishedAt, ExitCode: t.ExitCode}
	if !validExit(proof) {
		return empty, fail("incomplete termination proof")
	}
	return proof, nil
}

// Validate checks a record's internal consistency, not its provenance. Every
// status writer must also use ValidateTransition and a real API UID/RV CAS.
func Validate(e Execution) error {
	id := e.Identity
	if id.BackupUID == "" || id.LeaseUID == "" || id.JobUID == "" || id.Epoch <= 0 || !digestPattern.MatchString(id.PlanDigest) {
		return fail("invalid execution identity")
	}
	stage := rank(e.Phase)
	if stage < 0 {
		return fail("unknown phase")
	}
	if e.Pod != nil && (!validApproval(*e.Pod) || e.Pod.JobUID != id.JobUID) {
		return fail("invalid participant binding")
	}
	if e.Exit != nil && (e.Pod == nil || !validExit(*e.Exit) || e.Exit.Pod != *e.Pod) {
		return fail("invalid exit binding")
	}
	if stage < 2 && e.Terminal != nil {
		return fail("terminal receipt before terminal observation")
	}
	if stage >= 2 {
		r := e.Terminal
		if r == nil || r.BackupUID != id.BackupUID || r.PlanDigest != id.PlanDigest || r.ResourceVersion == "" {
			return fail("terminal receipt mismatch")
		}
		if r.Outcome != Succeeded && r.Outcome != Failed && r.Outcome != Cancelled {
			return fail("invalid terminal outcome")
		}
		if e.Pod != nil && e.Exit == nil {
			return fail("registered pod exit unknown")
		}
		if r.Outcome == Succeeded && (e.Pod == nil || e.Exit == nil || e.Exit.ExitCode != 0) {
			return fail("success lacks successful process evidence")
		}
	}
	if stage < 3 && e.Release != nil {
		return fail("release receipt before release observation")
	}
	if stage == 3 {
		r := e.Release
		if r == nil || r.LeaseUID != id.LeaseUID || r.Epoch != id.Epoch || r.PreviousHolderUID != id.BackupUID || r.CurrentHolderUID != "" || r.ResourceVersion == "" {
			return fail("release receipt mismatch")
		}
	}
	return nil
}

// ValidateTransition must run on fresh before/after values for ALL coordination
// status writes, including cleanup and error updates. Caller persists with
// resourceVersion CAS; a conflict requires rereading, not force-applying after.
func ValidateTransition(before, after Execution) error {
	if err := Validate(before); err != nil {
		return err
	}
	if err := Validate(after); err != nil {
		return err
	}
	if before.Identity != after.Identity {
		return fail("execution identity is immutable")
	}
	a, b := rank(before.Phase), rank(after.Phase)
	if b < a || b > a+1 {
		return fail("phase reversal or skipped phase")
	}
	if before.Pod != nil && (after.Pod == nil || *after.Pod != *before.Pod) {
		return fail("registered participant cannot be lost or replaced")
	}
	if before.Pod == nil && after.Pod != nil && (before.Phase != Open || after.Phase != Open) {
		return fail("registration requires open phase")
	}
	if before.Exit != nil && !reflect.DeepEqual(before.Exit, after.Exit) {
		return fail("exit proof cannot be lost or replaced")
	}
	if before.Terminal != nil && !reflect.DeepEqual(before.Terminal, after.Terminal) {
		return fail("terminal receipt cannot be lost or replaced")
	}
	if before.Release != nil && !reflect.DeepEqual(before.Release, after.Release) {
		return fail("release receipt cannot be lost or replaced")
	}
	return nil
}

// PermitsStart is only a predicate on authenticated input. It does not fetch a
// Lease, authenticate a gate, or enable an action. A previously issued permit is
// protected by the immutable participant and complete termination requirement.
func PermitsStart(e Execution, expected Identity, podUID types.UID) (bool, error) {
	if err := Validate(e); err != nil {
		return false, err
	}
	return e.Phase == Open && e.Identity == expected && e.Pod != nil && podUID != "" && e.Pod.UID == podUID && e.Exit == nil, nil
}

// ReadyToRelease allows the future adapter to attempt CAS, never unconditional
// deletion or takeover. It is false until terminal evidence has been read back.
func ReadyToRelease(e Execution) bool { return Validate(e) == nil && e.Phase == TerminalObserved }
