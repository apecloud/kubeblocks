/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// --- generateEventName ---

func TestGenerateEventName_Basic(t *testing.T) {
	os.Setenv(kbEnvPodUID, "uid-123")
	os.Setenv(kbEnvPodName, "pod-0")
	defer os.Unsetenv(kbEnvPodUID)
	defer os.Unsetenv(kbEnvPodName)

	name := generateEventName("reason1", "message1")
	assert.NotEmpty(t, name)
	assert.Contains(t, name, "pod-0.")
}

func TestGenerateEventName_Deterministic(t *testing.T) {
	os.Setenv(kbEnvPodUID, "uid-abc")
	os.Setenv(kbEnvPodName, "pod-1")
	defer os.Unsetenv(kbEnvPodUID)
	defer os.Unsetenv(kbEnvPodName)

	name1 := generateEventName("reason", "msg")
	name2 := generateEventName("reason", "msg")
	assert.Equal(t, name1, name2)
}

func TestGenerateEventName_DifferentReasons(t *testing.T) {
	os.Setenv(kbEnvPodUID, "uid-abc")
	os.Setenv(kbEnvPodName, "pod-1")
	defer os.Unsetenv(kbEnvPodUID)
	defer os.Unsetenv(kbEnvPodName)

	name1 := generateEventName("reason-a", "msg")
	name2 := generateEventName("reason-b", "msg")
	assert.NotEqual(t, name1, name2)
}

func TestGenerateEventName_DifferentMessages(t *testing.T) {
	os.Setenv(kbEnvPodUID, "uid-abc")
	os.Setenv(kbEnvPodName, "pod-1")
	defer os.Unsetenv(kbEnvPodUID)
	defer os.Unsetenv(kbEnvPodName)

	name1 := generateEventName("reason", "msg-a")
	name2 := generateEventName("reason", "msg-b")
	assert.NotEqual(t, name1, name2)
}

// --- newEvent ---

func TestNewEvent_Basic(t *testing.T) {
	os.Setenv(kbEnvNamespace, "test-ns")
	os.Setenv(kbEnvPodName, "test-pod-0")
	os.Setenv(kbEnvPodUID, "test-uid-123")
	os.Setenv(kbEnvNodeName, "test-node-1")
	defer func() {
		os.Unsetenv(kbEnvNamespace)
		os.Unsetenv(kbEnvPodName)
		os.Unsetenv(kbEnvPodUID)
		os.Unsetenv(kbEnvNodeName)
	}()

	event := newEvent("roleProbe", `{"probe":"roleProbe","code":0}`)
	require.NotNil(t, event)
	assert.Equal(t, "test-ns", event.Namespace)
	assert.Equal(t, "roleProbe", event.Reason)
	assert.Equal(t, `{"probe":"roleProbe","code":0}`, event.Message)
	assert.Equal(t, "Normal", event.Type)
	assert.Equal(t, int32(1), event.Count)
	assert.Equal(t, "roleProbe", event.Action)
	assert.Equal(t, proto.ProbeEventReportingController, event.ReportingController)
	assert.Equal(t, "test-pod-0", event.ReportingInstance)
}

func TestNewEvent_InvolvedObject(t *testing.T) {
	os.Setenv(kbEnvNamespace, "ns")
	os.Setenv(kbEnvPodName, "pod")
	os.Setenv(kbEnvPodUID, "uid")
	defer func() {
		os.Unsetenv(kbEnvNamespace)
		os.Unsetenv(kbEnvPodName)
		os.Unsetenv(kbEnvPodUID)
	}()

	event := newEvent("test-reason", "test-msg")
	assert.Equal(t, "Pod", event.InvolvedObject.Kind)
	assert.Equal(t, "ns", event.InvolvedObject.Namespace)
	assert.Equal(t, "pod", event.InvolvedObject.Name)
	assert.Equal(t, "uid", string(event.InvolvedObject.UID))
	assert.Equal(t, proto.ProbeEventFieldPath, event.InvolvedObject.FieldPath)
}

