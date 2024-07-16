/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"encoding/json"
	"fmt"
	"time"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	probeVersion = "v1.0"
	probeURI     = "probe"
)

func newProbeService(actionService *actionService, probes []proto.Probe) (*probeService, error) {
	sp := &probeService{}
	for i, p := range probes {
		if _, ok := actionService.actions[p.Action]; !ok {
			return nil, fmt.Errorf("probe %s has no action defined", p.Action)
		}
		sp.probes[p.Action] = &probes[i]
	}
	return sp, nil
}

type probeService struct {
	probes  map[string]*proto.Probe
	runners map[string]*probeRunner
}

var _ Service = &probeService{}

func (s *probeService) Kind() string {
	return "Probe"
}

func (s *probeService) Version() string {
	return probeVersion
}

func (s *probeService) URI() string {
	return probeURI
}

func (s *probeService) Start() error {
	for name := range s.probes {
		runner := &probeRunner{}
		go runner.run(s.probes[name])
		s.runners[name] = runner
	}
	return nil
}

func (s *probeService) Decode(payload []byte) (interface{}, error) {
	return nil, ErrNotImplemented
}

func (s *probeService) Call(ctx context.Context, req interface{}) ([]byte, error) {
	return nil, ErrNotImplemented
}

type probeRunner struct {
	ticker       *time.Ticker
	succeedCount int64
	failedCount  int64
	latestOutput []byte
}

func (r *probeRunner) run(probe *proto.Probe) {
	if probe.InitialDelaySeconds > 0 {
		time.Sleep(time.Duration(probe.InitialDelaySeconds) * time.Second)
	}

	r.ticker = time.NewTicker(time.Duration(probe.PeriodSeconds) * time.Second)
	defer r.ticker.Stop()

	r.runLoop(probe)
}

func (r *probeRunner) runLoop(probe *proto.Probe) {
	for range r.ticker.C {
		out, err := r.runOnce(probe)
		if err == nil {
			r.succeedCount++
			r.failedCount = 0
			r.latestOutput = out
		} else {
			r.succeedCount = 0
			r.failedCount++
		}
		r.report(probe, err)
	}
}

func (r *probeRunner) runOnce(probe *proto.Probe) ([]byte, error) {
	// TODO: call probe action
	return nil, nil
}

func (r *probeRunner) report(probe *proto.Probe, err error) {
	if r.succeedCount > 0 {
		if probe.SuccessThreshold > 0 && r.succeedCount == int64(probe.SuccessThreshold) ||
			probe.SuccessThreshold == 0 && r.succeedCount == 1 {
			r.sendEvent(probe.Action, 0, r.latestOutput, "")
		}
	}

	if r.failedCount > 0 {
		if probe.FailureThreshold > 0 && r.failedCount == int64(probe.FailureThreshold) ||
			probe.FailureThreshold == 0 && r.failedCount == 1 {
			r.sendEvent(probe.Action, -1, r.latestOutput, err.Error())
		}
	}
}

func (r *probeRunner) sendEvent(probe string, code int32, latestOutput []byte, message string) {
	eventMsg := &proto.ProbeEvent{
		Probe:        probe,
		Code:         code,
		Message:      message,
		LatestOutput: latestOutput,
	}
	msg, err := json.Marshal(&eventMsg)
	if err != nil {
		return
	}
	util.SendEventWithMessage(probe, string(msg))
}
