/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package binding

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"
)

type fakeOperations struct {
	BaseOperations
}

const (
	testDBPort = 34521
	testRole   = "leader"
)

func TestInit(t *testing.T) {
	p := mockFakeOperations()
	p.Init(bindings.Metadata{})
	if p.OriRole != "" {
		t.Errorf("p.oriRole init failed: %s", p.OriRole)
	}
	if p.CheckRunningFailedCount != 0 {
		t.Errorf("p.CheckRunningFailedCount init failed: %d", p.CheckRunningFailedCount)
	}
	if p.CheckRoleFailedCount != 0 {
		t.Errorf("p.CheckRoleFailedCount init failed: %d", p.CheckRoleFailedCount)
	}
	if p.RoleUnchangedCount != 0 {
		t.Errorf("p.RoleUnchangedCount init failed: %d", p.RoleUnchangedCount)
	}
	if p.CheckFailedThreshold != defaultCheckFailedThreshold {
		t.Errorf("p.CheckFailedThreshold init failed: %d", p.CheckFailedThreshold)
	}
	if p.RoleDetectionThreshold != defaultRoleDetectionThreshold {
		t.Errorf("p.RoleDetectionThreshold init failed: %d", p.RoleDetectionThreshold)
	}
	if p.DBPort != testDBPort {
		t.Errorf("p.DBPort init failed: %d", p.DBPort)
	}
}

func TestInvoke(t *testing.T) {
	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
	p := mockFakeOperations()
	p.Init(bindings.Metadata{})

	t.Run("CheckRunning", func(t *testing.T) {
		opsRes := OpsResult{}
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: CheckRunningOperation,
		}
		resp, err := p.Invoke(context.Background(), req)
		if err != nil {
			t.Errorf("CheckRunning failed: %s", err)
		}
		err = json.Unmarshal(resp.Data, &opsRes)
		if err != nil {
			t.Errorf("CheckRunning failed: %s", err)
		}
		if opsRes["event"] != "CheckRunningFailed" {
			t.Errorf("unexpected response: %s", string(resp.Data))
		}

		server := startFooServer(p.DBPort, t)
		defer stopFooServer(server)
		resp, _ = p.Invoke(context.Background(), req)
		err = json.Unmarshal(resp.Data, &opsRes)
		if err != nil {
			t.Errorf("CheckRunning failed: %s", err)
		}
		if opsRes["event"] != "CheckRunningSuccess" {
			t.Errorf("unexpected response: %s", string(resp.Data))
		}
	})

	t.Run("roleCheck", func(t *testing.T) {
		opsRes := OpsResult{}
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: CheckRoleOperation,
		}

		resp, err := p.Invoke(context.Background(), req)
		if err != nil {
			t.Errorf("roleCheck error: %s", err)
		}
		err = json.Unmarshal(resp.Data, &opsRes)
		if err != nil {
			t.Errorf("CheckRunning failed: %s", err)
		}
		if p.OriRole != testRole {
			t.Errorf("getRole error: %s", p.OriRole)
		}
		if opsRes["role"] != testRole {
			t.Errorf("roleCheck response error: %s", resp.Data)
		}
	})
}

func startFooServer(port int, t *testing.T) net.Listener {
	for i := 0; i < 3; i++ {
		server, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if server == nil {
			t.Errorf("couldn't start listening: %s", err)
		} else {
			return server
		}
		port++
	}
	return nil
}

func stopFooServer(server net.Listener) {
	if server != nil {
		server.Close()
	}
}

func mockFakeOperations() *fakeOperations {
	log := logger.NewLogger("base_test")
	p := BaseOperations{Logger: log}
	return &fakeOperations{BaseOperations: p}
}

// Init initializes the fake binding.
func (fakeOps *fakeOperations) Init(metadata bindings.Metadata) {
	fakeOps.BaseOperations.Init(metadata)
	fakeOps.Logger.Debug("Initializing MySQL binding")
	fakeOps.DBType = "mysql"
	fakeOps.InitIfNeed = fakeOps.initIfNeed
	fakeOps.BaseOperations.GetRole = fakeOps.GetRole
	fakeOps.DBPort = fakeOps.GetRunningPort()
	fakeOps.RegisterOperation(CheckStatusOperation, fakeOps.CheckStatusOps)
}

func (fakeOps *fakeOperations) initIfNeed() bool {
	return false
}

func (fakeOps *fakeOperations) GetRunningPort() int {
	return testDBPort
}

func (fakeOps *fakeOperations) CheckStatusOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	return OpsResult{}, nil
}

func (fakeOps *fakeOperations) GetRole(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (string, error) {
	return testRole, nil
}
