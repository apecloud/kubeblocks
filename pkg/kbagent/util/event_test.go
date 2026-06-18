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

package util

import (
	"testing"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestNewEventAndName(t *testing.T) {
	t.Setenv(kbEnvNamespace, "ns")
	t.Setenv(kbEnvPodName, "pod")
	t.Setenv(kbEnvPodUID, "uid")
	t.Setenv(kbEnvNodeName, "node")

	eventName := generateEventName("Ready", "ok")
	if eventName == "" || eventName == generateEventName("Ready", "changed") {
		t.Fatalf("unexpected generated event names")
	}

	event := newEvent("Ready", "ok")
	if event.Name != eventName || event.Namespace != "ns" {
		t.Fatalf("unexpected event identity: %s/%s", event.Namespace, event.Name)
	}
	if event.InvolvedObject.Name != "pod" || string(event.InvolvedObject.UID) != "uid" {
		t.Fatalf("unexpected involved object: %#v", event.InvolvedObject)
	}
	if event.Source.Component != proto.ProbeEventSourceComponent || event.Source.Host != "node" {
		t.Fatalf("unexpected event source: %#v", event.Source)
	}
	if event.ReportingController != proto.ProbeEventReportingController || event.ReportingInstance != "pod" {
		t.Fatalf("unexpected reporting fields: %#v", event)
	}
	if event.Action != "Ready" || event.Type != "Normal" || event.Count != 1 {
		t.Fatalf("unexpected event fields: %#v", event)
	}
}

func TestSendEventWithMessageSyncConfigError(t *testing.T) {
	t.Setenv("KUBECONFIG", "/path/to/missing/kubeconfig")

	logger := logr.Discard()
	if err := SendEventWithMessage(&logger, "Ready", "ok", true); err == nil {
		t.Fatalf("expected kube config error")
	}
	if err := createOrUpdateEvent("Ready", "ok", 0, 0, false); err == nil {
		t.Fatalf("expected create event kube config error")
	}
	if clientSet, err := getK8sClientSet(); clientSet != nil || err == nil {
		t.Fatalf("expected nil clientset and kube config error, got %#v, %v", clientSet, err)
	}
}
