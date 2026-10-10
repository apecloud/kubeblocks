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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

func TestBuildEnv4DataLoadResult(t *testing.T) {
	if env, err := BuildEnv4DataLoadResult(nil); env != nil || err != nil {
		t.Fatalf("nil result = %v, %v", env, err)
	}
	if _, err := BuildEnv4DataLoadResult(&proto.DataLoadResult{PVCName: "data"}); err == nil {
		t.Fatal("expected missing namespace error")
	}
	result := &proto.DataLoadResult{Namespace: "runtime", PVCName: "data"}
	env, err := BuildEnv4DataLoadResult(result)
	if err != nil || env.Name != dataLoadResultEnvName {
		t.Fatalf("result env = %v, %v", env, err)
	}
	var decoded proto.DataLoadResult
	if err := json.Unmarshal([]byte(env.Value), &decoded); err != nil || decoded != *result {
		t.Fatalf("result roundtrip = %v, %v", decoded, err)
	}
}

// workerPVCAPI uses the real client HTTP boundary and resourceVersion conflicts.
// The streaming and load actions below execute real subprocesses.
type workerPVCAPI struct {
	mu            sync.Mutex
	pvc           *corev1.PersistentVolumeClaim
	readCode      int
	writeCode     int
	writeFailures int
	conflictOnce  bool
	writes        int
	saved         bool
}

func newWorkerPVCAPI(t *testing.T) *workerPVCAPI {
	t.Helper()
	api := &workerPVCAPI{pvc: &corev1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "runtime", ResourceVersion: "1", Annotations: map[string]string{"owner": "keep"}},
		Spec:       corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}},
	}}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	config := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"runtime": {Server: server.URL}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{"worker": {}},
		Contexts:       map[string]*clientcmdapi.Context{"runtime": {Cluster: "runtime", AuthInfo: "worker"}},
		CurrentContext: "runtime",
	}
	path := filepath.Join(t.TempDir(), "config")
	if err := clientcmd.WriteToFile(config, path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", path)
	return api
}

