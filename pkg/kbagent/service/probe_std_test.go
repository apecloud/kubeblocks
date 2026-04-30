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

package service

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

// --- probeService URI / HandleConn / HandleRequest ---

func TestProbeService_URI(t *testing.T) {
	svc, err := newProbeService(logr.New(nil), newTestActionService(t, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, proto.ServiceProbe.URI, svc.URI())
}

func TestProbeService_HandleConn(t *testing.T) {
	svc, err := newProbeService(logr.New(nil), newTestActionService(t, nil), nil)
	require.NoError(t, err)
	assert.NoError(t, svc.HandleConn(context.Background(), nil))
}

func TestProbeService_HandleRequest_NotImplemented(t *testing.T) {
	svc, err := newProbeService(logr.New(nil), newTestActionService(t, nil), nil)
	require.NoError(t, err)
	_, err = svc.HandleRequest(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, proto.ErrNotImplemented))
}

// --- probeRunner succeed ---

func TestProbeRunner_Succeed_BelowThreshold(t *testing.T) {
	r := &probeRunner{succeedCount: 1}
	probe := &proto.Probe{SuccessThreshold: 3}
	ok, thresholdPoint := r.succeed(probe)
	assert.False(t, ok)
	assert.False(t, thresholdPoint)
}

func TestProbeRunner_Succeed_AtThreshold(t *testing.T) {
	r := &probeRunner{succeedCount: 3}
	probe := &proto.Probe{SuccessThreshold: 3}
	ok, thresholdPoint := r.succeed(probe)
	assert.True(t, ok)
	assert.True(t, thresholdPoint)
}

func TestProbeRunner_Succeed_AboveThreshold(t *testing.T) {
	r := &probeRunner{succeedCount: 5}
	probe := &proto.Probe{SuccessThreshold: 3}
	ok, thresholdPoint := r.succeed(probe)
	assert.True(t, ok)
	assert.False(t, thresholdPoint)
}

func TestProbeRunner_Succeed_ZeroCount(t *testing.T) {
	r := &probeRunner{succeedCount: 0}
	probe := &proto.Probe{SuccessThreshold: 1}
	ok, thresholdPoint := r.succeed(probe)
	assert.False(t, ok)
	assert.False(t, thresholdPoint)
}

func TestProbeRunner_Succeed_DefaultThreshold(t *testing.T) {
	r := &probeRunner{succeedCount: 1}
	probe := &proto.Probe{SuccessThreshold: 0} // defaults to 1
	ok, thresholdPoint := r.succeed(probe)
	assert.True(t, ok)
	assert.True(t, thresholdPoint)
}

// --- probeRunner fail ---

func TestProbeRunner_Fail_BelowThreshold(t *testing.T) {
	r := &probeRunner{failedCount: 1}
	probe := &proto.Probe{FailureThreshold: 3}
	assert.False(t, r.fail(probe))
}

func TestProbeRunner_Fail_AtThreshold(t *testing.T) {
	r := &probeRunner{failedCount: 3}
	probe := &proto.Probe{FailureThreshold: 3}
	assert.True(t, r.fail(probe))
}

func TestProbeRunner_Fail_ZeroCount(t *testing.T) {
	r := &probeRunner{failedCount: 0}
	probe := &proto.Probe{FailureThreshold: 1}
	assert.False(t, r.fail(probe))
}

func TestProbeRunner_Fail_DefaultThreshold(t *testing.T) {
	r := &probeRunner{failedCount: 1}
	probe := &proto.Probe{FailureThreshold: 0} // defaults to 1
	assert.True(t, r.fail(probe))
}

// --- probeRunner buildEvent ---

func TestProbeRunner_BuildEvent(t *testing.T) {
	r := &probeRunner{}
	event := r.buildEvent("inst-1", "roleProbe", 0, []byte("leader"), "")
	assert.Equal(t, "inst-1", event.Instance)
	assert.Equal(t, "roleProbe", event.Probe)
	assert.Equal(t, int32(0), event.Code)
	assert.Equal(t, []byte("leader"), event.Output)
	assert.Empty(t, event.Message)
}

