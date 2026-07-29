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

package kbagent

import (
	"context"
	"errors"
	"net"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2/ktesting"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/server"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

type setupFakeService struct {
	kind     string
	uri      string
	startErr error
}

func (s setupFakeService) Kind() string {
	return s.kind
}

func (s setupFakeService) URI() string {
	return s.uri
}

func (s setupFakeService) Start() error {
	return s.startErr
}

func (s setupFakeService) HandleConn(context.Context, net.Conn) error {
	return nil
}

func (s setupFakeService) HandleRequest(context.Context, []byte) ([]byte, error) {
	return nil, nil
}

func TestBuildEnv4Server(t *testing.T) {
	env, err := BuildEnv4Server(
		[]proto.Action{{Name: "backup", Exec: &proto.ExecAction{Commands: []string{"echo"}}}},
		[]proto.Probe{{Instance: "pod", Action: "backup"}},
		[]string{"backup"},
	)
	if err != nil {
		t.Fatalf("BuildEnv4Server() error = %v", err)
	}

	got := envVarMap(env)
	if got[actionEnvName] == "" || got[probeEnvName] == "" || got[streamingEnvName] != "backup" {
		t.Fatalf("unexpected env: %#v", got)
	}

	actions, probes, err := deserializeActionNProbe(got[actionEnvName], got[probeEnvName])
	if err != nil {
		t.Fatalf("deserialize action/probe: %v", err)
	}
	if len(actions) != 1 || actions[0].Name != "backup" || len(probes) != 1 || probes[0].Action != "backup" {
		t.Fatalf("unexpected action/probe: %#v %#v", actions, probes)
	}
}

func TestBuildAndUpdateEnv4Worker(t *testing.T) {
	taskEnv, err := BuildEnv4Worker([]proto.Task{{Instance: "inst", Task: "new-replica", UID: "u1", Replicas: "pod-0"}})
	if err != nil {
		t.Fatalf("BuildEnv4Worker() error = %v", err)
	}
	if taskEnv.Name != taskEnvName || taskEnv.Value == "" {
		t.Fatalf("unexpected task env: %#v", taskEnv)
	}

	updated, err := UpdateEnv4Worker(map[string]string{taskEnvName: taskEnv.Value}, func(task proto.Task) *proto.Task {
		task.Replicas = "pod-1"
		return &task
	})
	if err != nil {
		t.Fatalf("UpdateEnv4Worker() error = %v", err)
	}
	tasks, err := deserializeTask(updated.Value)
	if err != nil {
		t.Fatalf("deserialize task: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Replicas != "pod-1" {
		t.Fatalf("unexpected updated tasks: %#v", tasks)
	}

	updated, err = UpdateEnv4Worker(map[string]string{taskEnvName: taskEnv.Value}, func(proto.Task) *proto.Task { return nil })
	if err != nil {
		t.Fatalf("UpdateEnv4Worker remove error = %v", err)
	}
	tasks, err = deserializeTask(updated.Value)
	if err != nil {
		t.Fatalf("deserialize removed tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected removed tasks, got %#v", tasks)
	}

	if updated, err = UpdateEnv4Worker(nil, nil); updated != nil || err != nil {
		t.Fatalf("UpdateEnv4Worker(nil) = %#v, %v", updated, err)
	}
	if updated, err = UpdateEnv4Worker(map[string]string{}, nil); updated != nil || err != nil {
		t.Fatalf("UpdateEnv4Worker(no task) = %#v, %v", updated, err)
	}
	if updated, err = UpdateEnv4Worker(map[string]string{taskEnvName: "{"}, nil); updated != nil || err == nil {
		t.Fatalf("expected invalid task env error, got %#v, %v", updated, err)
	}
}

func TestInitializeAndEnvAccessors(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	actionsEnv, _, err := serializeActionNProbe([]proto.Action{{Name: "dump", Exec: &proto.ExecAction{Commands: []string{"echo"}}}}, nil)
	if err != nil {
		t.Fatalf("serialize action/probe: %v", err)
	}

	services, err := initialize(logger, map[string]string{actionEnvName: actionsEnv, streamingEnvName: "dump"})
	if err != nil {
		t.Fatalf("initialize() error = %v", err)
	}
	if actionService(services) == nil || streamingService(services) == nil {
		t.Fatalf("expected action and streaming services, got %#v", services)
	}

	if services, err = initialize(logger, nil); services != nil || err != nil {
		t.Fatalf("initialize(nil) = %#v, %v", services, err)
	}
	if services, err = initialize(logger, map[string]string{actionEnvName: "{"}); services != nil || err == nil {
		t.Fatalf("expected invalid action env error, got %#v, %v", services, err)
	}

	da, dp, ds := getActionProbeNStreamingEnvValues(map[string]string{
		actionEnvName:    "a",
		probeEnvName:     "p",
		streamingEnvName: "s",
	})
	if da != "a" || dp != "p" || ds != "s" {
		t.Fatalf("unexpected env values: %q %q %q", da, dp, ds)
	}
	da, dp, ds = getActionProbeNStreamingEnvValues(map[string]string{probeEnvName: "p"})
	if da != "" || dp != "" || ds != "" {
		t.Fatalf("unexpected missing action values: %q %q %q", da, dp, ds)
	}
}

func TestSerializeDeserializeTask(t *testing.T) {
	serialized, err := serializeTask([]proto.Task{{Instance: "inst", Task: "task", UID: "u1"}})
	if err != nil {
		t.Fatalf("serializeTask() error = %v", err)
	}
	tasks, err := deserializeTask(serialized)
	if err != nil {
		t.Fatalf("deserializeTask() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].UID != "u1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if tasks, err = deserializeTask("{"); tasks != nil || err == nil {
		t.Fatalf("expected invalid task error, got %#v, %v", tasks, err)
	}
}

func TestRunAsServerStableErrors(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	if err := runAsServer(logger, server.Config{Port: 3501, StreamingPort: 3501}, nil); err == nil {
		t.Fatalf("expected same port error")
	}

	startErr := errors.New("start")
	err := runAsServer(logger, server.Config{Port: 3501, StreamingPort: 3502}, []service.Service{
		setupFakeService{kind: proto.ServiceAction.Kind, uri: proto.ServiceAction.URI, startErr: startErr},
	})
	if !errors.Is(err, startErr) {
		t.Fatalf("runAsServer start error = %v, want %v", err, startErr)
	}
}

func TestRunAsServerStartsHTTPWithNoStreamingService(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	err := runAsServer(logger, server.Config{Address: "127.0.0.1", Port: 0, StreamingPort: 3502}, []service.Service{
		setupFakeService{kind: proto.ServiceAction.Kind, uri: proto.ServiceAction.URI},
	})
	if err != nil {
		t.Fatalf("runAsServer() error = %v", err)
	}
}

func TestRunAsWorkerStableBranches(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	if err := runAsWorker(logger, nil, nil); err != nil {
		t.Fatalf("runAsWorker(nil env) error = %v", err)
	}
	if err := runAsWorker(logger, nil, map[string]string{taskEnvName: "{"}); err == nil {
		t.Fatalf("expected invalid task error")
	}
}

func TestLaunchWorkerWithoutTask(t *testing.T) {
	t.Setenv(actionEnvName, "")
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	serverMode, err := Launch(logger, server.Config{})
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if serverMode {
		t.Fatalf("expected worker mode")
	}
}

func TestServiceSelectors(t *testing.T) {
	services := []service.Service{
		setupFakeService{kind: proto.ServiceProbe.Kind, uri: proto.ServiceProbe.URI},
		setupFakeService{kind: proto.ServiceAction.Kind, uri: proto.ServiceAction.URI},
		setupFakeService{kind: proto.ServiceStreaming.Kind, uri: proto.ServiceStreaming.URI},
	}
	if actionService(services) == nil {
		t.Fatalf("expected action service")
	}
	if streamingService(services) == nil {
		t.Fatalf("expected streaming service")
	}
	if actionService(nil) != nil || streamingService(nil) != nil {
		t.Fatalf("expected nil selectors for empty services")
	}
}

func envVarMap(vars []corev1.EnvVar) map[string]string {
	got := make(map[string]string, len(vars))
	for _, env := range vars {
		got[env.Name] = env.Value
	}
	return got
}