func (a *workerPVCAPI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if req.URL.Path != "/api/v1/namespaces/runtime/persistentvolumeclaims/data" {
		http.Error(w, "unexpected resource", http.StatusNotFound)
		return
	}
	status := func(code int) {
		reason := metav1.StatusReasonInternalError
		switch code {
		case http.StatusForbidden:
			reason = metav1.StatusReasonForbidden
		case http.StatusNotFound:
			reason = metav1.StatusReasonNotFound
		case http.StatusConflict:
			reason = metav1.StatusReasonConflict
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(&metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Failure", Reason: reason, Code: int32(code), Message: fmt.Sprintf("PVC request failed: %d", code)})
	}
	if req.Method == http.MethodGet {
		if a.readCode != 0 {
			status(a.readCode)
			return
		}
		_ = json.NewEncoder(w).Encode(a.pvc)
		return
	}
	if req.Method != http.MethodPut {
		status(http.StatusMethodNotAllowed)
		return
	}
	a.writes++
	if a.writeCode != 0 {
		status(a.writeCode)
		return
	}
	if a.writeFailures > 0 {
		a.writeFailures--
		status(http.StatusInternalServerError)
		return
	}
	if a.conflictOnce {
		a.conflictOnce = false
		a.pvc.ResourceVersion = "2"
		a.pvc.Annotations["concurrent"] = "keep"
		status(http.StatusConflict)
		return
	}
	var updated corev1.PersistentVolumeClaim
	if err := json.NewDecoder(req.Body).Decode(&updated); err != nil {
		status(http.StatusBadRequest)
		return
	}
	if updated.ResourceVersion != a.pvc.ResourceVersion {
		status(http.StatusConflict)
		return
	}
	a.pvc = &updated
	a.saved = updated.Annotations[proto.DataLoadedAnnotationKey] == "true"
	_ = json.NewEncoder(w).Encode(a.pvc)
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

func configureDataWorker(t *testing.T, command string, withResult bool) (string, string, <-chan error) {
	t.Helper()
	t.Setenv("KB_AGENT_POD_NAME", "pod-1")
	dataPath := filepath.Join(t.TempDir(), "data")
	countPath := filepath.Join(t.TempDir(), "loads")
	host, port, source := startWorkerDataSource(t)
	result := &proto.DataLoadResult{Namespace: "runtime", PVCName: "data"}
	task := proto.NewReplicaTask{Remote: host, Port: port, Parameters: map[string]string{"DATA_PATH": dataPath, "COUNT_PATH": countPath}}
	if withResult {
		task.DataLoadResult = result
	}
	serverEnv, err := BuildEnv4Server([]proto.Action{{Name: "dataLoad", Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", command}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range serverEnv {
		if env.ValueFrom == nil {
			t.Setenv(env.Name, env.Value)
		}
	}
	taskEnv, err := BuildEnv4Worker([]proto.Task{{Replicas: "pod-1", NewReplica: &task}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(taskEnv.Name, taskEnv.Value)
	t.Setenv(dataLoadResultEnvName, "")
	if withResult {
		resultEnv, err := BuildEnv4DataLoadResult(result)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv(resultEnv.Name, resultEnv.Value)
	}
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

func TestWorkerStreamsDataAndPersistsCompletionBeforeSuccess(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistent-result-%v", tracked), func(t *testing.T) {
			api := newWorkerPVCAPI(t)
			dataPath, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, tracked)
			if err := launchDataWorker(t); err != nil {
				t.Fatal(err)
			}
			if err := <-source; err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(dataPath)
			if err != nil || string(data) != "copied data\n" {
				t.Fatalf("loaded data = %q, %v", data, err)
			}
			loads, err := os.ReadFile(countPath)
			if err != nil || string(loads) != "load\n" {
				t.Fatalf("load executions = %q, %v", loads, err)
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			if api.saved != tracked {
				t.Fatalf("completion saved = %v, want %v", api.saved, tracked)
			}
			if api.pvc.Annotations["owner"] != "keep" || api.pvc.Spec.Resources.Requests.Storage().String() != "1Gi" {
				t.Fatalf("PVC fields lost: %#v", api.pvc)
			}
		})
	}
}

func TestWorkerLoadFailureDoesNotSaveAndRestartRetriesLoad(t *testing.T) {
	api := newWorkerPVCAPI(t)
	dataPath, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"; exit 1`, true)
	if err := launchDataWorker(t); err == nil {
		t.Fatal("failed load exited successfully")
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	api.mu.Lock()
	if api.saved || api.writes != 0 {
		t.Errorf("failed load persisted completion: %v, writes %d", api.saved, api.writes)
	}
	api.mu.Unlock()
	// Restart with the same data PVC and no saved result. The existing action
	// contract permits the load to run again before the database has started.
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
	serverEnv, err := BuildEnv4Server([]proto.Action{{Name: "dataLoad", Exec: &proto.ExecAction{Commands: []string{"/bin/sh", "-c", `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, env := range serverEnv {
		if env.ValueFrom == nil {
			t.Setenv(env.Name, env.Value)
		}
	}
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\nload\n" {
		t.Fatalf("restart executions = %q, %v", loads, err)
	}
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "copied data\n" {
		t.Fatalf("restart data = %q, %v", data, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if !api.saved {
		t.Fatal("restart did not save completed load")
	}
}

func TestWorkerSaveFailureRetriesWithoutReloadAndPreservesConcurrentUpdate(t *testing.T) {
	api := newWorkerPVCAPI(t)
	api.writeFailures = 1
	api.conflictOnce = true
	_, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, true)
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\n" {
		t.Fatalf("load executions = %q, %v", loads, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if !api.saved || api.writes != 3 || api.pvc.Annotations["concurrent"] != "keep" {
		t.Fatalf("save retry state: saved %v, writes %d, PVC %#v", api.saved, api.writes, api.pvc)
	}
}

func TestWorkerWritePermissionWaitsUntilRestoredWithoutReload(t *testing.T) {
	api := newWorkerPVCAPI(t)
	api.writeCode = http.StatusForbidden
	_, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, true)
	done := make(chan error, 1)
	go func() { done <- launchDataWorker(t) }()
	defer func() { api.mu.Lock(); api.writeCode = 0; api.mu.Unlock() }()
	deadline := time.After(5 * time.Second)
	for {
		api.mu.Lock()
		writes, saved := api.writes, api.saved
		api.mu.Unlock()
		if saved {
			t.Fatal("saved completion without PVC update permission")
		}
		if writes >= 2 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("worker stopped before permission restored: %v", err)
		case <-deadline:
			t.Fatal("worker did not retry forbidden result write")
		case <-time.After(20 * time.Millisecond):
		}
	}
	select {
	case err := <-done:
		t.Fatalf("worker completed without write permission: %v", err)
	default:
	}
	api.mu.Lock()
	api.writeCode = 0
	api.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not recover after permission restored")
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\n" {
		t.Fatalf("load executions = %q, %v", loads, err)
	}
}

func TestWorkerRestartSkipsSavedDataWithStaleTaskAndWithoutActions(t *testing.T) {
	api := newWorkerPVCAPI(t)
	_, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, true)
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	if err := <-source; err != nil {
		t.Fatal(err)
	}
	// Stale source endpoint is no longer serving. Neither dump nor load may run.
	// Exercise the task's own saved-result check without the startup env first.
	t.Setenv(dataLoadResultEnvName, "")
	if err := launchDataWorker(t); err != nil {
		t.Fatalf("restart with stale task: %v", err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\n" {
		t.Fatalf("load executions = %q, %v", loads, err)
	}
	t.Setenv(taskEnvName, "")
	t.Setenv(actionEnvName, "")
	env, err := BuildEnv4DataLoadResult(&proto.DataLoadResult{Namespace: "runtime", PVCName: "data"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(env.Name, env.Value)
	if err := launchDataWorker(t); err != nil {
		t.Fatalf("restart after task cleanup: %v", err)
	}
	api.mu.Lock()
	writes := api.writes
	// Replacing a PVC under the same name cannot inherit the previous result.
	api.pvc.Annotations = map[string]string{"owner": "replacement"}
	api.mu.Unlock()
	if writes != 1 {
		t.Fatalf("restart rewrote result %d times", writes)
	}
	if err := launchDataWorker(t); err == nil {
		t.Fatal("replacement PVC started without data loading")
	}
}

func TestWorkerStartupGuardBlocksAbsentResultAndAPIErrors(t *testing.T) {
	for _, code := range []int{0, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			api := newWorkerPVCAPI(t)
			api.readCode = code
			t.Setenv(actionEnvName, "")
			t.Setenv(taskEnvName, "")
			env, err := BuildEnv4DataLoadResult(&proto.DataLoadResult{Namespace: "runtime", PVCName: "data"})
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv(env.Name, env.Value)
			if err := launchDataWorker(t); err == nil {
				t.Fatal("worker allowed database startup without confirmed result")
			}
			// An old task targeting another Pod cannot satisfy the startup guard.
			taskEnv, err := BuildEnv4Worker([]proto.Task{{Replicas: "other-pod", NewReplica: &proto.NewReplicaTask{DataLoadResult: &proto.DataLoadResult{Namespace: "runtime", PVCName: "data"}}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv(taskEnv.Name, taskEnv.Value)
			if err := launchDataWorker(t); err == nil {
				t.Fatal("worker accepted task for another Pod")
			}
		})
	}
}

func TestWorkerDataTaskCannotLoadWhenResultCannotBeRead(t *testing.T) {
	api := newWorkerPVCAPI(t)
	api.readCode = http.StatusForbidden
	_, countPath, _ := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, true)
	// Exercise the task's own check without the independent startup guard.
	t.Setenv(dataLoadResultEnvName, "")
	if err := launchDataWorker(t); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("read permission error = %v", err)
	}
	if _, err := os.Stat(countPath); !os.IsNotExist(err) {
		t.Fatalf("load ran despite unavailable result: %v", err)
	}
}

func TestWorkerStartupGuardRequiresAPICredentials(t *testing.T) {
	t.Setenv(actionEnvName, "")
	t.Setenv(taskEnvName, "")
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	env, err := BuildEnv4DataLoadResult(&proto.DataLoadResult{Namespace: "runtime", PVCName: "data"})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(env.Name, env.Value)
	if err := launchDataWorker(t); err == nil {
		t.Fatal("worker allowed database startup without API credentials")
	}
}

func TestWorkerCompletedGuardSkipsOnlyItsDataTask(t *testing.T) {
	api := newWorkerPVCAPI(t)
	api.pvc.Annotations[proto.DataLoadedAnnotationKey] = "true"
	dataPath, countPath, source := configureDataWorker(t, `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"`, true)
	tasks, err := deserializeTask(os.Getenv(taskEnvName))
	if err != nil {
		t.Fatal(err)
	}
	// This retained task uses the same load action but has no PVC completion
	// contract. It must still execute after the tracked task is skipped.
	unrelated := tasks[0]
	unrelated.NewReplica = &proto.NewReplicaTask{
		Remote:     tasks[0].NewReplica.Remote,
		Port:       tasks[0].NewReplica.Port,
		Parameters: tasks[0].NewReplica.Parameters,
	}
	tasks[0].NewReplica.Remote = "invalid-source.invalid"
	env, err := BuildEnv4Worker(append(tasks, unrelated))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(env.Name, env.Value)
	if err := launchDataWorker(t); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-source:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("completed guard skipped the unrelated task")
	}
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "copied data\n" {
		t.Fatalf("unrelated task data = %q, %v", data, err)
	}
	loads, err := os.ReadFile(countPath)
	if err != nil || string(loads) != "load\n" {
		t.Fatalf("expected only unrelated load once, got %q, %v", loads, err)
	}
	api.mu.Lock()
	writes := api.writes
	api.mu.Unlock()
	if writes != 0 {
		t.Fatalf("skipped task rewrote its completion %d times", writes)
	}
	// Completed tracked inputs still work after action configuration cleanup.
	trackedEnv, err := BuildEnv4Worker(tasks)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(trackedEnv.Name, trackedEnv.Value)
	t.Setenv(actionEnvName, "")
	err = launchDataWorker(t)
	if err != nil {
		t.Fatalf("completed task required removed actions: %v", err)
	}
}