func TestProbeRunner_BuildEvent_Failed(t *testing.T) {
	r := &probeRunner{}
	event := r.buildEvent("inst-1", "roleProbe", -1, nil, "connection refused")
	assert.Equal(t, int32(-1), event.Code)
	assert.Equal(t, "connection refused", event.Message)
}

// --- probeRunner report ---

func TestProbeRunner_Report_SuccessAtThreshold(t *testing.T) {
	r := &probeRunner{
		logger:      logr.New(nil),
		succeedCount: 1,
		latestEvent: make(chan proto.ProbeEvent, 1),
	}
	probe := &proto.Probe{
		Action:           "roleProbe",
		Instance:         "inst-1",
		SuccessThreshold: 1,
		FailureThreshold: 1,
	}
	r.report(probe, []byte("leader"), nil)
	select {
	case ev := <-r.latestEvent:
		assert.Equal(t, int32(0), ev.Code)
		assert.Equal(t, []byte("leader"), ev.Output)
	default:
		t.Fatal("expected event to be sent")
	}
}

func TestProbeRunner_Report_FailureAtThreshold(t *testing.T) {
	r := &probeRunner{
		logger:      logr.New(nil),
		failedCount: 1,
		latestEvent: make(chan proto.ProbeEvent, 1),
	}
	probe := &proto.Probe{
		Action:           "roleProbe",
		Instance:         "inst-1",
		SuccessThreshold: 1,
		FailureThreshold: 1,
	}
	err := assert.AnError
	r.report(probe, nil, err)
	select {
	case ev := <-r.latestEvent:
		assert.Equal(t, int32(-1), ev.Code)
	default:
		t.Fatal("expected event to be sent")
	}
}

func TestProbeRunner_Report_NoEventBelowThreshold(t *testing.T) {
	r := &probeRunner{
		logger:      logr.New(nil),
		succeedCount: 1,
		latestEvent: make(chan proto.ProbeEvent, 1),
	}
	probe := &proto.Probe{
		Action:           "roleProbe",
		SuccessThreshold: 5,
		FailureThreshold: 5,
	}
	r.report(probe, []byte("data"), nil)
	select {
	case <-r.latestEvent:
		t.Fatal("no event should be sent below threshold")
	default:
		// expected
	}
}

func TestProbeRunner_Report_SuccessAboveThreshold_OutputChanged(t *testing.T) {
	r := &probeRunner{
		logger:       logr.New(nil),
		succeedCount: 3,
		latestOutput: []byte("old"),
		latestEvent:  make(chan proto.ProbeEvent, 1),
	}
	probe := &proto.Probe{
		Action:           "roleProbe",
		Instance:         "inst-1",
		SuccessThreshold: 2,
	}
	r.report(probe, []byte("new"), nil)
	select {
	case ev := <-r.latestEvent:
		assert.Equal(t, int32(0), ev.Code)
		assert.Equal(t, []byte("new"), ev.Output)
	default:
		t.Fatal("expected event when output changed above threshold")
	}
}

func TestProbeRunner_Report_SuccessAboveThreshold_OutputSame(t *testing.T) {
	r := &probeRunner{
		logger:       logr.New(nil),
		succeedCount: 3,
		latestOutput: []byte("same"),
		latestEvent:  make(chan proto.ProbeEvent, 1),
	}
	probe := &proto.Probe{
		Action:           "roleProbe",
		Instance:         "inst-1",
		SuccessThreshold: 2,
		FailureThreshold: 5,
	}
	r.report(probe, []byte("same"), nil)
	select {
	case <-r.latestEvent:
		t.Fatal("no event should be sent when output hasn't changed above threshold")
	default:
		// expected
	}
}

func TestProbeRunner_Report_ChannelFullDrain(t *testing.T) {
	r := &probeRunner{
		logger:       logr.New(nil),
		succeedCount: 1,
		latestEvent:  make(chan proto.ProbeEvent, 1),
	}
	// Fill channel
	r.latestEvent <- proto.ProbeEvent{Probe: "old"}
	probe := &proto.Probe{
		Action:           "roleProbe",
		Instance:         "inst-1",
		SuccessThreshold: 1,
	}
	r.report(probe, []byte("new"), nil)
	ev := <-r.latestEvent
	assert.Equal(t, "roleProbe", ev.Probe)
}