func TestNewEvent_Source(t *testing.T) {
	os.Setenv(kbEnvNodeName, "worker-1")
	defer os.Unsetenv(kbEnvNodeName)

	event := newEvent("reason", "msg")
	assert.Equal(t, proto.ProbeEventSourceComponent, event.Source.Component)
	assert.Equal(t, "worker-1", event.Source.Host)
}

func TestNewEvent_Timestamps(t *testing.T) {
	before := time.Now().Add(-1 * time.Second)
	event := newEvent("reason", "msg")
	after := time.Now().Add(1 * time.Second)

	assert.True(t, event.FirstTimestamp.Time.After(before))
	assert.True(t, event.FirstTimestamp.Time.Before(after))
	assert.True(t, event.LastTimestamp.Time.After(before))
	assert.True(t, event.LastTimestamp.Time.Before(after))
	assert.True(t, event.EventTime.Time.After(before))
	assert.True(t, event.EventTime.Time.Before(after))
}

func TestNewEvent_Name(t *testing.T) {
	os.Setenv(kbEnvPodUID, "uid-for-name")
	os.Setenv(kbEnvPodName, "pod-name")
	defer func() {
		os.Unsetenv(kbEnvPodUID)
		os.Unsetenv(kbEnvPodName)
	}()

	event := newEvent("reason", "msg")
	expected := generateEventName("reason", "msg")
	assert.Equal(t, expected, event.Name)
}

// --- SendEventWithMessage ---

func TestSendEventWithMessage_AsyncNoError(t *testing.T) {
	// Async path: goroutine calls createOrUpdateEvent which will fail (no K8s client),
	// but SendEventWithMessage itself returns nil immediately for async.
	logger := logr.New(nil)
	err := SendEventWithMessage(&logger, "test-reason", "test-msg", false)
	assert.NoError(t, err)
	// Give goroutine time to run (it will fail internally, that's fine)
	time.Sleep(50 * time.Millisecond)
}

func TestSendEventWithMessage_SyncError(t *testing.T) {
	// Sync path: createOrUpdateEvent will fail because there's no K8s config.
	// This exercises the sync branch and the error logging.
	logger := logr.New(nil)
	err := SendEventWithMessage(&logger, "test-reason", "test-msg", true)
	assert.Error(t, err)
}

func TestSendEventWithMessage_NilLogger_Sync(t *testing.T) {
	// Should not panic even with nil logger
	err := SendEventWithMessage(nil, "reason", "msg", true)
	assert.Error(t, err)
}

func TestSendEventWithMessage_NilLogger_Async(t *testing.T) {
	err := SendEventWithMessage(nil, "reason", "msg", false)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
}

// --- createOrUpdateEvent ---

func TestCreateOrUpdateEvent_NoK8sConfig(t *testing.T) {
	// Without KUBECONFIG or in-cluster config, getK8sClientSet fails
	err := createOrUpdateEvent("reason", "msg", 0, 1)
	assert.Error(t, err)
}

func TestCreateOrUpdateEvent_SingleAttempt(t *testing.T) {
	err := createOrUpdateEvent("reason", "msg", 0, 1)
	assert.Error(t, err)
}

func TestCreateOrUpdateEvent_ZeroAttempts(t *testing.T) {
	// When retryAttempts is 0, max(0,1)=1, so it still tries once
	err := createOrUpdateEvent("reason", "msg", 0, 0)
	assert.Error(t, err)
}

// --- getK8sClientSet ---

func TestGetK8sClientSet_ReturnsClientSet(t *testing.T) {
	// getK8sClientSet uses controller-runtime GetConfig, which reads
	// KUBECONFIG or in-cluster config. We just verify it doesn't panic
	// and returns either a valid client set or an error.
	clientSet, err := getK8sClientSet()
	if err != nil {
		assert.Nil(t, clientSet)
	} else {
		assert.NotNil(t, clientSet)
	}
}
