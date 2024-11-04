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
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	defaultProbePeriodSeconds = 60
)

func newProbeService(logger logr.Logger, actionService *actionService, probes []proto.Probe) (*probeService, error) {
	sp := &probeService{
		logger:        logger,
		actionService: actionService,
		probes:        make(map[string]*proto.Probe),
		runners:       make(map[string]*probeRunner),
	}
	for i, p := range probes {
		if _, ok := actionService.actions[p.Action]; !ok {
			return nil, fmt.Errorf("probe %s has no action defined", p.Action)
		}
		sp.probes[p.Action] = &probes[i]
	}
	logger.Info(fmt.Sprintf("create service %s", sp.Kind()), "probes", strings.Join(maps.Keys(sp.probes), ","))
	return sp, nil
}

type probeService struct {
	logger        logr.Logger
	actionService *actionService
	probes        map[string]*proto.Probe
	runners       map[string]*probeRunner
}

var _ Service = &probeService{}

func (s *probeService) Kind() string {
	return proto.ServiceProbe.Kind
}

func (s *probeService) URI() string {
	return proto.ServiceProbe.URI
}

func (s *probeService) Start() error {
	for name := range s.probes {
		runner := &probeRunner{
			logger:        s.logger.WithValues("probe", name),
			actionService: s.actionService,
			latestEvent:   make(chan proto.ProbeEvent, 1),
		}
		go runner.run(s.probes[name])
		s.runners[name] = runner
	}
	return nil
}

func (s *probeService) HandleRequest(ctx context.Context, payload []byte) ([]byte, error) {
	return nil, errors.Wrapf(proto.ErrNotImplemented, "service %s does not support request handling", s.Kind())
}

type probeRunner struct {
	logger        logr.Logger
	actionService *actionService
	ticker        *time.Ticker
	succeedCount  int64
	failedCount   int64
	latestOutput  []byte
	latestEvent   chan proto.ProbeEvent
}

func (r *probeRunner) run(probe *proto.Probe) {
	r.logger.Info("probe started", "config", probe)

	if probe.InitialDelaySeconds > 0 {
		time.Sleep(time.Duration(probe.InitialDelaySeconds) * time.Second)
	}

	// launch report loop first
	r.launchReportLoop(probe)

	r.launchRunLoop(probe)
}

func (r *probeRunner) launchRunLoop(probe *proto.Probe) {
	if probe.PeriodSeconds <= 0 {
		probe.PeriodSeconds = defaultProbePeriodSeconds
	}
	r.ticker = time.NewTicker(time.Duration(probe.PeriodSeconds) * time.Second)
	defer r.ticker.Stop()

	r.runLoop(probe)
}

func (r *probeRunner) runLoop(probe *proto.Probe) {
	runOnce := func() ([]byte, error) {
		return r.actionService.handleRequest(context.Background(), &proto.ActionRequest{Action: probe.Action})
	}

	for range r.ticker.C {
		output, err := runOnce()
		if err == nil {
			r.succeedCount++
			r.failedCount = 0
		} else {
			r.succeedCount = 0
			r.failedCount++
		}

		r.report(probe, output, err)

		if succeed, _ := r.succeed(probe); succeed && !reflect.DeepEqual(output, r.latestOutput) {
			r.latestOutput = output
		}
	}
}

func (r *probeRunner) launchReportLoop(probe *proto.Probe) {
	if probe.ReportPeriodSeconds <= 0 {
		return
	}

	if probe.ReportPeriodSeconds < probe.PeriodSeconds {
		probe.ReportPeriodSeconds = probe.PeriodSeconds
	}

	go func() {
		ticker := time.NewTicker(time.Duration(probe.ReportPeriodSeconds) * time.Second)
		defer ticker.Stop()

		var latestReportedEvent *proto.ProbeEvent
		for range ticker.C {
			latestEvent := gather(r.latestEvent)
			if latestEvent == nil && latestReportedEvent != nil {
				latestEvent = latestReportedEvent
			}
			if latestEvent != nil {
				r.logger.Info("report probe event periodically",
					"code", latestEvent.Code, "output", outputPrefix(latestEvent.Output), "message", latestEvent.Message)
				r.sendEvent(latestEvent)
			}
			latestReportedEvent = latestEvent
		}
	}()
}

func (r *probeRunner) report(probe *proto.Probe, output []byte, err error) {
	var latestEvent *proto.ProbeEvent

	succeed, thresholdPoint := r.succeed(probe)
	if succeed && thresholdPoint ||
		succeed && !thresholdPoint && !reflect.DeepEqual(output, r.latestOutput) {
		latestEvent = r.buildNSendEvent(probe.Instance, probe.Action, 0, output, "")
	}
	if r.fail(probe) {
		latestEvent = r.buildNSendEvent(probe.Instance, probe.Action, -1, r.latestOutput, err.Error())
	}

	if latestEvent != nil {
		select {
		case r.latestEvent <- *latestEvent:
		default:
			gather(r.latestEvent) // drain the channel
			r.latestEvent <- *latestEvent
		}
	}
}

func (r *probeRunner) succeed(probe *proto.Probe) (bool, bool) {
	if r.succeedCount > 0 {
		successThreshold := probe.SuccessThreshold
		if successThreshold <= 0 {
			successThreshold = 1
		}
		return r.succeedCount >= int64(successThreshold), r.succeedCount == int64(successThreshold)
	}
	return false, false
}

func (r *probeRunner) fail(probe *proto.Probe) bool {
	if r.failedCount > 0 {
		failureThreshold := probe.FailureThreshold
		if failureThreshold <= 0 {
			failureThreshold = 1
		}
		return r.failedCount >= int64(failureThreshold)
	}
	return false
}

func (r *probeRunner) buildNSendEvent(instance, probe string, code int32, output []byte, message string) *proto.ProbeEvent {
	r.logger.Info("send probe event", "probe", probe, "code", code, "output", outputPrefix(output), "message", message)
	event := &proto.ProbeEvent{
		Instance: instance,
		Probe:    probe,
		Code:     code,
		Output:   output,
		Message:  message,
	}
	r.sendEvent(event)
	return event
}

func (r *probeRunner) sendEvent(event *proto.ProbeEvent) {
	msg, err := json.Marshal(&event)
	if err == nil {
		util.SendEventWithMessage(&r.logger, event.Probe, string(msg))
	} else {
		r.logger.Error(err, fmt.Sprintf("failed to marshal probe event, probe: %s", event.Probe))
	}
}

func outputPrefix(output []byte) string {
	prefixLen := min(len(output), 32)
	return string(output[:prefixLen])
}
