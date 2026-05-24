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

package k8score

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// EventReconciler must skip kbagent roleProbe events at the outer guard so
// the shared `kubeblocks.io/event-handled` annotation is not stamped. The
// downstream InstanceEventReconciler uses the same annotation as its outer
// short-circuit, so stamping it here would silently drop every kbagent
// roleProbe event the EventReconciler instance processes first.
func TestIsKBAgentRoleProbeEventMatchesKBAgentSourcedRoleProbe(t *testing.T) {
	event := &corev1.Event{
		Reason:              "roleProbe",
		ReportingController: proto.ProbeEventReportingController,
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "pod-0",
			Namespace: "ns",
			FieldPath: proto.ProbeEventFieldPath,
		},
	}
	if !isKBAgentRoleProbeEvent(event) {
		t.Fatalf("expected kbagent roleProbe event to be recognised")
	}
}

func TestIsKBAgentRoleProbeEventRejectsWrongReason(t *testing.T) {
	event := &corev1.Event{
		Reason:              "availableProbe",
		ReportingController: proto.ProbeEventReportingController,
		InvolvedObject:      corev1.ObjectReference{FieldPath: proto.ProbeEventFieldPath},
	}
	if isKBAgentRoleProbeEvent(event) {
		t.Fatalf("availableProbe must not be classified as kbagent roleProbe")
	}
}

func TestIsKBAgentRoleProbeEventRejectsWrongReportingController(t *testing.T) {
	event := &corev1.Event{
		Reason:              "roleProbe",
		ReportingController: "some-other-controller",
		InvolvedObject:      corev1.ObjectReference{FieldPath: proto.ProbeEventFieldPath},
	}
	if isKBAgentRoleProbeEvent(event) {
		t.Fatalf("non-kbagent reporting controller must not be classified as kbagent roleProbe")
	}
}

func TestIsKBAgentRoleProbeEventRejectsWrongFieldPath(t *testing.T) {
	event := &corev1.Event{
		Reason:              "roleProbe",
		ReportingController: proto.ProbeEventReportingController,
		InvolvedObject:      corev1.ObjectReference{FieldPath: "spec.containers{some-other}"},
	}
	if isKBAgentRoleProbeEvent(event) {
		t.Fatalf("non-kbagent involvedObject FieldPath must not be classified")
	}
}
