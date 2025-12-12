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
	"encoding/json"
	"fmt"
	"net"
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

var (
	defaultRetrySendEventInterval = 1 * time.Minute
	retrySendEventInterval        = defaultRetrySendEventInterval
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
	logger               logr.Logger
	actionService        *actionService
	probes               map[string]*proto.Probe
	runners              map[string]*probeRunner
	sendEventWithMessage func(logger *logr.Logger, reason string, message string, sync bool) error
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
			logger:               s.logger.WithValues("probe", name),
			actionService:        s.actionService,
			latestEvent:          make(chan proto.ProbeEvent, 1),
			sendEventWithMessage: s.sendEventWithMessage,
		}
		go runner.run(s.probes[name])
		s.runners[name] = runner
	}
	return nil
}

func (s *probeService) HandleConn(context.Context, net.Conn) error {
	return nil
}

func (s *probeService) HandleRequest(context.Context, []byte) ([]byte, error) {
	return nil, errors.Wrapf(proto.ErrNotImplemented, "service %s does not support request handling", s.Kind())
}

type probeRunner struct {
	logger               logr.Logger
	actionService        *actionService
	ticker               *time.Ticker
	succeedCount         int64
	failedCount          int64
	latestOutput         []byte
	latestEvent          chan proto.ProbeEvent
	sendEventWithMessage func(logger *logr.Logger, reason string, message string, sync bool) error
}

func (r *probeRunner) run(probe *proto.Probe) {
	r.logger.Info("probe started", "config", probe)

	if probe.InitialDelaySeconds > 0 {
		time.Sleep(time.Duration(probe.InitialDelaySeconds) * time.Second)
	}

	// launch the report loop first
	r.launchReportLoop(probe)

	r.launchProbeLoop(probe)
}

func (r *probeRunner) launchProbeLoop(probe *proto.Probe) {
	if probe.PeriodSeconds <= 0 {
		probe.PeriodSeconds = defaultProbePeriodSeconds
	}
	r.ticker = time.NewTicker(time.Duration(probe.PeriodSeconds) * time.Second)
	defer r.ticker.Stop()

	r.probeLoop(probe)
}

func (r *probeRunner) probeLoop(probe *proto.Probe) {
	once := func() {
		output, err := r.actionService.handleRequest(context.Background(), &proto.ActionRequest{Action: probe.Action})
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

	// initial run
	once()

	for range r.ticker.C {
		once()
	}
}

func (r *probeRunner) report(probe *proto.Probe, output []byte, err error) {
	var latestEvent *proto.ProbeEvent

	succeed, thresholdPoint := r.succeed(probe)
	if succeed && thresholdPoint ||
		succeed && !thresholdPoint && !reflect.DeepEqual(output, r.latestOutput) {
		latestEvent = r.buildEvent(probe.Instance, probe.Action, 0, output, "")
	}
	if r.fail(probe) {
		latestEvent = r.buildEvent(probe.Instance, probe.Action, -1, r.latestOutput, err.Error())
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

func (r *probeRunner) buildEvent(instance, probe string, code int32, output []byte, message string) *proto.ProbeEvent {
	return &proto.ProbeEvent{
		Instance: instance,
		Probe:    probe,
		Code:     code,
		Output:   output,
		Message:  message,
	}
}

func (r *probeRunner) launchReportLoop(probe *proto.Probe) {
	go func() {
		var reportChan <-chan time.Time
		if probe.ReportPeriodSeconds > 0 {
			if probe.ReportPeriodSeconds < probe.PeriodSeconds {
				probe.ReportPeriodSeconds = probe.PeriodSeconds
			}
			ticker := time.NewTicker(time.Duration(probe.ReportPeriodSeconds) * time.Second)
			defer ticker.Stop()
			reportChan = ticker.C
		}

		retryTicker := time.NewTicker(retrySendEventInterval)
		defer retryTicker.Stop()

		var event proto.ProbeEvent
		var hasEvent bool

		output := func() string {
			prefixLen := min(len(event.Output), 32)
			return string(event.Output[:prefixLen])
		}

		log := func(err error, msg string, retry, periodically bool) {
			if err == nil {
				r.logger.Info(msg, "retry", retry, "periodically", periodically,
					"probe", probe.Action, "code", event.Code, "output", output(), "message", event.Message)
			} else {
				r.logger.Error(err, msg, "retry", retry, "periodically", periodically,
					"probe", probe.Action, "code", event.Code, "output", output(), "message", event.Message)
			}
		}

		trySend := func(retry, periodically bool) bool {
			if !hasEvent {
				return true
			}

			msg, err := json.Marshal(event)
			if err != nil {
				log(err, "failed to marshal the probe event", retry, periodically)
				return true
			}

			if r.sendEventWithMessage == nil {
				r.sendEventWithMessage = util.SendEventWithMessage
			}
			err = r.sendEventWithMessage(&r.logger, event.Probe, string(msg), true)
			if err == nil {
				log(nil, "succeed to send the probe event", retry, periodically)
			} else {
				log(err, "failed to send the probe event, will retry later", retry, periodically)
			}
			return err == nil
		}

		needsRetry := false
		for {
			select {
			case latest := <-r.latestEvent:
				event = latest
				hasEvent = true
				needsRetry = !trySend(false, false)

			case <-reportChan:
				needsRetry = !trySend(false, true)

			case <-retryTicker.C:
				if needsRetry {
					needsRetry = !trySend(true, false)
				}
			}
		}
	}()
}
