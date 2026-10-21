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
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

func TestInitializeStreamingWithoutProbes(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	actions, _, err := serializeActionNProbe([]proto.Action{{Name: "dataDump", Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", "printf 'copied data'"}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	services, err := initialize(logger, map[string]string{actionEnvName: actions, streamingEnvName: "dataDump"})
	if err != nil {
		t.Fatal(err)
	}
	conn, peer := net.Pipe()
	defer conn.Close()
	defer peer.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		defer peer.Close()
		done <- streamingService(services).HandleConn(context.Background(), peer)
	}()
	if err := json.NewEncoder(conn).Encode(proto.ActionRequest{Action: "dataDump"}); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("configured dataDump did not execute: %v", err)
	}
	if string(data) != "copied data" {
		t.Fatalf("dataDump produced %q, want copied data", data)
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

func startWorkerDataSource(t *testing.T) (string, int32, <-chan error) {
	t.Helper()
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	services, err := service.New(logger, []proto.Action{{Name: "dataDump", Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", "printf 'copied data\\n'"}}}}, nil, []string{"dataDump"})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	finished := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			finished <- err
			return
		}
		defer conn.Close()
		finished <- streamingService(services).HandleConn(context.Background(), conn)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return host, int32(portNumber), finished
}

func configureDataWorker(t *testing.T, command string) (string, string, <-chan error) {
	t.Helper()
	t.Setenv("KB_AGENT_POD_NAME", "pod-1")
	dataPath := filepath.Join(t.TempDir(), "data")
	countPath := filepath.Join(t.TempDir(), "loads")
	host, port, source := startWorkerDataSource(t)
	envs, err := BuildEnv4Server([]proto.Action{{Name: "dataLoad", Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", command}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range envs {
		if env.ValueFrom == nil {
			t.Setenv(env.Name, env.Value)
		}
	}
	taskEnv, err := BuildEnv4Worker([]proto.Task{{Replicas: "pod-1", NewReplica: &proto.NewReplicaTask{Remote: host, Port: port, Parameters: map[string]string{"DATA_PATH": dataPath, "COUNT_PATH": countPath}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(taskEnv.Name, taskEnv.Value)
	return dataPath, countPath, source
}

func launchDataWorker(t *testing.T) error {
	t.Helper()
	serverMode, err := Launch(ktesting.NewLogger(t, ktesting.NewConfig()), server.Config{})
	if serverMode {
		t.Fatal("worker entered server mode")
	}
	return err
}

func TestWorkerStreamsDataWithoutResultStorage(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	dataPath, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`)
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "copied data\n" {
		t.Fatalf("loaded data=%q, %v", data, err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\n" {
		t.Fatalf("loads=%q, %v", loads, err)
	}
}

func TestWorkerLoadFailureAndRestartRetry(t *testing.T) {
	t.Setenv("LOAD_EXIT_CODE", "1")
	dataPath, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"; exit "$LOAD_EXIT_CODE"`)
	if err := launchDataWorker(t); err == nil {
		t.Fatal("failed load exited successfully")
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	host, port, source := startWorkerDataSource(t)
	tasks, err := deserializeTask(os.Getenv(taskEnvName))
	if err != nil {
		t.Fatal(err)
	}
	tasks[0].NewReplica.Remote, tasks[0].NewReplica.Port = host, port
	taskEnv, err := BuildEnv4Worker(tasks)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(taskEnv.Name, taskEnv.Value)
	t.Setenv("LOAD_EXIT_CODE", "0")
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "copied data\n" {
		t.Fatalf("retried data=%q, %v", data, err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\nload\n" {
		t.Fatalf("retry executions=%q, %v", loads, err)
	}
}
